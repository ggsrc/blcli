package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tf "blcli/pkg/bootstrap/terraform"
	"blcli/pkg/config"
	"blcli/pkg/internal"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// BootstrapTerraform bootstraps Terraform projects based on template repository config
func BootstrapTerraform(global config.GlobalConfig, tfConfig *config.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, overwrite bool) ([]string, error) {
	return BootstrapTerraformWithProfiler(global, tfConfig, templateLoader, templateArgs, overwrite, nil, nil)
}

// BootstrapTerraformWithProfiler bootstraps Terraform projects with optional profiler and progress tracker
func BootstrapTerraformWithProfiler(global config.GlobalConfig, tfConfig *config.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, overwrite bool, profiler Profiler, progressTracker *ProgressTracker) ([]string, error) {
	workspace := config.WorkspacePath(global)
	terraformDir := filepath.Join(workspace, "terraform")

	// Check if terraform directory exists and has blcli marker
	if exists, err := internal.CheckBlcliMarker(terraformDir); err == nil && exists {
		if !overwrite {
			return nil, fmt.Errorf("terraform directory at %s was created by blcli. Use --overwrite to allow overwriting", terraformDir)
		}
	}

	// Load terraform config from template repository if available
	var terraformConfig *template.TerraformConfig
	if templateLoader != nil {
		cfg, err := templateLoader.LoadTerraformConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load terraform config from template repo: %w", err)
		}
		terraformConfig = cfg
	}

	// If no template config, fall back to built-in templates
	if terraformConfig == nil {
		return bootstrapTerraformBuiltin(global, tfConfig, templateLoader, templateArgs, overwrite)
	}

	// Bootstrap using template repository config
	projects, err := tf.GetTerraformProjectNames(tfConfig)
	if err != nil {
		return nil, err
	}
	return bootstrapTerraformFromConfig(global, tfConfig, terraformConfig, templateLoader, templateArgs, workspace, projects, overwrite, profiler, progressTracker)
}

// bootstrapTerraformFromConfig bootstraps using template repository config.yaml
func bootstrapTerraformFromConfig(global config.GlobalConfig, tfConfig *config.TerraformConfig, terraformConfig *template.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, workspace string, projects []string, overwrite bool, profiler Profiler, progressTracker *ProgressTracker) ([]string, error) {
	// Create terraform directory (marker written after init items with init plan)
	terraformDir := filepath.Join(workspace, "terraform")
	if err := internal.EnsureDir(terraformDir); err != nil {
		return nil, fmt.Errorf("failed to create terraform dir: %w", err)
	}

	// 1. Handle init items
	if progressTracker != nil {
		if err := progressTracker.StartStep("terraform", "init-items"); err != nil {
			// Non-fatal, continue
		}
	}
	initDir := filepath.Join(workspace, "terraform", "init")
	if err := internal.EnsureDir(initDir); err != nil {
		return nil, fmt.Errorf("failed to create terraform/init dir: %w", err)
	}

	// Prepare data for rendering (used for all init templates)
	data := tf.PrepareTerraformInitData(global, tfConfig, templateArgs)

	// Initialize init items (workspace is the root directory for destination paths)
	if err := tf.InitializeInitItems(terraformConfig, templateLoader, templateArgs, workspace, data); err != nil {
		if progressTracker != nil {
			progressTracker.FailStep("terraform", "init-items", fmt.Sprintf("%v", err))
		}
		return nil, err
	}
	if progressTracker != nil {
		progressTracker.CompleteStep("terraform", "init-items")
	}

	// Build init plan (prepare dirs + init dirs) for apply init; marker written after projects with dependency info
	prepareDirs, initDirs, err := tf.BuildInitPlan(terraformConfig, templateArgs, data)
	if err != nil {
		return nil, fmt.Errorf("build init plan: %w", err)
	}

	// Build project dependency plan (DAG + subdir components + layers) for apply order and dynamic promote
	dependencyOrder, subdirComponents, subdirComponentLayers, err := tf.BuildProjectDependencyPlan(templateArgs, projects)
	if err != nil {
		return nil, fmt.Errorf("build project dependency plan: %w", err)
	}

	// 2. Handle modules (shared across all projects)
	if progressTracker != nil {
		if err := progressTracker.StartStep("terraform", "modules"); err != nil {
			// Non-fatal, continue
		}
	}
	modulesDir := filepath.Join(workspace, "terraform", "gcp", "modules")
	if err := internal.EnsureDir(modulesDir); err != nil {
		return nil, fmt.Errorf("failed to create modules dir: %w", err)
	}

	if err := tf.InitializeModules(terraformConfig, templateLoader, templateArgs, modulesDir, global, profiler); err != nil {
		if progressTracker != nil {
			progressTracker.FailStep("terraform", "modules", fmt.Sprintf("%v", err))
		}
		return nil, err
	}
	if progressTracker != nil {
		progressTracker.CompleteStep("terraform", "modules")
	}

	// 3. Handle projects (one per project name)
	if progressTracker != nil {
		if err := progressTracker.StartStep("terraform", "projects"); err != nil {
			// Non-fatal, continue
		}
	}
	// gcpDir is terraform/gcp/ (GlobalName is only used in Terraform backend prefix, not file system path)
	gcpDir := filepath.Join(workspace, "terraform", "gcp")
	if err := internal.EnsureDir(gcpDir); err != nil {
		return nil, fmt.Errorf("failed to create terraform/gcp dir %s: %w", gcpDir, err)
	}

	initialized, err := tf.InitializeProjects(terraformConfig, templateLoader, templateArgs, gcpDir, projects, global, tfConfig, subdirComponents, profiler)
	if err != nil {
		if progressTracker != nil {
			progressTracker.FailStep("terraform", "projects", fmt.Sprintf("%v", err))
		}
		return nil, err
	}
	if progressTracker != nil {
		progressTracker.CompleteStep("terraform", "projects")
	}

	// Write terraform marker with init plan and project dependency info (for apply init and apply terraform dynamic promote)
	if err := internal.WriteTerraformMarkerWithDeps(terraformDir, prepareDirs, initDirs, dependencyOrder, subdirComponents, subdirComponentLayers); err != nil {
		return nil, fmt.Errorf("write terraform marker: %w", err)
	}

	return initialized, nil
}

// bootstrapTerraformBuiltin is the fallback when no template config is available
func bootstrapTerraformBuiltin(global config.GlobalConfig, tfConfig *config.TerraformConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData, overwrite bool) ([]string, error) {
	// This would use the old built-in template logic
	// For now, return an error suggesting to use a template repository
	return nil, fmt.Errorf("no template repository specified. Please use --template-repo to specify a template repository")
}

// DestroyTerraform destroys Terraform projects
func DestroyTerraform(global config.GlobalConfig, tfConfig *config.TerraformConfig) error {
	// Step 1: run terraform destroy against all known projects and init directories
	if err := runTerraformDestroy(global); err != nil {
		return err
	}
	// Step 2: clean up generated terraform directories on disk
	return tf.DestroyTerraform(global, tfConfig)
}

// runTerraformDestroy executes terraform destroy for all generated projects and init directories.
// It is intended for test / non-production environments where a full teardown is desired.
func runTerraformDestroy(global config.GlobalConfig) error {
	workspace := config.WorkspacePath(global)
	terraformDir := filepath.Join(workspace, "terraform")

	// If terraform directory does not exist, nothing to do.
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// 1) Destroy project directories under terraform/gcp in reverse order
	gcpDir := filepath.Join(terraformDir, "gcp")
	projectNames, err := getTerraformDirectories(gcpDir)
	if err == nil && len(projectNames) > 0 {
		for i := len(projectNames) - 1; i >= 0; i-- {
			name := projectNames[i]
			projectDir := filepath.Join(gcpDir, name)
			if err := terraformDestroyDirectory(ctx, projectDir); err != nil {
				fmt.Printf("Warning: terraform destroy failed for project %s: %v\n", name, err)
			}
		}
	}

	// 2) Destroy init directories under terraform/init (order is less strict, but destroy after projects)
	initDir := filepath.Join(terraformDir, "init")
	if _, err := os.Stat(initDir); err == nil {
		initSubDirs, err := getSortedInitDirectories(initDir)
		if err == nil {
			for i := len(initSubDirs) - 1; i >= 0; i-- {
				sub := initSubDirs[i]
				fullPath := getInitDirFullPath(terraformDir, sub)
				if err := terraformDestroyDirectory(ctx, fullPath); err != nil {
					fmt.Printf("Warning: terraform destroy failed for init dir %s: %v\n", sub, err)
				}
			}
		}
	}

	return nil
}

// terraformDestroyDirectory runs terraform init + terraform destroy in the given directory.
func terraformDestroyDirectory(ctx context.Context, projectDir string) error {
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		return nil
	}

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
		return fmt.Errorf("failed to change to project directory %s: %w", absProjectDir, err)
	}

	// Step 1: terraform init (reuse apply logic for backend config handling)
	initArgs := []string{"init", "-input=false"}
	backendConfigPath := filepath.Join(absProjectDir, "config.gcs.tfbackend")
	if _, statErr := os.Stat(backendConfigPath); statErr == nil {
		initArgs = append(initArgs, "-backend-config="+backendConfigPath)
		fmt.Println("   [1/2] Running: terraform init -backend-config=config.gcs.tfbackend (for destroy)")
	} else {
		fmt.Println("   [1/2] Running: terraform init (for destroy)")
	}
	if err := runTerraformCommand(ctx, initArgs, nil); err != nil {
		return fmt.Errorf("terraform init failed before destroy: %w", err)
	}

	// Step 2: terraform destroy
	fmt.Println("   [2/2] Running: terraform destroy -auto-approve")
	if err := runTerraformCommand(ctx, []string{"destroy", "-auto-approve"}, nil); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	return nil
}
