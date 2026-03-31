package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"blcli/pkg/config"
	"blcli/pkg/renderer"
	"blcli/pkg/state"
	"blcli/pkg/template"
)

// StatusOptions holds options for status command
type StatusOptions struct {
	Type         string   // "terraform", "kubernetes", "gitops", "all"
	ArgsPaths    []string // Required: args file paths
	Workspace    string   // Optional: workspace path (default from args)
	Format       string   // "table", "json", "yaml" (default: "table")
	Kubeconfig   string   // Optional: kubeconfig path
	Context      string   // Optional: Kubernetes context
	TemplateRepo string   // Optional: template repo for config loading
}

// ExecuteStatus executes the status command
func ExecuteStatus(opts StatusOptions) error {
	// Args file is required
	if len(opts.ArgsPaths) == 0 {
		return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
	}

	// Load args from files
	var allArgs []renderer.ArgsData
	for _, argsPath := range opts.ArgsPaths {
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

	// Determine workspace
	workspace := opts.Workspace
	if workspace == "" {
		workspace = config.WorkspacePath(cfg.Global)
	}

	// Determine modules to check
	modules := []string{}
	if opts.Type == "all" || opts.Type == "" {
		modules = []string{"terraform", "kubernetes", "gitops"}
	} else {
		modules = []string{opts.Type}
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
			fmt.Printf("Warning: failed to sync template cache: %v\n", err)
		}
	}

	// Build status result
	result := state.StatusResult{
		Type:        opts.Type,
		GeneratedAt: time.Now(),
	}

	// Check each module
	for _, module := range modules {
		switch module {
		case "terraform":
			if cfg.Terraform != nil {
				tfStatus, err := StatusTerraform(workspace, cfg.Terraform)
				if err != nil {
					fmt.Printf("Warning: failed to check terraform status: %v\n", err)
				} else {
					result.Terraform = tfStatus
				}
			}
		case "kubernetes":
			if cfg.Kubernetes != nil {
				k8sStatus, err := StatusKubernetes(workspace, cfg.Kubernetes, opts.Kubeconfig, opts.Context, templateLoader)
				if err != nil {
					fmt.Printf("Warning: failed to check kubernetes status: %v\n", err)
				} else {
					result.Kubernetes = k8sStatus
				}
			}
		case "gitops":
			if cfg.Gitops != nil {
				gitopsStatus, err := StatusGitOps(workspace, cfg.Gitops, opts.Kubeconfig, opts.Context)
				if err != nil {
					fmt.Printf("Warning: failed to check gitops status: %v\n", err)
				} else {
					result.GitOps = gitopsStatus
				}
			}
		}
	}

	// Calculate summary
	result.Summary = calculateSummary(&result)

	// Output result
	return outputStatus(&result, opts.Format)
}

// StatusTerraform checks Terraform resources status
func StatusTerraform(workspace string, terraformConfig *config.TerraformConfig) (*state.TerraformStatus, error) {
	tfStatus := &state.TerraformStatus{
		InitDirs: []state.TerraformDirStatus{},
		Projects: []state.TerraformDirStatus{},
		Modules:  []state.TerraformDirStatus{},
	}

	terraformDir := filepath.Join(workspace, "terraform")
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		return tfStatus, nil // No terraform directory, return empty status
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check init directories
	initDir := filepath.Join(terraformDir, "init")
	if _, err := os.Stat(initDir); err == nil {
		initDirs, err := getSortedInitDirectories(initDir)
		if err == nil {
			for _, initSubDir := range initDirs {
				initSubDirPath := filepath.Join(initDir, initSubDir)
				dirStatus := checkTerraformDirStatus(ctx, initSubDirPath, initSubDir)
				tfStatus.InitDirs = append(tfStatus.InitDirs, dirStatus)
			}
		}
	}

	// Check project directories
	terraformBaseDir := filepath.Join(terraformDir, "gcp")
	if _, err := os.Stat(terraformBaseDir); err == nil {
		projectDirs, err := getTerraformDirectories(terraformBaseDir)
		if err == nil {
			for _, projectName := range projectDirs {
				projectDir := filepath.Join(terraformBaseDir, projectName)
				dirStatus := checkTerraformDirStatus(ctx, projectDir, projectName)
				tfStatus.Projects = append(tfStatus.Projects, dirStatus)
			}
		}
	}

	// Calculate summary
	tfStatus.Summary = calculateTerraformSummary(tfStatus)

	return tfStatus, nil
}

// checkTerraformDirStatus checks status of a single Terraform directory
func checkTerraformDirStatus(ctx context.Context, dirPath, name string) state.TerraformDirStatus {
	status := state.TerraformDirStatus{
		Name:   name,
		Path:   dirPath,
		Status: "not_initialized",
	}

	// Check if terraform is initialized (terraform.tfstate exists or .terraform directory exists)
	terraformState := filepath.Join(dirPath, "terraform.tfstate")
	terraformDir := filepath.Join(dirPath, ".terraform")
	if _, err := os.Stat(terraformState); os.IsNotExist(err) {
		if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
			return status // Not initialized
		}
	}

	// Try to run terraform show -json
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(dirPath); err != nil {
		status.Status = "error"
		status.ErrorMessage = fmt.Sprintf("failed to change directory: %v", err)
		return status
	}

	cmd := exec.CommandContext(ctx, "terraform", "show", "-json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		status.Status = "error"
		status.ErrorMessage = stderr.String()
		if status.ErrorMessage == "" {
			status.ErrorMessage = err.Error()
		}
		return status
	}

	// Parse terraform show JSON output
	var tfState struct {
		FormatVersion    string `json:"format_version"`
		TerraformVersion string `json:"terraform_version"`
		Values           struct {
			RootModule struct {
				Resources []struct {
					Type      string   `json:"type"`
					Mode      string   `json:"mode"`
					Address   string   `json:"address"`
					DependsOn []string `json:"depends_on,omitempty"`
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &tfState); err != nil {
		status.Status = "error"
		status.ErrorMessage = fmt.Sprintf("failed to parse terraform state: %v", err)
		return status
	}

	// Count resources
	status.Status = "initialized"
	status.Resources = len(tfState.Values.RootModule.Resources)
	status.Created = status.Resources // Assume all are created (we don't track changes/destroyed from state)
	status.Changed = 0
	status.Destroyed = 0

	// Try to get last updated time from terraform.tfstate file
	if info, err := os.Stat(terraformState); err == nil {
		status.LastUpdated = info.ModTime()
	}

	return status
}

// calculateTerraformSummary calculates Terraform status summary
func calculateTerraformSummary(tfStatus *state.TerraformStatus) state.TerraformSummary {
	summary := state.TerraformSummary{
		Status: "unknown",
	}

	allDirs := []state.TerraformDirStatus{}
	allDirs = append(allDirs, tfStatus.InitDirs...)
	allDirs = append(allDirs, tfStatus.Projects...)
	allDirs = append(allDirs, tfStatus.Modules...)

	summary.TotalDirs = len(allDirs)
	for _, dir := range allDirs {
		if dir.Status == "initialized" {
			summary.Initialized++
			summary.TotalResources += dir.Resources
		} else {
			summary.NotInitialized++
		}
	}

	// Determine overall status
	if summary.TotalDirs == 0 {
		summary.Status = "unknown"
	} else if summary.NotInitialized == 0 {
		summary.Status = "healthy"
	} else if summary.Initialized > 0 {
		summary.Status = "degraded"
	} else {
		summary.Status = "unknown"
	}

	return summary
}

// StatusKubernetes checks Kubernetes resources status
func StatusKubernetes(workspace string, k8sConfig *config.ProjectConfig, kubeconfig, contextName string, templateLoader *template.Loader) (*state.KubernetesStatus, error) {
	k8sStatus := &state.KubernetesStatus{
		Projects: []state.KubernetesProjectStatus{},
	}

	kubernetesDir := filepath.Join(workspace, "kubernetes")
	if _, err := os.Stat(kubernetesDir); os.IsNotExist(err) {
		return k8sStatus, nil // No kubernetes directory, return empty status
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get all project directories
	projectEntries, err := os.ReadDir(kubernetesDir)
	if err != nil {
		return k8sStatus, fmt.Errorf("failed to read kubernetes directory: %w", err)
	}

	// Load kubernetes config for component info
	var kubernetesConfig *template.KubernetesConfig
	if templateLoader != nil {
		cfg, err := templateLoader.LoadKubernetesConfig()
		if err == nil {
			kubernetesConfig = cfg
		}
	}

	for _, projectEntry := range projectEntries {
		if !projectEntry.IsDir() {
			continue
		}
		projectName := projectEntry.Name()

		// Skip base directory
		if projectName == "base" {
			continue
		}

		projectDir := filepath.Join(kubernetesDir, projectName)
		projectStatus := state.KubernetesProjectStatus{
			Name:       projectName,
			Components: []state.KubernetesComponentStatus{},
		}

		// Get components in this project
		componentEntries, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		for _, componentEntry := range componentEntries {
			if !componentEntry.IsDir() {
				continue
			}
			componentName := componentEntry.Name()
			componentDir := filepath.Join(projectDir, componentName)

			componentStatus := checkKubernetesComponentStatus(ctx, componentDir, componentName, kubeconfig, contextName, kubernetesConfig)
			projectStatus.Components = append(projectStatus.Components, componentStatus)
		}

		// Calculate project summary
		projectStatus.Summary = calculateKubernetesComponentSummary(projectStatus.Components)
		k8sStatus.Projects = append(k8sStatus.Projects, projectStatus)
	}

	// Calculate overall summary
	k8sStatus.Summary = calculateKubernetesSummary(k8sStatus)

	return k8sStatus, nil
}

// checkKubernetesComponentStatus checks status of a Kubernetes component
func checkKubernetesComponentStatus(ctx context.Context, componentDir, componentName, kubeconfig, contextName string, k8sConfig *template.KubernetesConfig) state.KubernetesComponentStatus {
	status := state.KubernetesComponentStatus{
		Name:         componentName,
		Path:         componentDir,
		Status:       "unknown",
		Deployments:  state.ResourceStatus{},
		StatefulSets: state.ResourceStatus{},
		Services:     state.ResourceStatus{},
	}

	// Try to get namespace from namespace.yaml if exists
	namespaceFile := filepath.Join(componentDir, "namespace.yaml")
	if data, err := os.ReadFile(namespaceFile); err == nil {
		// Simple parsing to extract namespace name
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, "name:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					status.Namespace = parts[1]
					break
				}
			}
		}
	}

	// If namespace not found, try to infer from component name or use default
	if status.Namespace == "" {
		if k8sConfig != nil {
			// Try to find component in config
			for _, comp := range k8sConfig.Components {
				if comp.Name == componentName && comp.Namespace != "" {
					status.Namespace = comp.Namespace
					break
				}
			}
		}
		if status.Namespace == "" {
			status.Namespace = componentName // Default to component name
		}
	}

	// Check kubectl get deployments
	status.Deployments = checkKubernetesResources(ctx, "deployment", status.Namespace, kubeconfig, contextName)
	status.StatefulSets = checkKubernetesResources(ctx, "statefulset", status.Namespace, kubeconfig, contextName)
	status.Services = checkKubernetesResources(ctx, "service", status.Namespace, kubeconfig, contextName)

	// Determine overall component status
	if status.Deployments.Total == 0 && status.StatefulSets.Total == 0 {
		status.Status = "not_found"
	} else if status.Deployments.Failed > 0 || status.StatefulSets.Failed > 0 {
		status.Status = "degraded"
	} else if (status.Deployments.Total > 0 && status.Deployments.Ready < status.Deployments.Total) ||
		(status.StatefulSets.Total > 0 && status.StatefulSets.Ready < status.StatefulSets.Total) {
		status.Status = "degraded"
	} else {
		status.Status = "healthy"
	}

	return status
}

// checkKubernetesResources checks status of Kubernetes resources of a specific type
func checkKubernetesResources(ctx context.Context, resourceType, namespace, kubeconfig, contextName string) state.ResourceStatus {
	status := state.ResourceStatus{}

	args := []string{"get", resourceType, "-n", namespace, "--no-headers"}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if kubeconfig != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Resource type might not exist or namespace not found
		return status
	}

	// Parse kubectl output
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		status.Total++

		// Parse status from kubectl output
		// Format: NAME READY UP-TO-DATE AVAILABLE AGE
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			// Check READY status (e.g., "2/2" means 2 ready out of 2 desired)
			readyParts := strings.Split(fields[1], "/")
			if len(readyParts) == 2 {
				var ready, desired int
				fmt.Sscanf(readyParts[0], "%d", &ready)
				fmt.Sscanf(readyParts[1], "%d", &desired)
				if ready == desired {
					status.Ready++
				} else if ready > 0 {
					status.Pending++
				} else {
					status.Failed++
				}
			}
		}
	}

	return status
}

// calculateKubernetesComponentSummary calculates component-level summary
func calculateKubernetesComponentSummary(components []state.KubernetesComponentStatus) state.KubernetesComponentSummary {
	summary := state.KubernetesComponentSummary{
		TotalComponents: len(components),
	}

	for _, comp := range components {
		switch comp.Status {
		case "healthy":
			summary.Healthy++
		case "degraded":
			summary.Degraded++
		case "not_found":
			summary.NotFound++
		}
	}

	return summary
}

// calculateKubernetesSummary calculates Kubernetes status summary
func calculateKubernetesSummary(k8sStatus *state.KubernetesStatus) state.KubernetesSummary {
	summary := state.KubernetesSummary{
		TotalProjects: len(k8sStatus.Projects),
		Status:        "unknown",
	}

	for _, project := range k8sStatus.Projects {
		summary.TotalComponents += project.Summary.TotalComponents
		summary.Healthy += project.Summary.Healthy
		summary.Degraded += project.Summary.Degraded
	}

	// Determine overall status
	if summary.TotalComponents == 0 {
		summary.Status = "unknown"
	} else if summary.Degraded == 0 {
		summary.Status = "healthy"
	} else if summary.Healthy > 0 {
		summary.Status = "degraded"
	} else {
		summary.Status = "degraded"
	}

	return summary
}

// StatusGitOps checks GitOps/ArgoCD Application status
func StatusGitOps(workspace string, gitopsConfig *config.ProjectConfig, kubeconfig, contextName string) (*state.GitOpsStatus, error) {
	gitopsStatus := &state.GitOpsStatus{
		Projects: []state.GitOpsProjectStatus{},
	}

	gitopsDir := filepath.Join(workspace, "gitops")
	if _, err := os.Stat(gitopsDir); os.IsNotExist(err) {
		return gitopsStatus, nil // No gitops directory, return empty status
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get all project directories
	projectEntries, err := os.ReadDir(gitopsDir)
	if err != nil {
		return gitopsStatus, fmt.Errorf("failed to read gitops directory: %w", err)
	}

	for _, projectEntry := range projectEntries {
		if !projectEntry.IsDir() {
			continue
		}
		projectName := projectEntry.Name()

		projectDir := filepath.Join(gitopsDir, projectName)
		projectStatus := state.GitOpsProjectStatus{
			Name:         projectName,
			Applications: []state.GitOpsAppStatus{},
		}

		// Find all app.yaml files in this project
		err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.Name() == "app.yaml" {
				appStatus := checkGitOpsAppStatus(ctx, path, kubeconfig, contextName)
				projectStatus.Applications = append(projectStatus.Applications, appStatus)
			}
			return nil
		})

		if err == nil {
			// Calculate project summary
			projectStatus.Summary = calculateGitOpsAppSummary(projectStatus.Applications)
			gitopsStatus.Projects = append(gitopsStatus.Projects, projectStatus)
		}
	}

	// Calculate overall summary
	gitopsStatus.Summary = calculateGitOpsSummary(gitopsStatus)

	return gitopsStatus, nil
}

// checkGitOpsAppStatus checks status of an ArgoCD Application
func checkGitOpsAppStatus(ctx context.Context, appYamlPath, kubeconfig, contextName string) state.GitOpsAppStatus {
	status := state.GitOpsAppStatus{
		Name:         "",
		Namespace:    "argocd", // Default ArgoCD namespace
		SyncStatus:   "Unknown",
		HealthStatus: "Unknown",
	}

	// Try to read app.yaml to get application name and namespace
	if data, err := os.ReadFile(appYamlPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "name:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					status.Name = parts[1]
				}
			}
			if strings.HasPrefix(line, "namespace:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					status.Namespace = parts[1]
				}
			}
			if status.Name != "" && status.Namespace != "" {
				break
			}
		}
	}

	if status.Name == "" {
		// Try to infer from directory name
		dir := filepath.Dir(appYamlPath)
		status.Name = filepath.Base(dir)
	}

	// Check kubectl get application
	args := []string{"get", "application", status.Name, "-n", status.Namespace, "-o", "json"}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if kubeconfig != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		status.ErrorMessage = stderr.String()
		if status.ErrorMessage == "" {
			status.ErrorMessage = err.Error()
		}
		return status
	}

	// Parse ArgoCD Application JSON
	var app struct {
		Status struct {
			Sync struct {
				Status   string `json:"status"`
				Revision string `json:"revision"`
			} `json:"sync"`
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
			History []struct {
				ID         int       `json:"id"`
				Revision   string    `json:"revision"`
				DeployedAt time.Time `json:"deployedAt"`
			} `json:"history"`
		} `json:"status"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &app); err == nil {
		status.SyncStatus = app.Status.Sync.Status
		status.HealthStatus = app.Status.Health.Status
		status.Revision = app.Status.Sync.Revision
		if len(app.Status.History) > 0 {
			status.LastSynced = app.Status.History[0].DeployedAt
		}
	}

	return status
}

// calculateGitOpsAppSummary calculates application-level summary
func calculateGitOpsAppSummary(apps []state.GitOpsAppStatus) state.GitOpsAppSummary {
	summary := state.GitOpsAppSummary{
		TotalApps: len(apps),
	}

	for _, app := range apps {
		if app.SyncStatus == "Synced" {
			summary.Synced++
		} else if app.SyncStatus == "OutOfSync" {
			summary.OutOfSync++
		}
		if app.HealthStatus == "Healthy" {
			summary.Healthy++
		} else if app.HealthStatus == "Degraded" {
			summary.Degraded++
		}
	}

	return summary
}

// calculateGitOpsSummary calculates GitOps status summary
func calculateGitOpsSummary(gitopsStatus *state.GitOpsStatus) state.GitOpsSummary {
	summary := state.GitOpsSummary{
		TotalProjects: len(gitopsStatus.Projects),
		Status:        "unknown",
	}

	for _, project := range gitopsStatus.Projects {
		summary.TotalApps += project.Summary.TotalApps
		summary.Synced += project.Summary.Synced
		summary.OutOfSync += project.Summary.OutOfSync
	}

	// Determine overall status
	if summary.TotalApps == 0 {
		summary.Status = "unknown"
	} else if summary.OutOfSync == 0 {
		summary.Status = "healthy"
	} else if summary.Synced > 0 {
		summary.Status = "degraded"
	} else {
		summary.Status = "degraded"
	}

	return summary
}

// calculateSummary calculates overall status summary
func calculateSummary(result *state.StatusResult) state.StatusSummary {
	summary := state.StatusSummary{
		OverallStatus: "unknown",
	}

	// Aggregate from all modules
	if result.Terraform != nil {
		summary.TotalResources += result.Terraform.Summary.TotalResources
		if result.Terraform.Summary.Status == "healthy" {
			summary.HealthyResources += result.Terraform.Summary.TotalResources
		} else if result.Terraform.Summary.Status == "degraded" {
			summary.DegradedResources += result.Terraform.Summary.TotalResources
		}
	}

	if result.Kubernetes != nil {
		// Kubernetes resources are counted differently
		// For simplicity, use component count
		summary.TotalResources += result.Kubernetes.Summary.TotalComponents
		if result.Kubernetes.Summary.Status == "healthy" {
			summary.HealthyResources += result.Kubernetes.Summary.Healthy
		} else if result.Kubernetes.Summary.Status == "degraded" {
			summary.DegradedResources += result.Kubernetes.Summary.Degraded
		}
	}

	if result.GitOps != nil {
		summary.TotalResources += result.GitOps.Summary.TotalApps
		if result.GitOps.Summary.Status == "healthy" {
			summary.HealthyResources += result.GitOps.Summary.Synced
		} else if result.GitOps.Summary.Status == "degraded" {
			summary.DegradedResources += result.GitOps.Summary.OutOfSync
		}
	}

	// Determine overall status
	if summary.TotalResources == 0 {
		summary.OverallStatus = "unknown"
	} else if summary.DegradedResources == 0 {
		summary.OverallStatus = "healthy"
	} else if summary.HealthyResources > 0 {
		summary.OverallStatus = "degraded"
	} else {
		summary.OverallStatus = "degraded"
	}

	return summary
}

// outputStatus outputs status result in the specified format
func outputStatus(result *state.StatusResult, format string) error {
	switch format {
	case "json":
		return outputStatusJSON(result)
	case "yaml":
		return outputStatusYAML(result)
	default: // "table"
		return outputStatusTable(result)
	}
}

// outputStatusJSON outputs status in JSON format
func outputStatusJSON(result *state.StatusResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// outputStatusYAML outputs status in YAML format
func outputStatusYAML(result *state.StatusResult) error {
	data, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal status to YAML: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// outputStatusTable outputs status in table format
func outputStatusTable(result *state.StatusResult) error {
	fmt.Println("\n📊 Infrastructure Status")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Overall summary
	fmt.Printf("Overall Status: %s\n", getStatusIcon(result.Summary.OverallStatus))
	fmt.Printf("Total Resources: %d | Healthy: %d | Degraded: %d\n\n",
		result.Summary.TotalResources,
		result.Summary.HealthyResources,
		result.Summary.DegradedResources)

	// Terraform status
	if result.Terraform != nil {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📦 Terraform Status")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Status: %s | Total Dirs: %d | Initialized: %d | Resources: %d\n\n",
			getStatusIcon(result.Terraform.Summary.Status),
			result.Terraform.Summary.TotalDirs,
			result.Terraform.Summary.Initialized,
			result.Terraform.Summary.TotalResources)

		// Init directories
		if len(result.Terraform.InitDirs) > 0 {
			fmt.Println("Init Directories:")
			for _, dir := range result.Terraform.InitDirs {
				fmt.Printf("  %s %s: %d resources", getStatusIcon(dir.Status), dir.Name, dir.Resources)
				if !dir.LastUpdated.IsZero() {
					fmt.Printf(" (updated: %s)", dir.LastUpdated.Format("2006-01-02 15:04:05"))
				}
				if dir.ErrorMessage != "" {
					fmt.Printf(" - Error: %s", dir.ErrorMessage)
				}
				fmt.Println()
			}
			fmt.Println()
		}

		// Projects
		if len(result.Terraform.Projects) > 0 {
			fmt.Println("Projects:")
			for _, project := range result.Terraform.Projects {
				fmt.Printf("  %s %s: %d resources", getStatusIcon(project.Status), project.Name, project.Resources)
				if !project.LastUpdated.IsZero() {
					fmt.Printf(" (updated: %s)", project.LastUpdated.Format("2006-01-02 15:04:05"))
				}
				if project.ErrorMessage != "" {
					fmt.Printf(" - Error: %s", project.ErrorMessage)
				}
				fmt.Println()
			}
			fmt.Println()
		}
	}

	// Kubernetes status
	if result.Kubernetes != nil {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("☸️  Kubernetes Status")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Status: %s | Projects: %d | Components: %d | Healthy: %d | Degraded: %d\n\n",
			getStatusIcon(result.Kubernetes.Summary.Status),
			result.Kubernetes.Summary.TotalProjects,
			result.Kubernetes.Summary.TotalComponents,
			result.Kubernetes.Summary.Healthy,
			result.Kubernetes.Summary.Degraded)

		for _, project := range result.Kubernetes.Projects {
			fmt.Printf("Project: %s\n", project.Name)
			fmt.Printf("  Components: %d | Healthy: %d | Degraded: %d\n",
				project.Summary.TotalComponents,
				project.Summary.Healthy,
				project.Summary.Degraded)

			for _, comp := range project.Components {
				fmt.Printf("    %s %s (ns: %s): ", getStatusIcon(comp.Status), comp.Name, comp.Namespace)
				if comp.Deployments.Total > 0 {
					fmt.Printf("Deployments: %d/%d ready | ", comp.Deployments.Ready, comp.Deployments.Total)
				}
				if comp.StatefulSets.Total > 0 {
					fmt.Printf("StatefulSets: %d/%d ready | ", comp.StatefulSets.Ready, comp.StatefulSets.Total)
				}
				if comp.Services.Total > 0 {
					fmt.Printf("Services: %d", comp.Services.Total)
				}
				if comp.ErrorMessage != "" {
					fmt.Printf(" - Error: %s", comp.ErrorMessage)
				}
				fmt.Println()
			}
			fmt.Println()
		}
	}

	// GitOps status
	if result.GitOps != nil {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("🔄 GitOps Status")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Status: %s | Projects: %d | Applications: %d | Synced: %d | OutOfSync: %d\n\n",
			getStatusIcon(result.GitOps.Summary.Status),
			result.GitOps.Summary.TotalProjects,
			result.GitOps.Summary.TotalApps,
			result.GitOps.Summary.Synced,
			result.GitOps.Summary.OutOfSync)

		for _, project := range result.GitOps.Projects {
			fmt.Printf("Project: %s\n", project.Name)
			fmt.Printf("  Applications: %d | Synced: %d | OutOfSync: %d\n",
				project.Summary.TotalApps,
				project.Summary.Synced,
				project.Summary.OutOfSync)

			for _, app := range project.Applications {
				fmt.Printf("    %s %s (ns: %s): Sync=%s Health=%s",
					getStatusIcon(getAppStatus(app.SyncStatus, app.HealthStatus)),
					app.Name,
					app.Namespace,
					app.SyncStatus,
					app.HealthStatus)
				if app.Revision != "" {
					fmt.Printf(" | Revision: %s", app.Revision)
				}
				if !app.LastSynced.IsZero() {
					fmt.Printf(" | Last Synced: %s", app.LastSynced.Format("2006-01-02 15:04:05"))
				}
				if app.ErrorMessage != "" {
					fmt.Printf(" - Error: %s", app.ErrorMessage)
				}
				fmt.Println()
			}
			fmt.Println()
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Generated at: %s\n", result.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	return nil
}

// getStatusIcon returns an icon for status
func getStatusIcon(status string) string {
	switch status {
	case "healthy", "initialized", "Synced", "Healthy":
		return "✅"
	case "degraded", "OutOfSync", "Degraded":
		return "⚠️"
	case "error", "not_found", "not_initialized", "Unknown":
		return "❌"
	default:
		return "❓"
	}
}

// getAppStatus determines overall app status from sync and health
func getAppStatus(syncStatus, healthStatus string) string {
	if syncStatus == "Synced" && healthStatus == "Healthy" {
		return "healthy"
	} else if syncStatus == "OutOfSync" || healthStatus == "Degraded" {
		return "degraded"
	}
	return "unknown"
}
