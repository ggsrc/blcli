package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"blcli/pkg/bootstrap"
)

// NewApplyGitOpsCommand creates the apply gitops subcommand
func NewApplyGitOpsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gitops",
		Short: "Deploy GitOps configurations via ArgoCD",
		Long: `Deploy GitOps configurations via ArgoCD.

This command:
1. Creates or verifies GitHub repository (if --create-repo is specified)
2. Pushes GitOps configuration to the repository
3. Creates or updates ArgoCD Application
4. Waits for ArgoCD sync to complete (unless --skip-sync is specified)

Examples:
  # Apply gitops with repo creation
  blcli apply gitops -d ./generated/gitops --args args.yaml --create-repo

  # Apply gitops with existing repo
  blcli apply gitops -d ./generated/gitops --args args.yaml

  # Apply gitops without waiting for sync
  blcli apply gitops -d ./generated/gitops --args args.yaml --skip-sync`,
		Example: `  # Apply gitops with repo creation
  blcli apply gitops -d ./generated/gitops --args args.yaml --create-repo

  # Apply gitops with existing repo
  blcli apply gitops -d ./generated/gitops --args args.yaml

  # Apply gitops without waiting for sync
  blcli apply gitops -d ./generated/gitops --args args.yaml --skip-sync`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(applyGitOpsArgsPaths) == 0 {
				return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
			}

			// Parse timeout
			timeout := 30 * time.Minute
			if applyGitOpsTimeout != "" {
				var err error
				timeout, err = time.ParseDuration(applyGitOpsTimeout)
				if err != nil {
					return fmt.Errorf("invalid timeout duration: %w", err)
				}
			}

			// Don't print full usage on runtime errors (e.g. git push / ArgoCD apply failure)
			cmd.SilenceUsage = true

			return bootstrap.ExecuteApplyGitOps(bootstrap.ApplyGitOpsOptions{
				GitOpsDir:    applyGitOpsDir,
				Project:      applyGitOpsProject,
				ArgsPaths:    applyGitOpsArgsPaths,
				Kubeconfig:   applyGitOpsKubeconfig,
				Context:      applyGitOpsContext,
				CreateRepo:   applyGitOpsCreateRepo,
				RepoURL:      applyGitOpsRepoURL,
				Branch:       applyGitOpsBranch,
				ArgoCDServer: applyGitOpsArgoCDServer,
				ArgoCDToken:  applyGitOpsArgoCDToken,
				Timeout:      timeout,
				SkipSync:     applyGitOpsSkipSync,
				DryRun:       applyGitOpsDryRun,
			})
		},
	}

	cmd.Flags().StringVarP(&applyGitOpsDir, "dir", "d", "",
		"GitOps directory path (optional, defaults to workspace/gitops from args)")
	cmd.Flags().StringVarP(&applyGitOpsProject, "project", "p", "",
		"Only apply ArgoCD Applications under this project (e.g. stg). If not set, applies all projects.")
	cmd.Flags().StringArrayVar(&applyGitOpsArgsPaths, "args", nil,
		"Path to YAML or TOML file with blcli configuration (required, can be specified multiple times)")
	cmd.Flags().StringVar(&applyGitOpsKubeconfig, "kubeconfig", "",
		"Path to kubeconfig file for kubectl (default: KUBECONFIG env)")
	cmd.Flags().StringVar(&applyGitOpsContext, "context", "",
		"Kubernetes context name for kubectl")
	cmd.Flags().BoolVar(&applyGitOpsCreateRepo, "create-repo", false,
		"Create GitHub repository if it doesn't exist")
	cmd.Flags().StringVar(&applyGitOpsRepoURL, "repo-url", "",
		"GitHub repository URL (overrides value from args)")
	cmd.Flags().StringVar(&applyGitOpsBranch, "branch", "main",
		"Git branch name (default: main)")
	cmd.Flags().StringVar(&applyGitOpsArgoCDServer, "argocd-server", "",
		"ArgoCD server URL (overrides value from args)")
	cmd.Flags().StringVar(&applyGitOpsArgoCDToken, "argocd-token", "",
		"ArgoCD API token (overrides value from args or environment variable)")
	cmd.Flags().StringVar(&applyGitOpsTimeout, "timeout", "30m",
		"Apply timeout duration (e.g., 30m, 1h). Default: 30m")
	cmd.Flags().BoolVar(&applyGitOpsSkipSync, "skip-sync", false,
		"Create Application without waiting for sync to complete")
	cmd.Flags().BoolVar(&applyGitOpsDryRun, "dry-run", false,
		"Only show execution plan without applying ArgoCD Applications")

	return cmd
}
