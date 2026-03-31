package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	k8s "blcli/pkg/bootstrap/kubernetes"
	"blcli/pkg/template"
)

// ComponentChartMapping maps common component names to their helm charts
var ComponentChartMapping = map[string]string{
	"redis":         "bitnami/redis",
	"postgresql":    "bitnami/postgresql",
	"mysql":         "bitnami/mysql",
	"mongodb":       "bitnami/mongodb",
	"nginx":         "bitnami/nginx",
	"kiali":         "kiali/kiali-server",
	"prometheus":    "prometheus-community/prometheus",
	"grafana":       "grafana/grafana",
	"elasticsearch": "elastic/elasticsearch",
	"kibana":        "elastic/kibana",
}

// detectHelmChart detects helm chart for a component using hybrid approach:
// 1. If explicitChart is provided, use it
// 2. Check if Chart.yaml exists in componentDir (local chart)
// 3. Infer from component name using mapping table
func detectHelmChart(componentDir, componentName, explicitChart string) (string, error) {
	// 1. If explicit chart is provided, use it
	if explicitChart != "" {
		return explicitChart, nil
	}

	// 2. Check if Chart.yaml exists in component directory (local chart)
	chartYaml := filepath.Join(componentDir, "Chart.yaml")
	if _, err := os.Stat(chartYaml); err == nil {
		return componentDir, nil
	}

	// 3. Check if helm-chart subdirectory exists
	helmChartDir := filepath.Join(componentDir, "helm-chart")
	if _, err := os.Stat(helmChartDir); err == nil {
		chartYamlInSubdir := filepath.Join(helmChartDir, "Chart.yaml")
		if _, err := os.Stat(chartYamlInSubdir); err == nil {
			return helmChartDir, nil
		}
	}

	// 4. Infer from component name
	if chart, ok := ComponentChartMapping[componentName]; ok {
		return chart, nil
	}

	// 5. If all else fails, return error
	return "", fmt.Errorf("unable to detect helm chart for component %s: no explicit chart specified, no local Chart.yaml found, and no mapping available", componentName)
}

// inferChartFromComponentName infers helm chart from component name
func inferChartFromComponentName(componentName string) string {
	if chart, ok := ComponentChartMapping[componentName]; ok {
		return chart
	}
	// Default: try bitnami/{componentName}
	return fmt.Sprintf("bitnami/%s", componentName)
}

// ExecuteApplyKubernetes executes the apply kubernetes command
// It applies components in dependency order, supporting different installType (kubectl, helm, custom)
// After each component is applied successfully, it waits ComponentWaitAfterApply (e.g. 30s) before the next component.
func ExecuteApplyKubernetes(opts ApplyKubernetesOptions, templateLoader *template.Loader) error {
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

	// Load kubernetes config to get component installType and dependencies
	var kubernetesConfig *template.KubernetesConfig
	if templateLoader != nil {
		cfg, err := templateLoader.LoadKubernetesConfig()
		if err != nil {
			fmt.Printf("Warning: failed to load kubernetes config: %v (will use default kubectl apply)\n", err)
		} else {
			kubernetesConfig = cfg
			fmt.Println("ℹ️  Using template config for installType and dependency resolution")
		}
	} else {
		fmt.Println("ℹ️  No template repo provided, using default kubectl apply for all components")
	}

	var failedComponents []string

	// Step 1: Apply namespace (if exists) - this will be added to plan later
	namespaceDir := filepath.Join(opts.KubernetesDir, "base")
	hasNamespace := false
	if _, err := os.Stat(namespaceDir); err == nil {
		hasNamespace = true
	}

	// Step 2: Apply components (in dependency order)
	// New structure: kubernetes/{projectName}/{componentName}

	// Get all project directories
	projectEntries, err := os.ReadDir(opts.KubernetesDir)
	if err != nil {
		return fmt.Errorf("failed to read kubernetes directory: %w", err)
	}

	// Collect all components across all projects
	type componentInfo struct {
		projectName   string
		componentName string
		componentDir  string
	}
	var allComponents []componentInfo
	var allComponentNames []string
	componentDirs := make(map[string]componentInfo) // componentName -> componentInfo

	for _, projectEntry := range projectEntries {
		if !projectEntry.IsDir() {
			continue
		}
		projectName := projectEntry.Name()

		// Skip base directory if it exists
		if projectName == "base" {
			continue
		}

		// If ProjectName is set, only process that project
		if opts.ProjectName != "" && projectName != opts.ProjectName {
			continue
		}

		projectDir := filepath.Join(opts.KubernetesDir, projectName)
		componentEntries, err := os.ReadDir(projectDir)
		if err != nil {
			fmt.Printf("Warning: failed to read project directory %s: %v\n", projectName, err)
			continue
		}

		for _, componentEntry := range componentEntries {
			if !componentEntry.IsDir() {
				continue
			}
			componentName := componentEntry.Name()
			componentDir := filepath.Join(projectDir, componentName)

			info := componentInfo{
				projectName:   projectName,
				componentName: componentName,
				componentDir:  componentDir,
			}
			allComponents = append(allComponents, info)

			// Use first occurrence of component (if same component exists in multiple projects)
			if _, exists := componentDirs[componentName]; !exists {
				componentDirs[componentName] = info
				allComponentNames = append(allComponentNames, componentName)
			}
		}
	}

	if opts.ProjectName != "" && len(allComponents) == 0 {
		return fmt.Errorf("project %q not found or has no components. List project directories under: %s", opts.ProjectName, opts.KubernetesDir)
	}

	// Build execution plan
	var planItems []PlanItem
	step := 1

	// Add namespace to plan if exists
	if _, err := os.Stat(namespaceDir); err == nil {
		planItems = append(planItems, PlanItem{
			Step:         step,
			Name:         "namespace",
			Directory:    namespaceDir,
			Command:      "kubectl",
			Args:         []string{"apply", "-k", namespaceDir},
			Dependencies: []string{},
			Description:  "Apply Kubernetes namespace",
		})
		step++
	}

	if len(allComponentNames) == 0 {
		fmt.Println("   No components found to apply")
		return nil
	}

	// Normalize directory names for dependency resolution: config.yaml uses logical names
	// (e.g. external-secrets-operator) while output dirs may use prefixed names (e.g. 0-external-secrets-operator).
	normalizedNames := make([]string, 0, len(allComponentNames))
	logicalToOriginal := make(map[string]string, len(allComponentNames))
	for _, name := range allComponentNames {
		logical := k8s.NormalizeComponentName(name)
		normalizedNames = append(normalizedNames, logical)
		if _, exists := logicalToOriginal[logical]; !exists {
			logicalToOriginal[logical] = name
		}
	}

	// Resolve dependencies and build execution plan
	var orderedComponents []string
	var componentMap map[string]template.KubernetesComponent

	if kubernetesConfig != nil {
		// Resolve dependencies using logical names
		var err error
		orderedComponents, err = kubernetesConfig.ResolveKubernetesDependencies(normalizedNames)
		if err != nil {
			fmt.Printf("Warning: failed to resolve component dependencies: %v (applying in directory order)\n", err)
			orderedComponents = normalizedNames
			for _, n := range normalizedNames {
				if logicalToOriginal[n] == "" {
					logicalToOriginal[n] = n
				}
			}
		}

		// Create component map for lookup
		componentMap = make(map[string]template.KubernetesComponent)
		allComponentsMap := kubernetesConfig.GetAllComponents()
		for name, comp := range allComponentsMap {
			componentMap[name] = comp
		}
	} else {
		orderedComponents = normalizedNames
		for _, n := range normalizedNames {
			if logicalToOriginal[n] == "" {
				logicalToOriginal[n] = n
			}
		}
		componentMap = make(map[string]template.KubernetesComponent)
	}

	// Build execution plan items for components (orderedComponents are logical names)
	for _, logicalName := range orderedComponents {
		originalName := logicalToOriginal[logicalName]
		if originalName == "" {
			originalName = logicalName
		}
		info, exists := componentDirs[originalName]
		if !exists {
			continue
		}

		component, configExists := componentMap[logicalName]
		var command string
		var args []string
		var dependencies []string

		if configExists {
			dependencies = component.Dependencies
			switch component.InstallType {
			case template.InstallTypeHelm:
				releaseName := logicalName
				namespace := component.Namespace
				if namespace == "" {
					namespace = logicalName
				}
				chart := component.Chart
				if chart == "" {
					chart = fmt.Sprintf("bitnami/%s", logicalName) // Default fallback
				}
				command = "helm"
				args = []string{"install", releaseName, chart, "-n", namespace}
			case template.InstallTypeCustom:
				if component.Install != "" {
					// Parse custom install command
					parts := strings.Fields(component.Install)
					if len(parts) > 0 {
						command = parts[0]
						args = parts[1:]
					} else {
						command = "bash"
						args = []string{"install"}
					}
				} else {
					command = "kubectl"
					args = []string{"apply", "-k", info.componentDir}
				}
			default: // kubectl
				command = "kubectl"
				args = []string{"apply", "-k", info.componentDir}
			}
		} else {
			command = "kubectl"
			args = []string{"apply", "-k", info.componentDir}
		}

		planItems = append(planItems, PlanItem{
			Step:         step,
			Name:         fmt.Sprintf("%s/%s", info.projectName, logicalName),
			Directory:    info.componentDir,
			Command:      command,
			Args:         args,
			Dependencies: dependencies,
			Description:  fmt.Sprintf("Apply Kubernetes component %s in project %s", logicalName, info.projectName),
		})
		step++
	}

	// Build and print execution plan
	plan := ExecutionPlan{
		Module: "kubernetes",
		Items:  planItems,
		DryRun: opts.DryRun,
	}
	PrintExecutionPlan(plan)

	// If dry-run, exit here
	if opts.DryRun {
		return nil
	}

	// Step 1: Apply namespace (if exists)
	if hasNamespace {
		fmt.Println("\n📋 Step 1: Applying namespace...")
		if err := applyNamespace(ctx, opts, namespaceDir); err != nil {
			fmt.Printf("❌ Failed to apply namespace: %v\n", err)
			failedComponents = append(failedComponents, "namespace")
		} else {
			fmt.Println("✅ Namespace applied successfully")
		}
	}

	// Step 2: Apply components in dependency order (with parallel batches when possible)
	fmt.Println("\n📋 Step 2: Applying components (in dependency order, parallel within batches)...")
	if kubernetesConfig != nil {
		batches := buildKubernetesBatches(orderedComponents, componentMap)
		for batchIndex, batch := range batches {
			if len(batch) == 0 {
				continue
			}

			fmt.Printf("\n▶ Batch %d: %v\n", batchIndex+1, batch)

			var wg sync.WaitGroup
			batchErrorsMu := &sync.Mutex{}

			for _, logicalName := range batch {
				logicalName := logicalName

				originalName := logicalToOriginal[logicalName]
				if originalName == "" {
					originalName = logicalName
				}
				info, exists := componentDirs[originalName]
				if !exists {
					fmt.Printf("Warning: component %s not found in any project directory, skipping\n", logicalName)
					continue
				}

				component, configExists := componentMap[logicalName]

				wg.Add(1)
				go func() {
					defer wg.Done()

					var err error
					if !configExists {
						// Component not in config, use default kubectl apply
						fmt.Printf("   Applying component: %s (project: %s, using kubectl, not in config)\n", logicalName, info.projectName)
						err = applyWithKubectl(ctx, opts, info.componentDir)
					} else {
						err = applyComponent(ctx, opts, component, info.componentDir)
						if err == nil && component.Check != "" {
							if vErr := validateComponent(ctx, opts, component, info.componentDir); vErr != nil {
								err = fmt.Errorf("validation failed: %w", vErr)
							}
						}
					}

					if err != nil {
						fmt.Printf("❌ Failed to apply component %s (project: %s): %v\n", logicalName, info.projectName, err)
						batchErrorsMu.Lock()
						failedComponents = append(failedComponents, logicalName)
						batchErrorsMu.Unlock()
					} else {
						fmt.Printf("✅ Component %s (project: %s) applied successfully\n", logicalName, info.projectName)
					}
				}()
			}

			wg.Wait()
		}
	} else {
		// Fallback: apply all components in directory order (no parallelism, no validate)
		for _, info := range allComponents {
			fmt.Printf("   Applying component: %s (project: %s, using kubectl)\n", info.componentName, info.projectName)
			if err := applyWithKubectl(ctx, opts, info.componentDir); err != nil {
				fmt.Printf("❌ Failed to apply component %s: %v\n", info.componentName, err)
				failedComponents = append(failedComponents, info.componentName)
			} else {
				fmt.Printf("✅ Component %s applied successfully\n", info.componentName)
			}
		}
	}

	// Wait for resources if requested
	if opts.Wait && !opts.DryRun && len(failedComponents) == 0 {
		fmt.Println("\n⏳ Waiting for resources to be ready...")
		time.Sleep(2 * time.Second)
		fmt.Println("   ✓ Resources ready")
	}

	// Summary
	fmt.Printf("\n📊 Apply Summary:\n")
	fmt.Printf("   Failed components: %d\n", len(failedComponents))
	if len(failedComponents) > 0 {
		fmt.Printf("   Failed: %v\n", failedComponents)
		return fmt.Errorf("some components failed to apply")
	}

	return nil
}

// waitAfterComponentApply waits after a component is applied successfully before proceeding to the next.
// Uses fixed delay (ComponentWaitAfterApply, e.g. 30s); later can be replaced with readiness check.
func waitAfterComponentApply(opts ApplyKubernetesOptions, currentIndex, totalComponents int) {
	if opts.DryRun || opts.ComponentWaitAfterApply <= 0 {
		return
	}
	// Only wait if there is a next component
	if currentIndex+1 >= totalComponents {
		return
	}
	fmt.Printf("   ⏳ Waiting %v before next component...\n", opts.ComponentWaitAfterApply)
	time.Sleep(opts.ComponentWaitAfterApply)
}

// validateComponent runs post-apply validation for a component if Check is configured.
// It executes the Check command in the component directory, propagating kube-related options.
func validateComponent(ctx context.Context, opts ApplyKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
	if opts.DryRun {
		// In dry-run mode, we only describe the plan; validation is not executed.
		return nil
	}
	if component.Check == "" {
		return nil
	}

	fmt.Printf("   🔍 Validating component: %s\n", component.Name)

	cmd := exec.CommandContext(ctx, "sh", "-c", component.Check)
	cmd.Dir = componentDir
	cmd.Env = os.Environ()
	if opts.Kubeconfig != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", opts.Kubeconfig))
	}
	if opts.Context != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECTL_CONTEXT=%s", opts.Context))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Printf("   ✅ Validation succeeded for component: %s\n", component.Name)
	return nil
}

// buildKubernetesBatches groups components into parallelizable batches based on dependencies.
// Components in the same batch have all their dependencies satisfied by earlier batches.
func buildKubernetesBatches(orderedComponents []string, componentMap map[string]template.KubernetesComponent) [][]string {
	if len(orderedComponents) == 0 {
		return nil
	}

	// Depth-first computation of dependency depth for each component.
	depthMemo := make(map[string]int)
	visiting := make(map[string]bool)

	var computeDepth func(name string) int
	computeDepth = func(name string) int {
		if d, ok := depthMemo[name]; ok {
			return d
		}
		if visiting[name] {
			// Cycle should not happen (already validated when resolving deps); treat as depth 0.
			return 0
		}
		visiting[name] = true
		defer delete(visiting, name)

		comp, ok := componentMap[name]
		if !ok || len(comp.Dependencies) == 0 {
			depthMemo[name] = 0
			return 0
		}

		maxDepDepth := 0
		for _, dep := range comp.Dependencies {
			d := computeDepth(dep)
			if d > maxDepDepth {
				maxDepDepth = d
			}
		}
		depth := maxDepDepth + 1
		depthMemo[name] = depth
		return depth
	}

	maxDepth := 0
	for _, name := range orderedComponents {
		d := computeDepth(name)
		if d > maxDepth {
			maxDepth = d
		}
	}

	batches := make([][]string, maxDepth+1)
	for _, name := range orderedComponents {
		d := depthMemo[name]
		batches[d] = append(batches[d], name)
	}
	return batches
}

// applyNamespace applies namespace files
func applyNamespace(ctx context.Context, opts ApplyKubernetesOptions, namespaceDir string) error {
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

	// Apply each namespace file
	for _, file := range namespaceFiles {
		if err := applyKubectlFile(ctx, opts, file); err != nil {
			return err
		}
	}

	return nil
}

// applyComponent applies a component based on its installType
func applyComponent(ctx context.Context, opts ApplyKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
	installType := component.InstallType
	if installType == "" {
		installType = template.InstallTypeKubectl // Default
	}

	fmt.Printf("   Applying component: %s (installType: %s)\n", component.Name, installType)

	switch installType {
	case template.InstallTypeKubectl:
		// kubectl apply -k <component-dir>
		return applyWithKubectl(ctx, opts, componentDir)
	case template.InstallTypeHelm:
		// helm install <name> <chart> --namespace <namespace> --create-namespace
		return applyWithHelm(ctx, opts, component, componentDir)
	case template.InstallTypeCustom:
		// Use config.yaml install command
		return applyWithCustom(ctx, opts, component, componentDir)
	default:
		return fmt.Errorf("unknown installType: %s", installType)
	}
}

// applyWithKubectl applies using kubectl apply -k (kustomize) or kubectl apply -f
func applyWithKubectl(ctx context.Context, opts ApplyKubernetesOptions, componentDir string) error {
	// Check if kustomization.yaml exists
	kustomizationFile := filepath.Join(componentDir, "kustomization.yaml")
	if _, err := os.Stat(kustomizationFile); err == nil {
		// Use kubectl apply -k for kustomize
		args := []string{"apply", "-k", componentDir}
		if opts.Context != "" {
			args = append(args, "--context", opts.Context)
		}
		if opts.DryRun {
			args = append(args, "--dry-run=client")
		}

		cmd := exec.CommandContext(ctx, "kubectl", args...)
		if opts.Kubeconfig != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", opts.Kubeconfig))
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Fallback: apply all YAML files in directory using kubectl apply -f {dir}
	// kubectl apply -f accepts a directory and will apply all yaml/yml files in it
	args := []string{"apply", "-f", componentDir}
	if opts.Context != "" {
		args = append(args, "--context", opts.Context)
	}
	if opts.DryRun {
		args = append(args, "--dry-run=client")
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if opts.Kubeconfig != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", opts.Kubeconfig))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// applyKubectlFilesInDir applies all YAML files in a directory using kubectl apply -f
func applyKubectlFilesInDir(ctx context.Context, opts ApplyKubernetesOptions, dir string) error {
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
		if err := applyKubectlFile(ctx, opts, file); err != nil {
			return err
		}
	}

	return nil
}

// applyKubectlFile applies a single file using kubectl apply -f
func applyKubectlFile(ctx context.Context, opts ApplyKubernetesOptions, file string) error {
	args := []string{"apply", "-f", file}
	if opts.DryRun {
		args = append(args, "--dry-run=client")
	}
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

// applyWithHelm applies using helm install with hybrid chart detection
func applyWithHelm(ctx context.Context, opts ApplyKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
	// Check if helm is available
	if _, err := exec.LookPath("helm"); err != nil {
		return fmt.Errorf("helm not found in PATH. Please install helm")
	}

	// Detect helm chart using hybrid approach
	chart, err := detectHelmChart(componentDir, component.Name, component.Chart)
	if err != nil {
		return fmt.Errorf("failed to detect helm chart for component %s: %w", component.Name, err)
	}

	// Determine namespace (default to component name if not specified)
	namespace := component.Namespace
	if namespace == "" {
		namespace = component.Name
	}

	// Build helm install command
	args := []string{"install", component.Name, chart, "--namespace", namespace, "--create-namespace"}
	if opts.Context != "" {
		args = append(args, "--kube-context", opts.Context)
	}
	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = os.Environ()
	if opts.Kubeconfig != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", opts.Kubeconfig))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// applyWithCustom applies using custom install command from config.yaml
func applyWithCustom(ctx context.Context, opts ApplyKubernetesOptions, component template.KubernetesComponent, componentDir string) error {
	if component.Install == "" {
		return fmt.Errorf("install command not specified for custom component %s", component.Name)
	}

	// Switch kubectl context before executing install script so that
	// helm/kubectl inside the script target the correct cluster.
	if opts.Context != "" {
		switchCmd := exec.CommandContext(ctx, "kubectl", "config", "use-context", opts.Context)
		switchCmd.Stdout = os.Stdout
		switchCmd.Stderr = os.Stderr
		if err := switchCmd.Run(); err != nil {
			return fmt.Errorf("failed to switch kubectl context to %s: %w", opts.Context, err)
		}
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", component.Install)
	cmd.Dir = componentDir
	cmd.Env = os.Environ()
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

// applyAllComponentsInDir applies all components in a directory (fallback when config is not available)
func applyAllComponentsInDir(ctx context.Context, opts ApplyKubernetesOptions, componentsDir string) error {
	entries, err := os.ReadDir(componentsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		compDir := filepath.Join(componentsDir, entry.Name())
		fmt.Printf("   Applying component: %s (using kubectl)\n", entry.Name())
		if err := applyWithKubectl(ctx, opts, compDir); err != nil {
			fmt.Printf("❌ Failed to apply component %s: %v\n", entry.Name(), err)
			continue
		}
		fmt.Printf("✅ Component %s applied successfully\n", entry.Name())
	}

	return nil
}
