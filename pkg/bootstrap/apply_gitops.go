package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"blcli/pkg/config"
	"blcli/pkg/renderer"
)

// ExecuteApplyGitOps applies only ArgoCD Application resources under the gitops directory.
// Actual application deployment is performed by ArgoCD when it syncs from the Git repo.
// This command finds all app.yaml (ArgoCD Application CRs) under gitops/{project}/{app}/ and
// runs kubectl apply -f on each.
func ExecuteApplyGitOps(opts ApplyGitOpsOptions) error {
	var allArgs []renderer.ArgsData
	for _, argsPath := range opts.ArgsPaths {
		fmt.Printf("Loading args from: %s\n", argsPath)
		args, err := renderer.LoadArgs(argsPath)
		if err != nil {
			return fmt.Errorf("failed to load args file %s: %w", argsPath, err)
		}
		allArgs = append(allArgs, args)
	}

	if len(allArgs) == 0 {
		return fmt.Errorf("no valid args files loaded")
	}
	reversed := make([]renderer.ArgsData, len(allArgs))
	for i, args := range allArgs {
		reversed[len(allArgs)-1-i] = args
	}
	mergedArgs := renderer.MergeArgs(reversed...)

	cfg, err := config.LoadFromArgs(mergedArgs)
	if err != nil {
		return fmt.Errorf("failed to load config from args: %w", err)
	}

	if cfg.Gitops == nil {
		return fmt.Errorf("no gitops configuration found in args file")
	}

	gitopsDir := opts.GitOpsDir
	if gitopsDir == "" {
		workspace := config.WorkspacePath(cfg.Global)
		gitopsDir = filepath.Join(workspace, "gitops")
	}

	if _, err := os.Stat(gitopsDir); os.IsNotExist(err) {
		return fmt.Errorf("gitops directory not found: %s", gitopsDir)
	}

	fmt.Printf("\n📋 Applying ArgoCD Application resources only (deployment is done by ArgoCD)...\n")
	fmt.Printf("   GitOps directory: %s\n", gitopsDir)

	// Collect all app.yaml under gitops/{project}/{app}/app.yaml (ArgoCD Application CRs)
	var appFiles []string
	err = filepath.Walk(gitopsDir, func(path string, info os.FileInfo, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == "app.yaml" {
			appFiles = append(appFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk gitops dir: %w", err)
	}

	if len(appFiles) == 0 {
		fmt.Println("   ℹ️  No app.yaml (ArgoCD Application) files found under gitops directory")
		return nil
	}

	// Build application list for execution plan: projectName/appName from path .../gitops/projectName/appName/app.yaml
	var applications []applicationInfo
	for _, f := range appFiles {
		appDir := filepath.Dir(f)
		appName := filepath.Base(appDir)
		projectName := filepath.Base(filepath.Dir(appDir))
		if opts.Project != "" && projectName != opts.Project {
			continue
		}
		applications = append(applications, applicationInfo{
			projectName: projectName,
			appName:     appName,
			appDir:      appDir,
			appYaml:     f,
		})
	}

	if opts.Project != "" && len(applications) == 0 {
		return fmt.Errorf("project %q not found or has no app.yaml. List project directories under: %s", opts.Project, gitopsDir)
	}

	// Build and print execution plan
	var planItems []PlanItem
	for i, app := range applications {
		planItems = append(planItems, PlanItem{
			Step:         i + 1,
			Name:         fmt.Sprintf("%s/%s", app.projectName, app.appName),
			Directory:    app.appDir,
			Command:      "kubectl",
			Args:         []string{"apply", "-f", app.appYaml},
			Dependencies: []string{},
			Description:  fmt.Sprintf("Apply ArgoCD Application %s in project %s", app.appName, app.projectName),
		})
	}
	plan := ExecutionPlan{
		Module: "gitops",
		Items:  planItems,
		DryRun: opts.DryRun,
	}
	PrintExecutionPlan(plan)

	if opts.DryRun {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	var failed []string
	for _, app := range applications {
		fmt.Printf("   Applying ArgoCD Application: %s\n", app.appYaml)
		if err := runKubectlApply(ctx, opts, app.appYaml); err != nil {
			fmt.Printf("   ❌ Failed to apply %s: %v\n", app.appYaml, err)
			failed = append(failed, app.appYaml)
			continue
		}
		fmt.Printf("   ✓ Applied %s\n", app.appYaml)
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to apply %d ArgoCD Application(s): %v", len(failed), failed)
	}

	fmt.Println("   ✓ All ArgoCD Application resources applied. Actual deployment will be performed by ArgoCD when it syncs from Git.")

	if opts.CreateRepo {
		fmt.Println("   ℹ️  GitHub repository creation not yet implemented")
	}
	if !opts.SkipSync && (opts.ArgoCDServer != "" || opts.ArgoCDToken != "") {
		fmt.Println("   ℹ️  ArgoCD sync wait not yet implemented (use argocd CLI or UI to trigger sync)")
	}

	return nil
}

func runKubectlApply(ctx context.Context, opts ApplyGitOpsOptions, file string) error {
	args := []string{"apply", "-f", file}
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
