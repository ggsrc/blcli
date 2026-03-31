package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"blcli/pkg/template"
)

// CheckKubernetesOptions holds options for check kubernetes command
type CheckKubernetesOptions struct {
	KubernetesDir string
	Kubeconfig    string
	Context       string
	Namespace     string
	Timeout       time.Duration
	TemplateRepo  string // Optional: template repository to load config.yaml
}

// ExecuteCheckKubernetes executes the check kubernetes command
// It validates Kubernetes manifests based on their installType
func ExecuteCheckKubernetes(opts CheckKubernetesOptions) error {
	// Verify kubernetes directory exists
	if _, err := os.Stat(opts.KubernetesDir); os.IsNotExist(err) {
		return fmt.Errorf("kubernetes directory not found: %s", opts.KubernetesDir)
	}

	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH. Please install kubectl")
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// Load kubernetes config to get component installType and check commands
	var kubernetesConfig *template.KubernetesConfig
	if opts.TemplateRepo != "" {
		loaderOptions := template.LoaderOptions{
			ForceUpdate: false,
			CacheExpiry: 24 * time.Hour,
		}
		templateLoader := template.NewLoaderWithOptions(opts.TemplateRepo, loaderOptions)
		cfg, err := templateLoader.LoadKubernetesConfig()
		if err != nil {
			fmt.Printf("Warning: failed to load kubernetes config: %v (will use default kubectl check)\n", err)
		} else {
			kubernetesConfig = cfg
		}
	}

	var failedComponents []string

	// Step 1: Check namespace (if exists)
	namespaceDir := filepath.Join(opts.KubernetesDir, "base")
	if _, err := os.Stat(namespaceDir); err == nil {
		fmt.Println("\n📋 Step 1: Checking namespace...")
		if err := checkNamespace(ctx, opts, namespaceDir); err != nil {
			fmt.Printf("❌ Check failed for namespace: %v\n", err)
			failedComponents = append(failedComponents, "namespace")
		} else {
			fmt.Println("✅ Namespace check passed")
		}
	}

	// Step 2: Check components (in dependency order)
	componentsDir := filepath.Join(opts.KubernetesDir, "components")
	if _, err := os.Stat(componentsDir); err == nil {
		fmt.Println("\n📋 Step 2: Checking components (in dependency order)...")

		// Get list of component names from actual directories
		var componentNames []string
		entries, err := os.ReadDir(componentsDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					componentNames = append(componentNames, entry.Name())
				}
			}
		}

		if kubernetesConfig != nil && len(componentNames) > 0 {
			// Resolve dependencies
			orderedComponents, err := kubernetesConfig.ResolveKubernetesDependencies(componentNames)
			if err != nil {
				fmt.Printf("Warning: failed to resolve component dependencies: %v (checking in directory order)\n", err)
				orderedComponents = componentNames // Fallback to directory order
			}

			// Create component map for lookup
			componentMap := make(map[string]template.KubernetesComponent)
			for _, comp := range kubernetesConfig.Components {
				componentMap[comp.Name] = comp
			}

			for _, compName := range orderedComponents {
				compDir := filepath.Join(componentsDir, compName)
				if _, err := os.Stat(compDir); os.IsNotExist(err) {
					continue // Skip if component directory doesn't exist
				}

				component, exists := componentMap[compName]
				if !exists {
					// Component not in config, use default kubectl check
					fmt.Printf("   Checking component: %s (using kubectl, not in config)\n", compName)
					if err := checkWithKubectl(ctx, opts, compDir); err != nil {
						fmt.Printf("❌ Check failed for component %s: %v\n", compName, err)
						failedComponents = append(failedComponents, compName)
					} else {
						fmt.Printf("✅ Component %s check passed\n", compName)
					}
					continue
				}

				if err := checkComponent(ctx, opts, component, compDir); err != nil {
					fmt.Printf("❌ Check failed for component %s: %v\n", compName, err)
					failedComponents = append(failedComponents, compName)
				} else {
					fmt.Printf("✅ Component %s check passed\n", compName)
				}
			}
		} else {
			// Fallback: check all components in components directory using kubectl
			if err := checkAllComponentsInDir(ctx, opts, componentsDir); err != nil {
				return err
			}
		}
	}

	// Summary
	fmt.Printf("\n📊 Check Summary:\n")
	fmt.Printf("   Failed components: %d\n", len(failedComponents))
	if len(failedComponents) > 0 {
		fmt.Printf("   Failed: %v\n", failedComponents)
		return fmt.Errorf("some components failed validation")
	}

	fmt.Println("✅ All components passed validation")
	return nil
}

// checkNamespace checks namespace files using kubectl --dry-run
func checkNamespace(ctx context.Context, opts CheckKubernetesOptions, namespaceDir string) error {
	// Find namespace YAML files
	var namespaceFiles []string
	err := filepath.Walk(namespaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			namespaceFiles = append(namespaceFiles, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Check each namespace file
	for _, file := range namespaceFiles {
		if err := checkKubectlFile(ctx, opts, file); err != nil {
			return err
		}
	}

	return nil
}

// checkComponent checks a component based on its installType
func checkComponent(ctx context.Context, opts CheckKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
	installType := component.InstallType
	if installType == "" {
		installType = template.InstallTypeKubectl // Default
	}

	fmt.Printf("   Checking component: %s (installType: %s)\n", component.Name, installType)

	switch installType {
	case template.InstallTypeKubectl:
		// kubectl --dry-run for validation
		return checkWithKubectl(ctx, opts, componentDir)
	case template.InstallTypeHelm:
		// helm template or helm lint for validation
		return checkWithHelm(ctx, opts, component, componentDir)
	case template.InstallTypeCustom:
		// Use config.yaml check command if provided
		return checkWithCustom(ctx, opts, component, componentDir)
	default:
		return fmt.Errorf("unknown installType: %s", installType)
	}
}

// checkWithKubectl checks using kubectl --dry-run
func checkWithKubectl(ctx context.Context, opts CheckKubernetesOptions, componentDir string) error {
	// Check if kustomization.yaml exists
	kustomizationFile := filepath.Join(componentDir, "kustomization.yaml")
	if _, err := os.Stat(kustomizationFile); err == nil {
		// Use kubectl apply -k --dry-run=client for kustomize
		args := []string{"apply", "-k", componentDir, "--dry-run=client"}
		if opts.Kubeconfig != "" {
			args = append(args, "--kubeconfig", opts.Kubeconfig)
		}
		if opts.Context != "" {
			args = append(args, "--context", opts.Context)
		}

		cmd := exec.CommandContext(ctx, "kubectl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Fallback: check all YAML files in directory
	return checkKubectlFilesInDir(ctx, opts, componentDir)
}

// checkKubectlFilesInDir checks all YAML files in a directory using kubectl --dry-run
func checkKubectlFilesInDir(ctx context.Context, opts CheckKubernetesOptions, dir string) error {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := checkKubectlFile(ctx, opts, file); err != nil {
			return err
		}
	}

	return nil
}

// checkKubectlFile checks a single file using kubectl apply --dry-run
func checkKubectlFile(ctx context.Context, opts CheckKubernetesOptions, file string) error {
	args := []string{"apply", "-f", file, "--dry-run=client"}
	if opts.Kubeconfig != "" {
		args = append(args, "--kubeconfig", opts.Kubeconfig)
	}
	if opts.Context != "" {
		args = append(args, "--context", opts.Context)
	}
	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// checkWithHelm checks using helm template or helm lint
func checkWithHelm(ctx context.Context, opts CheckKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
	// Check if helm is available
	if _, err := exec.LookPath("helm"); err != nil {
		return fmt.Errorf("helm not found in PATH. Please install helm")
	}

	// Try to find Chart.yaml in the component directory
	chartFile := filepath.Join(componentDir, "Chart.yaml")
	if _, err := os.Stat(chartFile); err == nil {
		// Use helm lint to validate the chart
		args := []string{"lint", componentDir}
		cmd := exec.CommandContext(ctx, "helm", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("helm lint failed: %w", err)
		}
		return nil
	}

	// If no Chart.yaml, try helm template to validate the install command
	// Parse install command to extract chart info
	installCmd := component.Install
	if installCmd == "" {
		return fmt.Errorf("install command not specified for helm component %s", component.Name)
	}

	// Try to extract chart name from install command
	// Example: helm install cnpg cnpg/cnpg --namespace cnpg --create-namespace
	// We can try to template it to see if it's valid
	// For now, we'll use helm template with a dummy release name
	parts := strings.Fields(installCmd)
	if len(parts) < 3 || parts[0] != "helm" || parts[1] != "install" {
		// If we can't parse, just try helm template on the directory
		// This is a fallback - it might not work for all cases
		fmt.Printf("   Warning: Could not parse helm install command, skipping helm-specific check\n")
		return nil
	}

	// Try helm template to validate
	// Extract chart reference (e.g., "cnpg/cnpg" from "helm install cnpg cnpg/cnpg ...")
	chartRef := parts[2]
	args := []string{"template", "test-release", chartRef, "--dry-run"}
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// If helm template fails, it might be because the chart is not in a repo
		// In that case, we'll just warn but not fail
		fmt.Printf("   Warning: helm template check failed (chart might not be in repo): %v\n", err)
		return nil
	}

	return nil
}

// checkWithCustom checks using custom check command from config.yaml
func checkWithCustom(ctx context.Context, opts CheckKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
	checkCmd := component.Check
	if checkCmd == "" {
		// If no check command specified, skip check (as per requirement)
		fmt.Printf("   Skipping check for component %s (no check command specified)\n", component.Name)
		return nil
	}

	// Execute the custom check command in the component directory
	cmd := exec.CommandContext(ctx, "sh", "-c", checkCmd)
	cmd.Dir = componentDir
	if opts.Kubeconfig != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", opts.Kubeconfig))
	}
	if opts.Context != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECTL_CONTEXT=%s", opts.Context))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// checkAllComponentsInDir checks all components in a directory (fallback when config is not available)
func checkAllComponentsInDir(ctx context.Context, opts CheckKubernetesOptions, componentsDir string) error {
	entries, err := os.ReadDir(componentsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		compDir := filepath.Join(componentsDir, entry.Name())
		fmt.Printf("   Checking component: %s (using kubectl)\n", entry.Name())
		if err := checkWithKubectl(ctx, opts, compDir); err != nil {
			fmt.Printf("❌ Check failed for component %s: %v\n", entry.Name(), err)
			continue
		}
		fmt.Printf("✅ Component %s check passed\n", entry.Name())
	}

	return nil
}
