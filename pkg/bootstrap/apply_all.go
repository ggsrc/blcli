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

	// Initialize progress tracker
	operationID := GenerateOperationID("apply")
	progressTracker, err := NewProgressTracker(operationID, "apply", false)
	if err != nil {
		fmt.Printf("Warning: failed to initialize progress tracker: %v\n", err)
		progressTracker = nil
	}

	if progressTracker != nil {
		if err := progressTracker.StartOperation(); err != nil {
			fmt.Printf("Warning: failed to start progress tracking: %v\n", err)
		}
		defer func() {
			if progressTracker != nil {
				progressTracker.PrintSummary()
			}
		}()
	}

	var errors []string

	// Check which modules to skip
	skipTerraform := contains(opts.SkipModules, "terraform")
	skipKubernetes := contains(opts.SkipModules, "kubernetes")
	skipGitOps := contains(opts.SkipModules, "gitops")

	// Step 1: Apply terraform
	if !skipTerraform {
		if progressTracker != nil {
			progressTracker.StartModule("terraform", 1)
		}
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📦 Step 1: Applying Terraform")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		terraformOpts := ApplyTerraformOptions{
			TerraformDir:    opts.TerraformDir,
			Timeout:         1 * time.Hour, // Default timeout
			UseEmulator:     opts.TerraformUseEmulator,
			EmulatorPort:    opts.TerraformEmulatorPort,
			EmulatorDataDir: opts.TerraformEmulatorDataDir,
			AutoApprove:     opts.TerraformAutoApprove,
			SkipBackend:     opts.TerraformSkipBackend,
			DryRun:          opts.TerraformDryRun,
		}

		if err := ExecuteApplyTerraform(terraformOpts); err != nil {
			errors = append(errors, fmt.Sprintf("terraform: %v", err))
			if progressTracker != nil {
				progressTracker.FailModule("terraform", fmt.Sprintf("%v", err))
			}
			if !opts.ContinueOnError {
				if progressTracker != nil {
					progressTracker.FailOperation(fmt.Sprintf("terraform apply failed: %v", err))
				}
				return fmt.Errorf("terraform apply failed: %w", err)
			}
			fmt.Printf("⚠️  Terraform apply failed, but continuing due to --continue-on-error\n")
		} else {
			if progressTracker != nil {
				progressTracker.CompleteModule("terraform")
			}
			fmt.Println("✅ Terraform apply completed successfully")
			fmt.Println()
		}
	} else {
		if progressTracker != nil {
			progressTracker.SkipModule("terraform")
		}
		fmt.Println("⏭️  Skipping terraform (--skip-modules=terraform)")
	}

	// Step 2: Apply kubernetes
	if !skipKubernetes {
		if progressTracker != nil {
			progressTracker.StartModule("kubernetes", 1)
		}
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("☸️  Step 2: Applying Kubernetes")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		kubernetesOpts := ApplyKubernetesOptions{
			KubernetesDir:           opts.KubernetesDir,
			Kubeconfig:              opts.KubernetesKubeconfig,
			Context:                 opts.KubernetesContext,
			Namespace:               opts.KubernetesNamespace,
			Timeout:                 30 * time.Minute, // Default timeout
			DryRun:                  opts.KubernetesDryRun,
			Wait:                    opts.KubernetesWait,
			ComponentWaitAfterApply: 30 * time.Second, // Wait after each component before next
		}

		// For apply all, template repo is not available, so pass nil
		// This means installType will be inferred from file structure
		if err := ExecuteApplyKubernetes(kubernetesOpts, nil); err != nil {
			errors = append(errors, fmt.Sprintf("kubernetes: %v", err))
			if progressTracker != nil {
				progressTracker.FailModule("kubernetes", fmt.Sprintf("%v", err))
			}
			if !opts.ContinueOnError {
				if progressTracker != nil {
					progressTracker.FailOperation(fmt.Sprintf("kubernetes apply failed: %v", err))
				}
				return fmt.Errorf("kubernetes apply failed: %w", err)
			}
			fmt.Printf("⚠️  Kubernetes apply failed, but continuing due to --continue-on-error\n")
		} else {
			if progressTracker != nil {
				progressTracker.CompleteModule("kubernetes")
			}
			fmt.Println("✅ Kubernetes apply completed successfully")
			fmt.Println()
		}
	} else {
		if progressTracker != nil {
			progressTracker.SkipModule("kubernetes")
		}
		fmt.Println("⏭️  Skipping kubernetes (--skip-modules=kubernetes)")
	}

	// Step 3: Apply gitops
	if !skipGitOps {
		if progressTracker != nil {
			progressTracker.StartModule("gitops", 1)
		}
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("🔄 Step 3: Applying GitOps")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		gitopsOpts := ApplyGitOpsOptions{
			GitOpsDir:    opts.GitOpsDir,
			ArgsPaths:    opts.ArgsPaths,
			Kubeconfig:   opts.KubernetesKubeconfig,
			Context:      opts.KubernetesContext,
			CreateRepo:   opts.GitOpsCreateRepo,
			RepoURL:      opts.GitOpsRepoURL,
			Branch:       opts.GitOpsBranch,
			ArgoCDServer: opts.GitOpsArgoCDServer,
			ArgoCDToken:  opts.GitOpsArgoCDToken,
			Timeout:      30 * time.Minute, // Default timeout
			SkipSync:     opts.GitOpsSkipSync,
			DryRun:       opts.GitOpsDryRun,
		}

		if err := ExecuteApplyGitOps(gitopsOpts); err != nil {
			errors = append(errors, fmt.Sprintf("gitops: %v", err))
			if progressTracker != nil {
				progressTracker.FailModule("gitops", fmt.Sprintf("%v", err))
			}
			if !opts.ContinueOnError {
				if progressTracker != nil {
					progressTracker.FailOperation(fmt.Sprintf("gitops apply failed: %v", err))
				}
				return fmt.Errorf("gitops apply failed: %w", err)
			}
			fmt.Printf("⚠️  GitOps apply failed, but continuing due to --continue-on-error\n")
		} else {
			if progressTracker != nil {
				progressTracker.CompleteModule("gitops")
			}
			fmt.Println("✅ GitOps apply completed successfully")
			fmt.Println()
		}
	} else {
		if progressTracker != nil {
			progressTracker.SkipModule("gitops")
		}
		fmt.Println("⏭️  Skipping gitops (--skip-modules=gitops)")
	}

	// Summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 Apply All Summary")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if len(errors) > 0 {
		if progressTracker != nil {
			progressTracker.FailOperation(fmt.Sprintf("some modules failed: %v", errors))
		}
		fmt.Printf("❌ Some modules failed:\n")
		for _, err := range errors {
			fmt.Printf("   - %s\n", err)
		}
		return fmt.Errorf("some modules failed")
	}

	if progressTracker != nil {
		progressTracker.CompleteOperation()
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
