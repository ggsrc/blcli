package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"blcli/pkg/internal"
	"blcli/pkg/template"
)

// Go regexp does not support backreferences; capture project_id from the error line.
var tfAlreadyExistsProjectRe = regexp.MustCompile(`error creating project ([a-z0-9-]+) \([a-z0-9-]+\): googleapi: Error 409: Requested entity already exists`)
var tfBucketAlreadyOwnedRe = regexp.MustCompile(`googleapi: Error 409: Your previous request to create the named bucket succeeded and you already own it`)
var tfStateStoreBucketNameRe = regexp.MustCompile(`resource "google_storage_bucket" "terraform-statestore"[\s\S]*?\n\s*name\s*=\s*"([^"]+)"`)

func extractStateStoreBucketName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if m := tfStateStoreBucketNameRe.FindSubmatch(b); len(m) == 2 {
			return string(m[1]), nil
		}
	}
	return "", fmt.Errorf("bucket name not found in %s", dir)
}

// ExecuteApplyTerraform executes the apply terraform command
func ExecuteApplyTerraform(opts ApplyTerraformOptions) error {
	// Verify terraform directory exists
	if _, err := os.Stat(opts.TerraformDir); os.IsNotExist(err) {
		return fmt.Errorf("terraform directory not found: %s", opts.TerraformDir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	var emulator *GCSEmulator
	if opts.UseEmulator {
		fmt.Println("\n🚀 Starting GCS emulator...")
		emulator = NewGCSEmulator(opts.EmulatorPort, opts.EmulatorDataDir)
		if err := emulator.Start(ctx); err != nil {
			return fmt.Errorf("failed to start GCS emulator: %w", err)
		}
		fmt.Printf("   ✓ GCS emulator started at %s\n", emulator.Endpoint())
		defer func() {
			fmt.Println("\n🛑 Stopping GCS emulator...")
			if err := emulator.Stop(); err != nil {
				fmt.Printf("   ⚠️  Warning: failed to stop emulator: %v\n", err)
			} else {
				fmt.Println("   ✓ GCS emulator stopped")
			}
		}()
	}

	// Load terraform config from template repository if available
	var terraformConfig *template.TerraformConfig
	if opts.TemplateRepo != "" {
		loaderOptions := template.LoaderOptions{
			ForceUpdate: false,
			CacheExpiry: 24 * time.Hour,
		}
		templateLoader := template.NewLoaderWithOptions(opts.TemplateRepo, loaderOptions)
		if err := templateLoader.SyncCache(); err != nil {
			fmt.Printf("Warning: failed to sync template cache: %v (will use default order)\n", err)
		} else {
			cfg, err := templateLoader.LoadTerraformConfig()
			if err != nil {
				fmt.Printf("Warning: failed to load terraform config: %v (will use default order)\n", err)
			} else {
				terraformConfig = cfg
			}
		}
	}

	// Build execution plan
	var plan ExecutionPlan
	var initDirs []string
	var projectsToApply []string

	// Step 1: Collect init directories (use marker order if present: prepare first, then init)
	marker, errMarker := internal.ReadTerraformMarker(opts.TerraformDir)
	if errMarker == nil && marker != nil && (len(marker.InitPrepareDirs) > 0 || len(marker.InitDirs) > 0) {
		initDirs = append(initDirs, marker.InitPrepareDirs...)
		initDirs = append(initDirs, marker.InitDirs...)
	} else {
		initDir := filepath.Join(opts.TerraformDir, "init")
		var errInit error
		initDirs, errInit = getSortedInitDirectories(initDir)
		if errInit != nil {
			return fmt.Errorf("failed to get init directories: %w", errInit)
		}
	}

	// Step 2: Collect project directories and components
	terraformBaseDir := filepath.Join(opts.TerraformDir, "gcp")
	projectDirs, err := getTerraformDirectories(terraformBaseDir)
	if err != nil {
		return fmt.Errorf("failed to get project directories: %w", err)
	}

	// Filter projects if specific project is requested
	projectsToApply = projectDirs
	if opts.ProjectName != "" {
		found := false
		for _, p := range projectDirs {
			if p == opts.ProjectName {
				projectsToApply = []string{p}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("project '%s' not found. Available projects: %v", opts.ProjectName, projectDirs)
		}
	}

	// Build execution plan items
	var planItems []PlanItem
	step := 1

	// Add init directories to plan
	for _, initSubDir := range initDirs {
		initSubDirPath := getInitDirFullPath(opts.TerraformDir, initSubDir)
		displayName := "init/" + strings.TrimPrefix(initSubDir, "init/")
		planItems = append(planItems, PlanItem{
			Step:         step,
			Name:         displayName,
			Directory:    initSubDirPath,
			Command:      "terraform",
			Args:         []string{"apply", "-auto-approve"},
			Dependencies: []string{},
			Description:  fmt.Sprintf("Apply Terraform init directory: %s", initSubDir),
		})
		step++
	}

	// Resolve project order using config dependencies if available
	orderedProjects := projectsToApply
	if terraformConfig != nil {
		ordered, err := terraformConfig.ResolveTerraformDependencies(projectsToApply)
		if err != nil {
			fmt.Printf("Warning: failed to resolve terraform dependencies: %v (using directory order)\n", err)
		} else {
			orderedProjects = ordered
		}
	}

	// Add project directories to plan in dependency order
	for _, projectName := range orderedProjects {
		projectDir := filepath.Join(terraformBaseDir, projectName)
		if _, err := os.Stat(projectDir); os.IsNotExist(err) {
			continue
		}

		// Get dependencies for this project
		// For now, dependencies are shown at component level in the description
		// In the future, we can add project-level dependencies if config.yaml supports it
		var dependencies []string

		planItems = append(planItems, PlanItem{
			Step:         step,
			Name:         fmt.Sprintf("gcp/%s", projectName),
			Directory:    projectDir,
			Command:      "terraform",
			Args:         []string{"apply", "-auto-approve"},
			Dependencies: dependencies,
			Description:  fmt.Sprintf("Apply Terraform project: %s", projectName),
		})
		step++
	}

	plan = ExecutionPlan{
		Module: "terraform",
		Items:  planItems,
		DryRun: opts.DryRun,
	}

	// Print execution plan
	PrintExecutionPlan(plan)

	// If dry-run, exit here
	if opts.DryRun {
		return nil
	}

	var failedDirs []string

	// Step 1: Apply init directories in numeric order
	fmt.Println("\n📋 Step 1: Applying init directories (prepare first, then init)...")
	if len(initDirs) > 0 {
		for _, initSubDir := range initDirs {
			initSubDirPath := getInitDirFullPath(opts.TerraformDir, initSubDir)
			fmt.Printf("\n📁 Applying init directory: %s\n", initSubDir)
			fmt.Printf("   Directory: %s\n", initSubDirPath)

			if err := applyTerraformDirectory(ctx, initSubDirPath, opts.AutoApprove, opts.SkipBackend, emulator); err != nil {
				fmt.Printf("❌ Apply failed for init directory %s: %v\n", initSubDir, err)
				failedDirs = append(failedDirs, "init/"+strings.TrimPrefix(initSubDir, "init/"))
			} else {
				fmt.Printf("✅ Apply succeeded for init directory: %s\n", initSubDir)
			}
		}
	} else {
		fmt.Println("   ℹ️  No init directories found, skipping...")
	}

	// Step 2: Apply gcp project directories (in dependency order if available)
	fmt.Println("\n📋 Step 2: Applying gcp project directories...")
	if len(projectsToApply) == 0 {
		fmt.Println("   ℹ️  No project directories found, skipping...")
	} else {
		// Apply projects in the order from execution plan (which respects dependencies)
		for _, planItem := range planItems {
			if !strings.HasPrefix(planItem.Name, "gcp/") {
				continue // Skip init items
			}

			projectName := strings.TrimPrefix(planItem.Name, "gcp/")
			projectDir := planItem.Directory

			// Check if project directory exists
			if _, err := os.Stat(projectDir); os.IsNotExist(err) {
				fmt.Printf("⚠️  Project directory not found: %s (skipping)\n", projectDir)
				continue
			}

			fmt.Printf("\n📁 Applying terraform project: %s\n", projectName)
			if len(planItem.Dependencies) > 0 {
				fmt.Printf("   Dependencies: %s\n", strings.Join(planItem.Dependencies, ", "))
			}
			fmt.Printf("   Directory: %s\n", projectDir)

			if err := applyTerraformDirectory(ctx, projectDir, opts.AutoApprove, opts.SkipBackend, emulator); err != nil {
				fmt.Printf("❌ Apply failed for project %s: %v\n", projectName, err)
				failedDirs = append(failedDirs, fmt.Sprintf("gcp/%s", projectName))
			} else {
				fmt.Printf("✅ Apply succeeded for project: %s\n", projectName)
			}
		}
	}

	// Step 3: Dynamic promote and apply by dependency layer (from marker.SubdirComponentLayers)
	if marker != nil && len(marker.SubdirComponents) > 0 {
		terraformBaseDir := filepath.Join(opts.TerraformDir, "gcp")
		layersToRounds := buildLayerRounds(marker.SubdirComponentLayers)
		if len(layersToRounds) == 0 {
			// Old marker without SubdirComponentLayers: single promote + apply all with subdir
			if err := promoteSubdirComponents(terraformBaseDir, marker.SubdirComponents, projectsToApply); err != nil {
				return fmt.Errorf("promote subdir components: %w", err)
			}
			fmt.Println("\n📋 Step 3: Applying gcp project directories (after promote)...")
			for _, planItem := range planItems {
				if !strings.HasPrefix(planItem.Name, "gcp/") {
					continue
				}
				projectName := strings.TrimPrefix(planItem.Name, "gcp/")
				projectDir := planItem.Directory
				if _, err := os.Stat(projectDir); os.IsNotExist(err) {
					continue
				}
				if _, hasPromoted := marker.SubdirComponents[projectName]; !hasPromoted {
					continue
				}
				fmt.Printf("\n📁 Applying terraform project (post-promote): %s\n", projectName)
				if err := applyTerraformDirectory(ctx, projectDir, opts.AutoApprove, opts.SkipBackend, emulator); err != nil {
					fmt.Printf("❌ Apply failed for project %s (post-promote): %v\n", projectName, err)
					failedDirs = append(failedDirs, fmt.Sprintf("gcp/%s (post-promote)", projectName))
				} else {
					fmt.Printf("✅ Apply succeeded for project %s (post-promote)\n", projectName)
				}
			}
		} else {
			// New marker with SubdirComponentLayers: one round per layer
			for _, layer := range layersToRounds {
				toPromote := getSubdirComponentsForLayer(marker.SubdirComponentLayers, layer)
				if len(toPromote) == 0 {
					continue
				}
				if err := promoteSubdirComponents(terraformBaseDir, toPromote, projectsToApply); err != nil {
					return fmt.Errorf("promote subdir components (layer %d): %w", layer, err)
				}
				fmt.Printf("\n📋 Step 3 (layer %d): Applying gcp project directories (after promote)...\n", layer)
				for projectName := range toPromote {
					projectDir := filepath.Join(terraformBaseDir, projectName)
					if _, err := os.Stat(projectDir); os.IsNotExist(err) {
						continue
					}
					fmt.Printf("\n📁 Applying terraform project (layer %d, post-promote): %s\n", layer, projectName)
					if err := applyTerraformDirectory(ctx, projectDir, opts.AutoApprove, opts.SkipBackend, emulator); err != nil {
						fmt.Printf("❌ Apply failed for project %s (layer %d): %v\n", projectName, layer, err)
						failedDirs = append(failedDirs, fmt.Sprintf("gcp/%s (layer %d)", projectName, layer))
					} else {
						fmt.Printf("✅ Apply succeeded for project %s (layer %d)\n", projectName, layer)
					}
				}
			}
		}
	}

	// Summary
	fmt.Printf("\n📊 Apply Summary:\n")
	totalDirs := len(initDirs) + len(projectsToApply)
	fmt.Printf("   Total directories: %d\n", totalDirs)
	fmt.Printf("   Passed: %d\n", totalDirs-len(failedDirs))
	fmt.Printf("   Failed: %d\n", len(failedDirs))

	if len(failedDirs) > 0 {
		fmt.Printf("   Failed directories: %v\n", failedDirs)
		return fmt.Errorf("some applies failed")
	}

	return nil
}

// promoteSubdirComponents moves .tf files from project/component/ to project/ and removes empty subdirs.
func promoteSubdirComponents(terraformBaseDir string, subdirComponents map[string][]string, projectsToApply []string) error {
	projectSet := make(map[string]bool)
	for _, p := range projectsToApply {
		projectSet[p] = true
	}
	fmt.Println("\n📦 Promoting subdir components to project root...")
	for projectName, components := range subdirComponents {
		if !projectSet[projectName] {
			continue
		}
		projectDir := filepath.Join(terraformBaseDir, projectName)
		for _, componentName := range components {
			subdir := filepath.Join(projectDir, componentName)
			entries, err := os.ReadDir(subdir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("read subdir %s: %w", subdir, err)
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
					continue
				}
				src := filepath.Join(subdir, e.Name())
				dst := filepath.Join(projectDir, e.Name())
				if err := os.Rename(src, dst); err != nil {
					return fmt.Errorf("move %s -> %s: %w", src, dst, err)
				}
				fmt.Printf("   Moved %s -> %s\n", filepath.Join(projectName, componentName, e.Name()), filepath.Join(projectName, e.Name()))
			}
			// Remove empty subdir
			if err := os.Remove(subdir); err != nil && !os.IsNotExist(err) {
				fmt.Printf("   Warning: remove empty subdir %s: %v\n", subdir, err)
			}
		}
	}
	return nil
}

// buildLayerRounds returns sorted layer numbers (1, 2, ...) that appear in subdirComponentLayers, for apply order.
func buildLayerRounds(subdirComponentLayers map[string]int) []int {
	if len(subdirComponentLayers) == 0 {
		return nil
	}
	layerSet := make(map[int]bool)
	for _, layer := range subdirComponentLayers {
		if layer >= 1 {
			layerSet[layer] = true
		}
	}
	var layers []int
	for L := range layerSet {
		layers = append(layers, L)
	}
	for i := 0; i < len(layers); i++ {
		for j := i + 1; j < len(layers); j++ {
			if layers[j] < layers[i] {
				layers[i], layers[j] = layers[j], layers[i]
			}
		}
	}
	return layers
}

// getSubdirComponentsForLayer returns project -> components for the given layer (for promote + apply that round).
func getSubdirComponentsForLayer(subdirComponentLayers map[string]int, layer int) map[string][]string {
	out := make(map[string][]string)
	for key, L := range subdirComponentLayers {
		if L != layer {
			continue
		}
		idx := strings.Index(key, "/")
		if idx <= 0 || idx >= len(key)-1 {
			continue
		}
		projectName := key[:idx]
		componentName := key[idx+1:]
		out[projectName] = append(out[projectName], componentName)
	}
	return out
}

// getInitDirFullPath returns the full path for an init subdir.
// rel may be "init/0-xxx" (from marker) or "0-xxx" (from getSortedInitDirectories).
func getInitDirFullPath(terraformDir, rel string) string {
	if strings.HasPrefix(rel, "init/") {
		return filepath.Join(terraformDir, rel)
	}
	return filepath.Join(terraformDir, "init", rel)
}

// ExecuteApplyInit runs only the init phase: first prepare init dirs (e.g. 0-terraform-statestore), then other init dirs.
// Used as step 0 before applying projects. Does not apply gcp project directories.
func ExecuteApplyInit(opts ApplyTerraformOptions) error {
	if _, err := os.Stat(opts.TerraformDir); os.IsNotExist(err) {
		return fmt.Errorf("terraform directory not found: %s", opts.TerraformDir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	var emulator *GCSEmulator
	if opts.UseEmulator {
		fmt.Println("\n🚀 Starting GCS emulator...")
		emulator = NewGCSEmulator(opts.EmulatorPort, opts.EmulatorDataDir)
		if err := emulator.Start(ctx); err != nil {
			return fmt.Errorf("failed to start GCS emulator: %w", err)
		}
		fmt.Printf("   ✓ GCS emulator started at %s\n", emulator.Endpoint())
		defer func() {
			fmt.Println("\n🛑 Stopping GCS emulator...")
			if err := emulator.Stop(); err != nil {
				fmt.Printf("   ⚠️  Warning: failed to stop emulator: %v\n", err)
			} else {
				fmt.Println("   ✓ GCS emulator stopped")
			}
		}()
	}

	marker, err := internal.ReadTerraformMarker(opts.TerraformDir)
	if err != nil {
		return fmt.Errorf("read terraform marker: %w", err)
	}

	var initDirsOrdered []string
	if marker != nil && (len(marker.InitPrepareDirs) > 0 || len(marker.InitDirs) > 0) {
		initDirsOrdered = append(initDirsOrdered, marker.InitPrepareDirs...)
		initDirsOrdered = append(initDirsOrdered, marker.InitDirs...)
	} else {
		initDir := filepath.Join(opts.TerraformDir, "init")
		initDirsOrdered, err = getSortedInitDirectories(initDir)
		if err != nil {
			return fmt.Errorf("failed to get init directories: %w", err)
		}
	}

	if len(initDirsOrdered) == 0 {
		fmt.Println("   ℹ️  No init directories to apply")
		return nil
	}

	fmt.Println("\n📋 Apply init (prepare first, then init directories)...")
	var failedDirs []string
	for i, rel := range initDirsOrdered {
		initSubDirPath := getInitDirFullPath(opts.TerraformDir, rel)
		if _, err := os.Stat(initSubDirPath); os.IsNotExist(err) {
			fmt.Printf("⚠️  Init directory not found: %s (skipping)\n", initSubDirPath)
			continue
		}
		fmt.Printf("\n📁 Applying init directory: %s\n", rel)
		fmt.Printf("   Directory: %s\n", initSubDirPath)
		if err := applyTerraformDirectory(ctx, initSubDirPath, opts.AutoApprove, opts.SkipBackend, emulator); err != nil {
			fmt.Printf("❌ Apply failed for init directory %s: %v\n", rel, err)
			failedDirs = append(failedDirs, rel)
		} else {
			fmt.Printf("✅ Apply succeeded for init directory: %s\n", rel)
			// Wait before next directory to allow GCP APIs/resources to propagate
			if opts.InitDelay > 0 && i < len(initDirsOrdered)-1 {
				fmt.Printf("   ⏳ Waiting %v before next directory...\n", opts.InitDelay)
				time.Sleep(opts.InitDelay)
			}
		}
	}

	fmt.Printf("\n📊 Apply init summary: %d total, %d passed, %d failed\n",
		len(initDirsOrdered), len(initDirsOrdered)-len(failedDirs), len(failedDirs))
	if len(failedDirs) > 0 {
		return fmt.Errorf("apply init failed for: %v", failedDirs)
	}
	return nil
}

// applyTerraformDirectory applies a terraform directory by running init, plan, and apply
func applyTerraformDirectory(ctx context.Context, projectDir string, autoApprove bool, skipBackend bool, emulator *GCSEmulator) error {
	// Resolve projectDir to absolute path before chdir so filepath.Join stays valid
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s: %w", projectDir, err)
	}

	// Change to project directory for terraform commands
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(absProjectDir); err != nil {
		return fmt.Errorf("failed to change to project directory: %w", err)
	}

	// Step 1: terraform init
	initArgs := []string{"init", "-input=false"}
	if skipBackend {
		initArgs = append(initArgs, "-backend=false", "-reconfigure")
		fmt.Println("   [1/4] Running: terraform init (skipping backend, using local state)")
	} else {
		backendConfigPath := filepath.Join(absProjectDir, "config.gcs.tfbackend")
		if _, err := os.Stat(backendConfigPath); err == nil {
			initArgs = append(initArgs, "-backend-config="+backendConfigPath)
			fmt.Println("   [1/4] Running: terraform init -backend-config=config.gcs.tfbackend")
		} else {
			fmt.Println("   [1/4] Running: terraform init")
		}
	}
	if err := runTerraformCommand(ctx, initArgs, emulator); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}
	fmt.Println("   ✓ terraform init succeeded")

	// Step 2: terraform validate
	fmt.Println("   [2/4] Running: terraform validate")
	if err := runTerraformCommand(ctx, []string{"validate"}, emulator); err != nil {
		return fmt.Errorf("terraform validate failed: %w", err)
	}
	fmt.Println("   ✓ terraform validate succeeded")

	// Step 3: terraform plan
	fmt.Println("   [3/4] Running: terraform plan")
	planArgs := []string{"plan", "-input=false", "-out=tfplan"}
	planOutput, err := runTerraformCommandWithOutput(ctx, planArgs, emulator)
	if err != nil {
		return fmt.Errorf("terraform plan failed: %w", err)
	}
	fmt.Println("   ✓ terraform plan succeeded")

	// Check if plan has changes
	if len(planOutput) > 0 {
		if bytes.Contains(planOutput, []byte("No changes")) {
			fmt.Println("   ℹ️  No changes detected, skipping apply")
			// Clean up plan file
			planFile := filepath.Join(projectDir, "tfplan")
			if _, err := os.Stat(planFile); err == nil {
				os.Remove(planFile)
			}
			return nil
		}
	}

	// Step 4: terraform apply
	fmt.Println("   [4/4] Running: terraform apply")
	applyArgs := []string{"apply", "-input=false"}
	if autoApprove {
		applyArgs = append(applyArgs, "-auto-approve", "tfplan")
	} else {
		// For interactive mode, we still need to pass the plan file
		applyArgs = append(applyArgs, "tfplan")
		// Note: User will need to confirm interactively
		fmt.Println("   ⚠️  Waiting for user confirmation...")
	}

	applyOutput, err := runTerraformCommandWithOutput(ctx, applyArgs, emulator)
	if err != nil {
		// E2E idempotency: when local state is missing but resource already exists (common for init/0-terraform-statestore),
		// terraform apply may fail with Error 409. Auto-import known resources and retry once.
		didImport := false

		if m := tfAlreadyExistsProjectRe.FindSubmatch(applyOutput); len(m) == 2 {
			projectID := string(m[1])
			fmt.Printf("   ⚠️  Project already exists: %s. Importing into state...\n", projectID)
			if importErr := runTerraformCommand(ctx, []string{"import", "-input=false", "google_project.main", projectID}, emulator); importErr != nil {
				return fmt.Errorf("terraform apply failed: %w (auto-import project failed: %v)", err, importErr)
			}
			didImport = true
		}

		if tfBucketAlreadyOwnedRe.Match(applyOutput) {
			bucketName, nameErr := extractStateStoreBucketName(absProjectDir)
			if nameErr != nil {
				return fmt.Errorf("terraform apply failed: %w (bucket conflict detected but failed to extract bucket name: %v)", err, nameErr)
			}
			fmt.Printf("   ⚠️  Bucket already owned: %s. Importing into state...\n", bucketName)
			if importErr := runTerraformCommand(ctx, []string{"import", "-input=false", "google_storage_bucket.terraform-statestore", bucketName}, emulator); importErr != nil {
				return fmt.Errorf("terraform apply failed: %w (auto-import bucket failed: %v)", err, importErr)
			}
			didImport = true
		}

		if didImport {
			// Import changes the state, so the previously saved plan (tfplan) is stale.
			// Re-run plan to generate a fresh tfplan, then apply again.
			fmt.Printf("   🔁 Re-planning after auto-import...\n")
			if _, planErr := runTerraformCommandWithOutput(ctx, planArgs, emulator); planErr != nil {
				return fmt.Errorf("terraform apply failed: %w (auto-import succeeded but re-plan failed: %v)", err, planErr)
			}
			fmt.Printf("   🔁 Retrying terraform apply after auto-import...\n")
			if _, retryErr := runTerraformCommandWithOutput(ctx, applyArgs, emulator); retryErr != nil {
				return fmt.Errorf("terraform apply failed after auto-import (and re-plan): %w", retryErr)
			}
			fmt.Println("   ✓ terraform apply succeeded (after auto-import + re-plan)")
		} else {
			return fmt.Errorf("terraform apply failed: %w", err)
		}
	}
	if err == nil {
		fmt.Println("   ✓ terraform apply succeeded")
	}

	// Clean up plan file
	planFile := filepath.Join(projectDir, "tfplan")
	if _, err := os.Stat(planFile); err == nil {
		os.Remove(planFile)
	}

	return nil
}
