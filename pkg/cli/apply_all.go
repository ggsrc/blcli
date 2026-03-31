package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"blcli/pkg/bootstrap"
)

// NewApplyAllCommand creates the apply all subcommand
func NewApplyAllCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Apply all modules in order",
		Long: `Apply all modules in order: terraform -> kubernetes -> gitops.

This command executes apply for all modules in the correct dependency order:
1. terraform: Apply Terraform configurations (must succeed)
2. kubernetes: Deploy Kubernetes resources (depends on terraform)
3. gitops: Deploy GitOps configurations (depends on kubernetes)

Examples:
  # Apply all modules
  blcli apply all -d ./generated --args args.yaml

  # Apply all with continue on error
  blcli apply all -d ./generated --args args.yaml --continue-on-error

  # Apply all skipping gitops
  blcli apply all -d ./generated --args args.yaml --skip-modules gitops`,
		Example: `  # Apply all modules
  blcli apply all -d ./generated --args args.yaml

  # Apply all with continue on error
  blcli apply all -d ./generated --args args.yaml --continue-on-error

  # Apply all skipping gitops
  blcli apply all -d ./generated --args args.yaml --skip-modules gitops`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if applyAllDir == "" {
				return fmt.Errorf("workspace directory is required. Use -d to specify the directory")
			}
			if len(applyAllArgsPaths) == 0 {
				return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
			}

			// Set default directories if not specified
			terraformDir := applyAllTerraformDir
			if terraformDir == "" {
				terraformDir = fmt.Sprintf("%s/terraform", applyAllDir)
			}
			kubernetesDir := applyAllKubernetesDir
			if kubernetesDir == "" {
				kubernetesDir = fmt.Sprintf("%s/kubernetes", applyAllDir)
			}
			gitopsDir := applyAllGitOpsDir
			if gitopsDir == "" {
				gitopsDir = fmt.Sprintf("%s/gitops", applyAllDir)
			}

			// Don't print full usage on runtime errors (e.g. a module apply failure)
			cmd.SilenceUsage = true

			return bootstrap.ExecuteApplyAll(bootstrap.ApplyAllOptions{
				WorkspaceDir:    applyAllDir,
				TerraformDir:    terraformDir,
				KubernetesDir:   kubernetesDir,
				GitOpsDir:       gitopsDir,
				ArgsPaths:       applyAllArgsPaths,
				ContinueOnError: applyAllContinueOnError,
				SkipModules:     applyAllSkipModules,
				// Pass through terraform options
				TerraformUseEmulator:     applyTerraformUseEmulator,
				TerraformEmulatorPort:    applyTerraformEmulatorPort,
				TerraformEmulatorDataDir: applyTerraformEmulatorDataDir,
				TerraformAutoApprove:     applyTerraformAutoApprove,
				TerraformSkipBackend:     applyTerraformSkipBackend,
				TerraformDryRun:          applyTerraformDryRun,
				// Pass through kubernetes options
				KubernetesKubeconfig: applyKubernetesKubeconfig,
				KubernetesContext:    applyKubernetesContext,
				KubernetesNamespace:  applyKubernetesNamespace,
				KubernetesDryRun:     applyKubernetesDryRun,
				KubernetesWait:       applyKubernetesWait,
				// Pass through gitops options
				GitOpsCreateRepo:   applyGitOpsCreateRepo,
				GitOpsRepoURL:      applyGitOpsRepoURL,
				GitOpsBranch:       applyGitOpsBranch,
				GitOpsArgoCDServer: applyGitOpsArgoCDServer,
				GitOpsArgoCDToken:  applyGitOpsArgoCDToken,
				GitOpsSkipSync:     applyGitOpsSkipSync,
				GitOpsDryRun:       applyGitOpsDryRun,
			})
		},
	}

	cmd.Flags().StringVarP(&applyAllDir, "dir", "d", "",
		"Workspace root directory path (required, contains terraform, kubernetes, gitops subdirectories)")
	cmd.Flags().StringVar(&applyAllTerraformDir, "terraform-dir", "",
		"Terraform directory path (default: {workspace}/terraform)")
	cmd.Flags().StringVar(&applyAllKubernetesDir, "kubernetes-dir", "",
		"Kubernetes directory path (default: {workspace}/kubernetes)")
	cmd.Flags().StringVar(&applyAllGitOpsDir, "gitops-dir", "",
		"GitOps directory path (default: {workspace}/gitops)")
	cmd.Flags().StringArrayVar(&applyAllArgsPaths, "args", nil,
		"Path to YAML or TOML file with blcli configuration (required, can be specified multiple times)")
	cmd.Flags().BoolVar(&applyAllContinueOnError, "continue-on-error", false,
		"Continue executing remaining modules when one fails")
	cmd.Flags().StringArrayVar(&applyAllSkipModules, "skip-modules", nil,
		"Skip specified modules (e.g., --skip-modules=gitops)")

	// Inherit flags from subcommands
	cmd.Flags().BoolVar(&applyTerraformUseEmulator, "use-emulator", false,
		"Use GCS emulator for terraform (for testing)")
	cmd.Flags().IntVar(&applyTerraformEmulatorPort, "emulator-port", 4443,
		"GCS emulator port")
	cmd.Flags().StringVar(&applyTerraformEmulatorDataDir, "emulator-data-dir", "",
		"GCS emulator data directory")
	cmd.Flags().BoolVar(&applyTerraformAutoApprove, "auto-approve", false,
		"Automatically approve terraform apply")
	cmd.Flags().BoolVar(&applyTerraformSkipBackend, "skip-backend", false,
		"Skip terraform backend initialization")
	cmd.Flags().StringVar(&applyKubernetesKubeconfig, "kubeconfig", "",
		"kubeconfig file path")
	cmd.Flags().StringVar(&applyKubernetesContext, "context", "",
		"Kubernetes context name")
	cmd.Flags().StringVar(&applyKubernetesNamespace, "namespace", "",
		"Kubernetes namespace")
	cmd.Flags().BoolVar(&applyKubernetesDryRun, "dry-run", false,
		"Kubernetes dry-run mode")
	cmd.Flags().BoolVar(&applyKubernetesWait, "wait", true,
		"Wait for kubernetes resources to be ready")
	cmd.Flags().BoolVar(&applyGitOpsCreateRepo, "create-repo", false,
		"Create GitHub repository if it doesn't exist")
	cmd.Flags().StringVar(&applyGitOpsRepoURL, "repo-url", "",
		"GitHub repository URL")
	cmd.Flags().StringVar(&applyGitOpsBranch, "branch", "main",
		"Git branch name")
	cmd.Flags().StringVar(&applyGitOpsArgoCDServer, "argocd-server", "",
		"ArgoCD server URL")
	cmd.Flags().StringVar(&applyGitOpsArgoCDToken, "argocd-token", "",
		"ArgoCD API token")
	cmd.Flags().BoolVar(&applyGitOpsSkipSync, "skip-sync", false,
		"Skip ArgoCD sync wait")
	cmd.Flags().BoolVar(&applyTerraformDryRun, "terraform-dry-run", false,
		"Terraform dry-run: only show execution plan")
	cmd.Flags().BoolVar(&applyGitOpsDryRun, "gitops-dry-run", false,
		"GitOps dry-run: only show execution plan")

	return cmd
}
