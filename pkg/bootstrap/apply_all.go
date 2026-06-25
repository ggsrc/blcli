package bootstrap

import (
	"fmt"
	"strings"
	"time"
)

// ExecuteApplyAll executes the apply all command
func ExecuteApplyAll(opts ApplyAllOptions) error {
	fmt.Println("\n🚀 Applying all modules in order: terraform -> kubernetes -> gitops")
	fmt.Println()

	progressTracker, err := ResolveProgressTracker(progressResumeOptions{
		OperationType: "apply",
		Quiet:         false,
		NoResume:      opts.NoResume,
	})
	if err != nil {
		fmt.Printf("Warning: failed to initialize progress tracker: %v\n", err)
		progressTracker = nil
	}
	if progressTracker != nil {
		defer progressTracker.PrintSummary()
	}

	var errors []string

	modules := []struct {
		name      string
		skipped   bool
		skipLabel string
		title     string
		run       func() error
	}{
		{
			name:      "terraform",
			skipped:   contains(opts.SkipModules, "terraform"),
			skipLabel: "⏭️  Skipping terraform (--skip-modules=terraform)",
			title:     "📦 Step 1: Applying Terraform",
			run: func() error {
				return ExecuteApplyTerraform(ApplyTerraformOptions{
					TerraformDir:    opts.TerraformDir,
					Timeout:         1 * time.Hour,
					UseEmulator:     opts.TerraformUseEmulator,
					EmulatorPort:    opts.TerraformEmulatorPort,
					EmulatorDataDir: opts.TerraformEmulatorDataDir,
					AutoApprove:     opts.TerraformAutoApprove,
					SkipBackend:     opts.TerraformSkipBackend,
					DryRun:          opts.TerraformDryRun,
				})
			},
		},
		{
			name:      "kubernetes",
			skipped:   contains(opts.SkipModules, "kubernetes"),
			skipLabel: "⏭️  Skipping kubernetes (--skip-modules=kubernetes)",
			title:     "☸️  Step 2: Applying Kubernetes",
			run: func() error {
				return ExecuteApplyKubernetes(ApplyKubernetesOptions{
					KubernetesDir:           opts.KubernetesDir,
					Kubeconfig:              opts.KubernetesKubeconfig,
					Context:                 opts.KubernetesContext,
					Namespace:               opts.KubernetesNamespace,
					Timeout:                 30 * time.Minute,
					DryRun:                  opts.KubernetesDryRun,
					Wait:                    opts.KubernetesWait,
					ComponentWaitAfterApply: 30 * time.Second,
				}, nil)
			},
		},
		{
			name:      "gitops",
			skipped:   contains(opts.SkipModules, "gitops"),
			skipLabel: "⏭️  Skipping gitops (--skip-modules=gitops)",
			title:     "🔄 Step 3: Applying GitOps",
			run: func() error {
				return ExecuteApplyGitOps(ApplyGitOpsOptions{
					GitOpsDir:    opts.GitOpsDir,
					ArgsPaths:    opts.ArgsPaths,
					Kubeconfig:   opts.KubernetesKubeconfig,
					Context:      opts.KubernetesContext,
					CreateRepo:   opts.GitOpsCreateRepo,
					RepoURL:      opts.GitOpsRepoURL,
					Branch:       opts.GitOpsBranch,
					ArgoCDServer: opts.GitOpsArgoCDServer,
					ArgoCDToken:  opts.GitOpsArgoCDToken,
					Timeout:      30 * time.Minute,
					SkipSync:     opts.GitOpsSkipSync,
					DryRun:       opts.GitOpsDryRun,
				})
			},
		},
	}

	for _, module := range modules {
		if module.skipped {
			if progressTracker != nil {
				_ = progressTracker.SkipModule(module.name)
			}
			fmt.Println(module.skipLabel)
			continue
		}

		if ModuleAlreadyCompleted(progressTracker, module.name) {
			fmt.Printf("⏭️  Skipping %s (already completed in resumed operation)\n", module.name)
			continue
		}

		if progressTracker != nil {
			if err := progressTracker.StartModule(module.name, 1); err != nil {
				fmt.Printf("Warning: failed to track module %s: %v\n", module.name, err)
			}
		}

		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println(module.title)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		if err := module.run(); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", module.name, err))
			if progressTracker != nil {
				_ = progressTracker.FailModule(module.name, fmt.Sprintf("%v", err))
			}
			if !opts.ContinueOnError {
				if progressTracker != nil {
					_ = progressTracker.FailOperation(fmt.Sprintf("%s apply failed: %v", module.name, err))
				}
				return WrapApplyError("apply "+module.name, fmt.Errorf("%s apply failed: %w", module.name, err))
			}
			fmt.Printf("⚠️  %s apply failed, but continuing due to --continue-on-error\n", module.name)
			continue
		}

		if progressTracker != nil {
			_ = progressTracker.CompleteModule(module.name)
		}
		fmt.Printf("✅ %s apply completed successfully\n\n", module.name)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 Apply All Summary")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if len(errors) > 0 {
		if progressTracker != nil {
			_ = progressTracker.FailOperation(fmt.Sprintf("some modules failed: %v", errors))
		}
		fmt.Println("❌ Some modules failed:")
		for _, errMsg := range errors {
			fmt.Printf("   - %s\n", errMsg)
		}
		return WrapApplyError("apply all", fmt.Errorf("some modules failed"))
	}

	if progressTracker != nil {
		_ = progressTracker.CompleteOperation()
	}
	fmt.Println("✅ All modules applied successfully!")
	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}
