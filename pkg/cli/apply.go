package cli

import (
	"github.com/spf13/cobra"
)

var (
	// Apply terraform flags
	applyTerraformDir             string
	applyTerraformProject         string
	applyTerraformTimeout         string
	applyTerraformInitDelay       string // Delay after each init dir (apply init only), e.g. "30s"
	applyTerraformUseEmulator     bool
	applyTerraformEmulatorPort    int
	applyTerraformEmulatorDataDir string
	applyTerraformAutoApprove     bool
	applyTerraformSkipBackend     bool
	applyTerraformTemplateRepo    string
	applyTerraformDryRun          bool

	// Apply kubernetes flags
	applyKubernetesDir            string
	applyKubernetesProject        string
	applyKubernetesKubeconfig     string
	applyKubernetesContext        string
	applyKubernetesNamespace      string
	applyKubernetesTimeout        string
	applyKubernetesDryRun         bool
	applyKubernetesWait           bool
	applyKubernetesComponentWait  string // Wait after each component (e.g. "30s"). "0" = no wait.

	// Apply gitops flags
	applyGitOpsDir          string
	applyGitOpsProject      string
	applyGitOpsArgsPaths    []string
	applyGitOpsKubeconfig   string
	applyGitOpsContext      string
	applyGitOpsCreateRepo   bool
	applyGitOpsRepoURL      string
	applyGitOpsBranch       string
	applyGitOpsArgoCDServer string
	applyGitOpsArgoCDToken  string
	applyGitOpsTimeout      string
	applyGitOpsSkipSync     bool
	applyGitOpsDryRun       bool

	// Apply all flags
	applyAllDir             string
	applyAllTerraformDir    string
	applyAllKubernetesDir   string
	applyAllGitOpsDir       string
	applyAllArgsPaths       []string
	applyAllContinueOnError bool
	applyAllSkipModules     []string
)

// NewApplyCommand creates the apply command with subcommands
func NewApplyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply infrastructure configurations",
		Long: `Apply command applies infrastructure configurations to the target environment.

This command supports subcommands:
  - init: Apply only Terraform init directories (step 0: prepare then init, e.g. remote state)
  - terraform: Apply Terraform configurations to GCP
  - kubernetes: Deploy Kubernetes resources to cluster
  - gitops: Deploy GitOps configurations via ArgoCD
  - all: Apply all modules in order (terraform -> kubernetes -> gitops)
  - init-repos: 为 terraform/kubernetes/gitops 目录执行 git init、创建 GitHub 仓库并提交推送（需 Y 确认）`,
	}

	// Add subcommands
	cmd.AddCommand(NewApplyInitCommand())
	cmd.AddCommand(NewApplyTerraformCommand())
	cmd.AddCommand(NewApplyKubernetesCommand())
	cmd.AddCommand(NewApplyGitOpsCommand())
	cmd.AddCommand(NewApplyAllCommand())
	cmd.AddCommand(NewApplyInitReposCommand())

	return cmd
}
