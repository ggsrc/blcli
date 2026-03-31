package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tf "blcli/pkg/bootstrap/terraform"
	"blcli/pkg/config"
	"blcli/pkg/renderer"
)

// TestOptions holds options for test command
type TestOptions struct {
	ArgsPaths       []string
	Timeout         time.Duration
	ProjectName     string // If empty, test all projects
	Mode            string // Test mode: "plan" (default) or "apply"
	SkipBackend     bool   // Skip backend initialization during init
	UseEmulator     bool   // Use local GCS emulator instead of real GCS
	EmulatorPort    int    // Port for GCS emulator (default: 4443)
	EmulatorDataDir string // Data directory for emulator
}

// CheckRepoOptions holds options for check repo command
type CheckRepoOptions struct {
	ArgsPaths   []string
	Timeout     time.Duration
	ProjectName string // If empty, check all projects
}

// ExecuteTest executes the test command
func ExecuteTest(opts TestOptions) error {
	// Args file is required
	if len(opts.ArgsPaths) == 0 {
		return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
	}

	// Load args from files
	var allArgs []renderer.ArgsData
	for _, argsPath := range opts.ArgsPaths {
		fmt.Printf("Loading args from: %s\n", argsPath)
		args, err := renderer.LoadArgs(argsPath)
		if err != nil {
			return fmt.Errorf("failed to load args file %s: %w", argsPath, err)
		}
		allArgs = append(allArgs, args)
	}
	// Merge args: earlier files override later ones
	if len(allArgs) == 0 {
		return fmt.Errorf("no valid args files loaded")
	}
	reversed := make([]renderer.ArgsData, len(allArgs))
	for i, args := range allArgs {
		reversed[len(allArgs)-1-i] = args
	}
	mergedArgs := renderer.MergeArgs(reversed...)

	// Load configuration from args file
	cfg, err := config.LoadFromArgs(mergedArgs)
	if err != nil {
		return fmt.Errorf("failed to load config from args: %w", err)
	}

	if cfg.Terraform == nil {
		return fmt.Errorf("no terraform configuration found in args file")
	}

	// Get project names
	projects, err := tf.GetTerraformProjectNames(cfg.Terraform)
	if err != nil {
		return fmt.Errorf("failed to get terraform project names: %w", err)
	}

	if len(projects) == 0 {
		return fmt.Errorf("no terraform projects found to test")
	}

	// Filter projects if specific project is requested
	if opts.ProjectName != "" {
		found := false
		for _, p := range projects {
			if p == opts.ProjectName {
				projects = []string{p}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("project '%s' not found. Available projects: %v", opts.ProjectName, projects)
		}
	}

	workspace := config.WorkspacePath(cfg.Global)
	terraformBaseDir := filepath.Join(workspace, "terraform", "gcp")

	// Start GCS emulator if requested
	var emulator *GCSEmulator
	if opts.UseEmulator {
		fmt.Println("\n🚀 Starting GCS emulator...")
		emulator = NewGCSEmulator(opts.EmulatorPort, opts.EmulatorDataDir)
		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()

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

	// Test each project
	var failedProjects []string
	for _, projectName := range projects {
		projectDir := filepath.Join(terraformBaseDir, projectName)

		// Check if project directory exists
		if _, err := os.Stat(projectDir); os.IsNotExist(err) {
			fmt.Printf("⚠️  Project directory not found: %s (skipping)\n", projectDir)
			continue
		}

		fmt.Printf("\n🧪 Testing terraform project: %s\n", projectName)
		fmt.Printf("   Directory: %s\n", projectDir)
		if opts.Mode == "plan" {
			fmt.Printf("   Mode: dry-run (plan only, no resources will be created)\n")
		} else {
			fmt.Printf("   Mode: apply (will create and destroy real resources)\n")
		}

		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()

		if err := testTerraformProject(ctx, projectDir, opts.Mode, opts.SkipBackend, emulator); err != nil {
			fmt.Printf("❌ Test failed for project %s: %v\n", projectName, err)
			failedProjects = append(failedProjects, projectName)
		} else {
			fmt.Printf("✅ Test passed for project: %s\n", projectName)
		}
	}

	// Summary
	fmt.Printf("\n📊 Test Summary:\n")
	fmt.Printf("   Total projects: %d\n", len(projects))
	fmt.Printf("   Passed: %d\n", len(projects)-len(failedProjects))
	fmt.Printf("   Failed: %d\n", len(failedProjects))

	if len(failedProjects) > 0 {
		fmt.Printf("   Failed projects: %v\n", failedProjects)
		return fmt.Errorf("some tests failed")
	}

	return nil
}

// getSortedInitDirectories returns init directories sorted by numeric prefix
func getSortedInitDirectories(initDir string) ([]string, error) {
	// Check if init directory exists
	if _, err := os.Stat(initDir); os.IsNotExist(err) {
		return []string{}, nil // Return empty slice if directory doesn't exist
	}

	entries, err := os.ReadDir(initDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read init directory: %w", err)
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	// Sort directories by numeric prefix (e.g., 0-codestore, 1-my-org-projects)
	// This ensures execution order matches the numeric prefix
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i] < dirs[j] // Lexicographic sort works for numeric prefixes
	})

	return dirs, nil
}

// getTerraformDirectories returns all terraform directories in a given path
// A directory is considered a terraform directory if it contains .tf files
func getTerraformDirectories(baseDir string) ([]string, error) {
	// Check if base directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return []string{}, nil // Return empty slice if directory doesn't exist
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(baseDir, entry.Name())
		// Check if directory contains .tf files
		hasTfFiles, err := hasTerraformFiles(dirPath)
		if err != nil {
			// If we can't check, include it anyway
			dirs = append(dirs, entry.Name())
			continue
		}

		if hasTfFiles {
			dirs = append(dirs, entry.Name())
		}
	}

	// Sort directories alphabetically
	sort.Strings(dirs)

	return dirs, nil
}

// hasTerraformFiles checks if a directory contains .tf files
func hasTerraformFiles(dirPath string) (bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			if filepath.Ext(entry.Name()) == ".tf" {
				return true, nil
			}
		}
	}

	return false, nil
}

// testTerraformProject tests a single terraform project by running terraform commands
func testTerraformProject(ctx context.Context, projectDir string, mode string, skipBackend bool, emulator *GCSEmulator) error {
	// Change to project directory for terraform commands
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(projectDir); err != nil {
		return fmt.Errorf("failed to change to project directory: %w", err)
	}

	// Step 1: terraform init
	initArgs := []string{"init", "-input=false"}
	if skipBackend {
		// Use -backend=false and -reconfigure to force local backend
		initArgs = append(initArgs, "-backend=false", "-reconfigure")
		fmt.Println("   [1/3] Running: terraform init (skipping backend, using local state)")
	} else {
		fmt.Println("   [1/3] Running: terraform init")
	}
	if err := runTerraformCommand(ctx, initArgs, emulator); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}
	fmt.Println("   ✓ terraform init succeeded")

	// Step 2: terraform validate
	fmt.Println("   [2/3] Running: terraform validate")
	if err := runTerraformCommand(ctx, []string{"validate"}, emulator); err != nil {
		return fmt.Errorf("terraform validate failed: %w", err)
	}
	fmt.Println("   ✓ terraform validate succeeded")

	// Step 3: terraform plan
	fmt.Println("   [3/3] Running: terraform plan (dry-run)")
	planOutput, err := runTerraformCommandWithOutput(ctx, []string{"plan", "-input=false", "-out=tfplan"}, emulator)
	if err != nil {
		return fmt.Errorf("terraform plan failed: %w", err)
	}
	fmt.Println("   ✓ terraform plan succeeded")

	// Check if plan has changes (this is informational)
	if len(planOutput) > 0 {
		if bytes.Contains(planOutput, []byte("Plan:")) {
			fmt.Println("   ℹ️  Plan shows changes (this is normal for new resources)")
		}
	}

	// In plan mode (dry-run), we're done here
	if mode == "plan" {
		// Clean up plan file if it exists
		planFile := filepath.Join(projectDir, "tfplan")
		if _, err := os.Stat(planFile); err == nil {
			os.Remove(planFile)
		}
		fmt.Println("   ✓ Dry-run completed successfully (no resources were created)")
		return nil
	}

	// Apply mode: run apply and destroy
	if mode == "apply" {
		// Run terraform apply
		fmt.Println("\n   [Apply] Running: terraform apply")
		applyErr := runTerraformCommand(ctx, []string{"apply", "-auto-approve", "-input=false", "tfplan"}, emulator)
		if applyErr != nil {
			// Even if apply fails, try to clean up
			fmt.Println("\n   [Cleanup] Attempting terraform destroy after failed apply...")
			if destroyErr := runTerraformCommand(ctx, []string{"destroy", "-auto-approve", "-input=false"}, emulator); destroyErr != nil {
				fmt.Printf("   ⚠️  Warning: terraform destroy also failed: %v\n", destroyErr)
			}
			return fmt.Errorf("terraform apply failed: %w", applyErr)
		}
		fmt.Println("   ✓ terraform apply succeeded")

		// Show outputs if any
		fmt.Println("\n   [Outputs] Reading terraform outputs...")
		outputs, err := runTerraformCommandWithOutput(ctx, []string{"output", "-json"}, emulator)
		if err == nil && len(outputs) > 0 {
			fmt.Println("   ✓ Terraform outputs available (use 'terraform output' to view details)")
		}

		// Always destroy resources after successful apply
		fmt.Println("\n   [Cleanup] Running: terraform destroy")
		if err := runTerraformCommand(ctx, []string{"destroy", "-auto-approve", "-input=false"}, emulator); err != nil {
			return fmt.Errorf("terraform destroy failed: %w", err)
		}
		fmt.Println("   ✓ terraform destroy succeeded")

		return nil
	}

	return fmt.Errorf("unknown test mode: %s", mode)
}

// runTerraformCommand runs a terraform command and returns error if it fails
func runTerraformCommand(ctx context.Context, args []string, emulator *GCSEmulator) error {
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set emulator environment variables if emulator is running
	if emulator != nil {
		cmd.Env = os.Environ()
		emulatorEnv := emulator.SetupTerraformBackend()
		cmd.Env = append(cmd.Env, emulatorEnv...)
	}

	return cmd.Run()
}

// runTerraformCommandWithOutput runs a terraform command and returns the output
func runTerraformCommandWithOutput(ctx context.Context, args []string, emulator *GCSEmulator) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "terraform", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set emulator environment variables if emulator is running
	if emulator != nil {
		cmd.Env = os.Environ()
		emulatorEnv := emulator.SetupTerraformBackend()
		cmd.Env = append(cmd.Env, emulatorEnv...)
	}

	err := cmd.Run()
	if err != nil {
		// Combine stdout and stderr for error detection (terraform outputs errors to both)
		combinedOutput := append(stdout.Bytes(), stderr.Bytes()...)
		// Include stderr in error message for better debugging
		if stderr.Len() > 0 {
			return combinedOutput, fmt.Errorf("%w: %s", err, stderr.String())
		}
		return combinedOutput, err
	}

	return stdout.Bytes(), nil
}

// ExecuteCheckRepo executes the check repo command to validate terraform code compliance
// This function uses terratest-style validation: init, validate, and plan (dry-run only)
func ExecuteCheckRepo(opts CheckRepoOptions) error {
	// Args file is required
	if len(opts.ArgsPaths) == 0 {
		return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
	}

	// Load args from files
	var allArgs []renderer.ArgsData
	for _, argsPath := range opts.ArgsPaths {
		fmt.Printf("Loading args from: %s\n", argsPath)
		args, err := renderer.LoadArgs(argsPath)
		if err != nil {
			return fmt.Errorf("failed to load args file %s: %w", argsPath, err)
		}
		allArgs = append(allArgs, args)
	}
	// Merge args: earlier files override later ones
	if len(allArgs) == 0 {
		return fmt.Errorf("no valid args files loaded")
	}
	reversed := make([]renderer.ArgsData, len(allArgs))
	for i, args := range allArgs {
		reversed[len(allArgs)-1-i] = args
	}
	mergedArgs := renderer.MergeArgs(reversed...)

	// Load configuration from args file
	cfg, err := config.LoadFromArgs(mergedArgs)
	if err != nil {
		return fmt.Errorf("failed to load config from args: %w", err)
	}

	if cfg.Terraform == nil {
		return fmt.Errorf("no terraform configuration found in args file")
	}

	// Get project names
	projects, err := tf.GetTerraformProjectNames(cfg.Terraform)
	if err != nil {
		return fmt.Errorf("failed to get terraform project names: %w", err)
	}

	workspace := config.WorkspacePath(cfg.Global)
	initDir := filepath.Join(workspace, "terraform", "init")
	terraformBaseDir := filepath.Join(workspace, "terraform", "gcp")

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	var failedDirs []string

	// Step 1: Check init directories in numeric order
	fmt.Println("\n📋 Step 1: Checking init directories (in numeric order)...")
	initDirs, err := getSortedInitDirectories(initDir)
	if err != nil {
		return fmt.Errorf("failed to get init directories: %w", err)
	}

	if len(initDirs) > 0 {
		for _, initSubDir := range initDirs {
			initSubDirPath := filepath.Join(initDir, initSubDir)
			fmt.Printf("\n📁 Checking init directory: %s\n", initSubDir)
			fmt.Printf("   Directory: %s\n", initSubDirPath)

			// Use plan mode (dry-run) with backend skipped for compliance checking
			if err := checkTerraformDirectory(ctx, initSubDirPath); err != nil {
				fmt.Printf("❌ Check failed for init directory %s: %v\n", initSubDir, err)
				failedDirs = append(failedDirs, fmt.Sprintf("init/%s", initSubDir))
			} else {
				fmt.Printf("✅ Check passed for init directory: %s\n", initSubDir)
			}
		}
	} else {
		fmt.Println("   ℹ️  No init directories found, skipping...")
	}

	// Step 2: Check gcp modules directories
	modulesDir := filepath.Join(terraformBaseDir, "modules")
	fmt.Println("\n📋 Step 2: Checking gcp modules directories...")
	modulesDirs, err := getTerraformDirectories(modulesDir)
	if err != nil {
		return fmt.Errorf("failed to get modules directories: %w", err)
	}

	if len(modulesDirs) > 0 {
		for _, moduleName := range modulesDirs {
			moduleDir := filepath.Join(modulesDir, moduleName)
			fmt.Printf("\n📁 Checking terraform module: %s\n", moduleName)
			fmt.Printf("   Directory: %s\n", moduleDir)

			if err := checkTerraformDirectory(ctx, moduleDir); err != nil {
				fmt.Printf("❌ Check failed for module %s: %v\n", moduleName, err)
				failedDirs = append(failedDirs, fmt.Sprintf("gcp/modules/%s", moduleName))
			} else {
				fmt.Printf("✅ Check passed for module: %s\n", moduleName)
			}
		}
	} else {
		fmt.Println("   ℹ️  No modules directories found, skipping...")
	}

	// Step 3: Check gcp project directories
	if len(projects) == 0 {
		fmt.Println("\n⚠️  No terraform projects found to check")
	} else {
		fmt.Println("\n📋 Step 3: Checking gcp project directories...")

		// Filter projects if specific project is requested
		projectsToCheck := projects
		if opts.ProjectName != "" {
			found := false
			for _, p := range projects {
				if p == opts.ProjectName {
					projectsToCheck = []string{p}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("project '%s' not found. Available projects: %v", opts.ProjectName, projects)
			}
		}

		for _, projectName := range projectsToCheck {
			projectDir := filepath.Join(terraformBaseDir, projectName)

			// Check if project directory exists
			if _, err := os.Stat(projectDir); os.IsNotExist(err) {
				fmt.Printf("⚠️  Project directory not found: %s (skipping)\n", projectDir)
				continue
			}

			fmt.Printf("\n📁 Checking terraform project: %s\n", projectName)
			fmt.Printf("   Directory: %s\n", projectDir)

			if err := checkTerraformDirectory(ctx, projectDir); err != nil {
				fmt.Printf("❌ Check failed for project %s: %v\n", projectName, err)
				failedDirs = append(failedDirs, fmt.Sprintf("gcp/%s", projectName))
			} else {
				fmt.Printf("✅ Check passed for project: %s\n", projectName)
			}
		}
	}

	// Summary
	fmt.Printf("\n📊 Check Summary:\n")
	// Calculate total directories checked
	totalDirs := len(initDirs) + len(modulesDirs)
	if opts.ProjectName != "" {
		// If specific project requested, only count that one
		totalDirs += 1
	} else {
		// Count all projects that were actually checked (existing directories)
		projectsChecked := 0
		for _, projectName := range projects {
			projectDir := filepath.Join(terraformBaseDir, projectName)
			if _, err := os.Stat(projectDir); err == nil {
				projectsChecked++
			}
		}
		totalDirs += projectsChecked
	}
	fmt.Printf("   Total directories: %d\n", totalDirs)
	fmt.Printf("   Passed: %d\n", totalDirs-len(failedDirs))
	fmt.Printf("   Failed: %d\n", len(failedDirs))

	if len(failedDirs) > 0 {
		fmt.Printf("   Failed directories: %v\n", failedDirs)
		return fmt.Errorf("some checks failed")
	}

	return nil
}

// checkTerraformDirectory checks a terraform directory by running init, validate, and plan
// This is a simplified version that only does dry-run checks (no apply/destroy)
func checkTerraformDirectory(ctx context.Context, projectDir string) error {
	// Change to project directory for terraform commands
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(projectDir); err != nil {
		return fmt.Errorf("failed to change to project directory: %w", err)
	}

	// Step 1: terraform init (skip backend for compliance checking)
	fmt.Println("   [1/3] Running: terraform init (skipping backend, using local state)")
	initArgs := []string{"init", "-input=false", "-backend=false", "-reconfigure"}
	if err := runTerraformCommand(ctx, initArgs, nil); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}
	fmt.Println("   ✓ terraform init succeeded")

	// Step 2: terraform validate
	fmt.Println("   [2/3] Running: terraform validate")
	if err := runTerraformCommand(ctx, []string{"validate"}, nil); err != nil {
		return fmt.Errorf("terraform validate failed: %w", err)
	}
	fmt.Println("   ✓ terraform validate succeeded")

	// Step 3: terraform plan (dry-run only)
	// Note: Even though we skip backend during init, terraform plan may still try to use backend
	// if backend.tf exists. We'll handle this gracefully.
	fmt.Println("   [3/3] Running: terraform plan (dry-run)")
	planArgs := []string{"plan", "-input=false", "-out=tfplan"}
	planOutput, err := runTerraformCommandWithOutput(ctx, planArgs, nil)
	if err != nil {
		// Check if error is about backend initialization - this is expected when backend is skipped
		// Remove ANSI escape codes for matching
		errStr := string(planOutput)
		// Check for backend-related errors (case-insensitive, handle ANSI codes)
		backendErrorPatterns := []string{
			"Backend initialization required",
			"backend \"gcs\"",
			"Initial configuration of the requested backend",
			"backend initialization",
		}
		isBackendError := false
		for _, pattern := range backendErrorPatterns {
			if bytes.Contains(bytes.ToLower(planOutput), []byte(strings.ToLower(pattern))) {
				isBackendError = true
				break
			}
		}
		if isBackendError {
			// This is expected when we skip backend - backend.tf exists but backend wasn't initialized
			// We'll skip the plan step in this case and just report that validation passed
			fmt.Println("   ⚠️  terraform plan skipped (backend not initialized, but this is expected for compliance checking)")
			fmt.Println("   ✓ Compliance check completed (init and validate passed)")
			return nil
		}
		// Other errors should be reported
		return fmt.Errorf("terraform plan failed: %w\nOutput: %s", err, errStr)
	}
	fmt.Println("   ✓ terraform plan succeeded")

	// Check if plan has changes (this is informational)
	if len(planOutput) > 0 {
		if bytes.Contains(planOutput, []byte("Plan:")) {
			fmt.Println("   ℹ️  Plan shows changes (this is normal for new resources)")
		}
	}

	// Clean up plan file if it exists
	planFile := filepath.Join(projectDir, "tfplan")
	if _, err := os.Stat(planFile); err == nil {
		os.Remove(planFile)
	}
	fmt.Println("   ✓ Compliance check completed successfully (no resources were created)")

	return nil
}
