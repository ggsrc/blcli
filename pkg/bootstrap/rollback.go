package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"blcli/pkg/config"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

// RollbackOptions contains options for rollback operation
type RollbackOptions struct {
	Module       string   // terraform, kubernetes, gitops
	Component    string   // Optional: specific component name
	Project      string   // Optional: specific project name (for terraform)
	Workspace    string   // Workspace directory
	ArgsPaths    []string // Args file paths
	TemplateRepo string   // Template repository URL or local path
	DryRun       bool     // Only show rollback plan, don't execute
	AutoApprove  bool     // Auto approve rollback
	Kubeconfig   string   // Kubernetes kubeconfig path
	Context      string   // Kubernetes context name
}

// RollbackResult represents the result of a rollback operation
type RollbackResult struct {
	Component string
	Success   bool
	Error     error
	Command   string
}

// ExecuteRollback executes rollback for the specified module
func ExecuteRollback(opts RollbackOptions) error {
	// Load args
	var templateArgs renderer.ArgsData
	var allArgs []renderer.ArgsData
	for _, argsPath := range opts.ArgsPaths {
		fmt.Printf("Loading args from: %s\n", argsPath)
		args, err := renderer.LoadArgs(argsPath)
		if err != nil {
			return fmt.Errorf("failed to load args file %s: %w", argsPath, err)
		}
		allArgs = append(allArgs, args)
	}
	if len(allArgs) > 0 {
		reversed := make([]renderer.ArgsData, len(allArgs))
		for i, args := range allArgs {
			reversed[len(allArgs)-1-i] = args
		}
		templateArgs = renderer.MergeArgs(reversed...)
	}

	// Load config
	cfg, err := config.LoadFromArgs(templateArgs)
	if err != nil {
		return fmt.Errorf("failed to load config from args: %w", err)
	}

	// Override workspace if specified
	if opts.Workspace != "" {
		cfg.Global.Workspace = opts.Workspace
	} else {
		opts.Workspace = config.WorkspacePath(cfg.Global)
	}

	// Load template loader if template repo is provided
	var templateLoader *template.Loader
	if opts.TemplateRepo != "" {
		loaderOptions := template.LoaderOptions{
			ForceUpdate: false,
			CacheExpiry: 24 * time.Hour,
		}
		templateLoader = template.NewLoaderWithOptions(opts.TemplateRepo, loaderOptions)
		if err := templateLoader.SyncCache(); err != nil {
			return fmt.Errorf("failed to sync template cache: %w", err)
		}
	}

	// Execute rollback based on module type
	switch opts.Module {
	case "terraform":
		return RollbackTerraform(opts, cfg, templateLoader, templateArgs)
	case "kubernetes":
		return RollbackKubernetes(opts, cfg, templateLoader, templateArgs)
	case "gitops":
		return RollbackGitOps(opts, cfg, templateLoader, templateArgs)
	default:
		return fmt.Errorf("unsupported module type: %s", opts.Module)
	}
}

// RollbackTerraform executes rollback for Terraform resources
func RollbackTerraform(opts RollbackOptions, cfg config.BlcliConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData) error {
	if cfg.Terraform == nil {
		return fmt.Errorf("no terraform configuration found")
	}

	terraformDir := filepath.Join(opts.Workspace, "terraform")
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		return fmt.Errorf("terraform directory not found: %s", terraformDir)
	}

	// Load terraform config from template repository if available
	var terraformConfig *template.TerraformConfig
	if templateLoader != nil {
		tfConfig, err := templateLoader.LoadTerraformConfig()
		if err != nil {
			fmt.Printf("Warning: failed to load terraform config: %v (will use default rollback)\n", err)
		} else {
			terraformConfig = tfConfig
		}
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🗑️  Rolling back Terraform resources")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var rollbackItems []rollbackItem

	// Collect init items to rollback
	if terraformConfig != nil {
		initDir := filepath.Join(terraformDir, "init")
		if _, err := os.Stat(initDir); err == nil {
			initDirs, err := getSortedInitDirectories(initDir)
			if err == nil {
				for _, initSubDir := range initDirs {
					initSubDirPath := filepath.Join(initDir, initSubDir)
					// Find matching init item config
					for _, initItem := range terraformConfig.Init {
						if strings.Contains(initSubDir, initItem.Name) || strings.Contains(initSubDir, strings.ReplaceAll(initItem.Destination, "terraform/init/", "")) {
							rollbackItems = append(rollbackItems, rollbackItem{
								Name:     fmt.Sprintf("init/%s", initSubDir),
								Path:     initSubDirPath,
								Rollback: initItem.Rollback,
								Type:     "init",
							})
							break
						}
					}
				}
			}
		}

		// Collect project items to rollback
		if opts.Project != "" {
			// Rollback specific project
			projectDir := filepath.Join(terraformDir, "gcp", opts.Project)
			if _, err := os.Stat(projectDir); err == nil {
				// Find matching project item config
				for _, projectItem := range terraformConfig.Projects {
					rollbackItems = append(rollbackItems, rollbackItem{
						Name:     fmt.Sprintf("gcp/%s/%s", opts.Project, projectItem.Name),
						Path:     projectDir,
						Rollback: projectItem.Rollback,
						Type:     "project",
					})
				}
			}
		} else {
			// Rollback all projects
			gcpDir := filepath.Join(terraformDir, "gcp")
			if _, err := os.Stat(gcpDir); err == nil {
				projectDirs, err := getTerraformDirectories(gcpDir)
				if err == nil {
					for _, projectName := range projectDirs {
						projectDir := filepath.Join(gcpDir, projectName)
						// Find matching project items config
						for _, projectItem := range terraformConfig.Projects {
							rollbackItems = append(rollbackItems, rollbackItem{
								Name:     fmt.Sprintf("gcp/%s/%s", projectName, projectItem.Name),
								Path:     projectDir,
								Rollback: projectItem.Rollback,
								Type:     "project",
							})
						}
					}
				}
			}
		}
	}

	// If no config items found, use default rollback for existing directories
	if len(rollbackItems) == 0 {
		// Default: rollback all init directories
		initDir := filepath.Join(terraformDir, "init")
		if _, err := os.Stat(initDir); err == nil {
			initDirs, err := getSortedInitDirectories(initDir)
			if err == nil {
				for _, initSubDir := range initDirs {
					initSubDirPath := filepath.Join(initDir, initSubDir)
					rollbackItems = append(rollbackItems, rollbackItem{
						Name:     fmt.Sprintf("init/%s", initSubDir),
						Path:     initSubDirPath,
						Rollback: "", // Use default
						Type:     "init",
					})
				}
			}
		}

		// Default: rollback all project directories
		gcpDir := filepath.Join(terraformDir, "gcp")
		if _, err := os.Stat(gcpDir); err == nil {
			projectDirs, err := getTerraformDirectories(gcpDir)
			if err == nil {
				for _, projectName := range projectDirs {
					if opts.Project != "" && projectName != opts.Project {
						continue
					}
					projectDir := filepath.Join(gcpDir, projectName)
					rollbackItems = append(rollbackItems, rollbackItem{
						Name:     fmt.Sprintf("gcp/%s", projectName),
						Path:     projectDir,
						Rollback: "", // Use default
						Type:     "project",
					})
				}
			}
		}
	}

	// Reverse order for rollback (last deployed first)
	for i := len(rollbackItems) - 1; i >= 0; i-- {
		item := rollbackItems[i]
		fmt.Printf("\n📁 Rolling back: %s\n", item.Name)
		fmt.Printf("   Directory: %s\n", item.Path)

		rollbackCmd := item.Rollback
		if rollbackCmd == "" {
			// Default rollback command
			rollbackCmd = "terraform destroy -auto-approve"
		} else {
			// Render template variables in rollback command
			data := map[string]interface{}{
				"ProjectName":   opts.Project,
				"ComponentName": item.Name,
				"Workspace":     opts.Workspace,
			}
			// Merge with template args
			for k, v := range templateArgs {
				data[k] = v
			}
			rendered, err := template.RenderWithArgs(rollbackCmd, data, templateArgs)
			if err != nil {
				fmt.Printf("⚠️  Warning: failed to render rollback command: %v\n", err)
				rollbackCmd = "terraform destroy -auto-approve" // Fallback to default
			} else {
				rollbackCmd = rendered
			}
		}

		if opts.DryRun {
			fmt.Printf("   [DRY RUN] Would execute: %s\n", rollbackCmd)
			continue
		}

		// Execute rollback command
		parts := strings.Fields(rollbackCmd)
		if len(parts) == 0 {
			continue
		}

		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Dir = item.Path
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("❌ Rollback failed for %s: %v\n", item.Name, err)
			if !opts.AutoApprove {
				return fmt.Errorf("rollback failed for %s: %w", item.Name, err)
			}
		} else {
			fmt.Printf("✅ Rollback succeeded for %s\n", item.Name)
		}
	}

	fmt.Println("\n✅ Terraform rollback completed")
	return nil
}

// RollbackKubernetes executes rollback for Kubernetes resources
func RollbackKubernetes(opts RollbackOptions, cfg config.BlcliConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData) error {
	if cfg.Kubernetes == nil {
		return fmt.Errorf("no kubernetes configuration found")
	}

	kubernetesDir := filepath.Join(opts.Workspace, "kubernetes")
	if _, err := os.Stat(kubernetesDir); os.IsNotExist(err) {
		return fmt.Errorf("kubernetes directory not found: %s", kubernetesDir)
	}

	// Load kubernetes config from template repository if available
	var kubernetesConfig *template.KubernetesConfig
	if templateLoader != nil {
		k8sConfig, err := templateLoader.LoadKubernetesConfig()
		if err != nil {
			fmt.Printf("Warning: failed to load kubernetes config: %v (will use default rollback)\n", err)
		} else {
			kubernetesConfig = k8sConfig
		}
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🗑️  Rolling back Kubernetes resources")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Get all components from config
	var componentsToRollback []string
	if kubernetesConfig != nil {
		allComponents := kubernetesConfig.GetAllComponents()
		if opts.Component != "" {
			// Rollback specific component
			if _, exists := allComponents[opts.Component]; exists {
				componentsToRollback = []string{opts.Component}
			} else {
				return fmt.Errorf("component %s not found in config", opts.Component)
			}
		} else {
			// Rollback all components
			for name := range allComponents {
				componentsToRollback = append(componentsToRollback, name)
			}
		}

		// Resolve dependencies and reverse order for rollback
		orderedComponents, err := kubernetesConfig.ResolveKubernetesDependencies(componentsToRollback)
		if err != nil {
			return fmt.Errorf("failed to resolve dependencies: %w", err)
		}

		// Reverse order for rollback
		for i := len(orderedComponents) - 1; i >= 0; i-- {
			componentName := orderedComponents[i]
			component := allComponents[componentName]

			// Find component directory
			projectDirs, err := getKubernetesProjectDirs(kubernetesDir)
			if err != nil {
				return fmt.Errorf("failed to get project directories: %w", err)
			}

			for _, projectName := range projectDirs {
				componentDir := filepath.Join(kubernetesDir, projectName, componentName)
				if _, err := os.Stat(componentDir); os.IsNotExist(err) {
					continue
				}

				fmt.Printf("\n📦 Rolling back component: %s/%s\n", projectName, componentName)
				fmt.Printf("   Directory: %s\n", componentDir)

				rollbackCmd := component.Rollback
				if rollbackCmd == "" {
					// Default rollback based on installType
					switch component.InstallType {
					case template.InstallTypeHelm:
						releaseName := componentName
						namespace := component.Namespace
						if namespace == "" {
							namespace = componentName
						}
						rollbackCmd = fmt.Sprintf("helm uninstall %s -n %s", releaseName, namespace)
					case template.InstallTypeKubectl:
						rollbackCmd = fmt.Sprintf("kubectl delete -k %s", componentDir)
					case template.InstallTypeCustom:
						fmt.Printf("⚠️  Warning: custom installType requires rollback command in config.yaml\n")
						continue
					default:
						rollbackCmd = fmt.Sprintf("kubectl delete -k %s", componentDir)
					}
				} else {
					// Render template variables in rollback command
					data := map[string]interface{}{
						"ComponentName": componentName,
						"ProjectName":   projectName,
						"Namespace":     component.Namespace,
						"Workspace":     opts.Workspace,
					}
					// Merge with template args
					for k, v := range templateArgs {
						data[k] = v
					}
					rendered, err := template.RenderWithArgs(rollbackCmd, data, templateArgs)
					if err != nil {
						fmt.Printf("⚠️  Warning: failed to render rollback command: %v\n", err)
						continue
					}
					rollbackCmd = rendered
				}

				if opts.DryRun {
					fmt.Printf("   [DRY RUN] Would execute: %s\n", rollbackCmd)
					continue
				}

				// Execute rollback command
				parts := strings.Fields(rollbackCmd)
				if len(parts) == 0 {
					continue
				}

				cmd := exec.Command(parts[0], parts[1:]...)
				if opts.Kubeconfig != "" {
					cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", opts.Kubeconfig))
				}
				if opts.Context != "" {
					cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECTL_CONTEXT=%s", opts.Context))
				}
				cmd.Dir = componentDir
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				if err := cmd.Run(); err != nil {
					fmt.Printf("❌ Rollback failed for %s/%s: %v\n", projectName, componentName, err)
					if !opts.AutoApprove {
						return fmt.Errorf("rollback failed for %s/%s: %w", projectName, componentName, err)
					}
				} else {
					fmt.Printf("✅ Rollback succeeded for %s/%s\n", projectName, componentName)
				}
			}
		}
	} else {
		// No config, use default rollback
		projectDirs, err := getKubernetesProjectDirs(kubernetesDir)
		if err != nil {
			return fmt.Errorf("failed to get project directories: %w", err)
		}

		for _, projectName := range projectDirs {
			projectDir := filepath.Join(kubernetesDir, projectName)
			componentDirs, err := getKubernetesComponentDirs(projectDir)
			if err != nil {
				continue
			}

			for _, componentName := range componentDirs {
				if opts.Component != "" && componentName != opts.Component {
					continue
				}

				componentDir := filepath.Join(projectDir, componentName)
				fmt.Printf("\n📦 Rolling back component: %s/%s\n", projectName, componentName)
				fmt.Printf("   Directory: %s\n", componentDir)

				rollbackCmd := fmt.Sprintf("kubectl delete -k %s", componentDir)

				if opts.DryRun {
					fmt.Printf("   [DRY RUN] Would execute: %s\n", rollbackCmd)
					continue
				}

				cmd := exec.Command("kubectl", "delete", "-k", componentDir)
				if opts.Kubeconfig != "" {
					cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", opts.Kubeconfig))
				}
				if opts.Context != "" {
					cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECTL_CONTEXT=%s", opts.Context))
				}
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				if err := cmd.Run(); err != nil {
					fmt.Printf("❌ Rollback failed for %s/%s: %v\n", projectName, componentName, err)
					if !opts.AutoApprove {
						return fmt.Errorf("rollback failed for %s/%s: %w", projectName, componentName, err)
					}
				} else {
					fmt.Printf("✅ Rollback succeeded for %s/%s\n", projectName, componentName)
				}
			}
		}
	}

	fmt.Println("\n✅ Kubernetes rollback completed")
	return nil
}

// RollbackGitOps executes rollback for GitOps resources
func RollbackGitOps(opts RollbackOptions, cfg config.BlcliConfig, templateLoader *template.Loader, templateArgs renderer.ArgsData) error {
	if cfg.Gitops == nil {
		return fmt.Errorf("no gitops configuration found")
	}

	gitopsDir := filepath.Join(opts.Workspace, "gitops")
	if _, err := os.Stat(gitopsDir); os.IsNotExist(err) {
		return fmt.Errorf("gitops directory not found: %s", gitopsDir)
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🗑️  Rolling back GitOps resources")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Find all app.yaml files
	projectDirs, err := getGitOpsProjectDirs(gitopsDir)
	if err != nil {
		return fmt.Errorf("failed to get project directories: %w", err)
	}

	for _, projectName := range projectDirs {
		projectDir := filepath.Join(gitopsDir, projectName)
		appDirs, err := getGitOpsAppDirs(projectDir)
		if err != nil {
			continue
		}

		for _, appName := range appDirs {
			if opts.Component != "" && appName != opts.Component {
				continue
			}

			appDir := filepath.Join(projectDir, appName)
			appYaml := filepath.Join(appDir, "app.yaml")
			if _, err := os.Stat(appYaml); os.IsNotExist(err) {
				continue
			}

			fmt.Printf("\n📦 Rolling back GitOps application: %s/%s\n", projectName, appName)
			fmt.Printf("   Directory: %s\n", appDir)

			// Parse app.yaml to get namespace
			// For simplicity, use default namespace or extract from app.yaml
			namespace := "argocd" // Default namespace

			rollbackCmd := fmt.Sprintf("kubectl delete application %s -n %s", appName, namespace)

			if opts.DryRun {
				fmt.Printf("   [DRY RUN] Would execute: %s\n", rollbackCmd)
				continue
			}

			cmd := exec.Command("kubectl", "delete", "application", appName, "-n", namespace)
			if opts.Kubeconfig != "" {
				cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", opts.Kubeconfig))
			}
			if opts.Context != "" {
				cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECTL_CONTEXT=%s", opts.Context))
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				fmt.Printf("❌ Rollback failed for %s/%s: %v\n", projectName, appName, err)
				if !opts.AutoApprove {
					return fmt.Errorf("rollback failed for %s/%s: %w", projectName, appName, err)
				}
			} else {
				fmt.Printf("✅ Rollback succeeded for %s/%s\n", projectName, appName)
			}
		}
	}

	fmt.Println("\n✅ GitOps rollback completed")
	return nil
}

// rollbackItem represents an item to rollback
type rollbackItem struct {
	Name     string
	Path     string
	Rollback string
	Type     string // init, project, module
}

// Helper functions
func getKubernetesProjectDirs(kubernetesDir string) ([]string, error) {
	entries, err := os.ReadDir(kubernetesDir)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

func getKubernetesComponentDirs(projectDir string) ([]string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

func getGitOpsProjectDirs(gitopsDir string) ([]string, error) {
	entries, err := os.ReadDir(gitopsDir)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

func getGitOpsAppDirs(projectDir string) ([]string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}
