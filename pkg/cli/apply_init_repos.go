package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"blcli/pkg/bootstrap"
)

var (
	initReposDir           string
	initReposOrg           string
	initReposTerraformDir  string
	initReposKubernetesDir string
	initReposGitOpsDir     string
)

// NewApplyInitReposCommand creates the apply init-repos subcommand.
func NewApplyInitReposCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-repos",
		Short: "Create GitHub repositories for the terraform, kubernetes and gitops directories and push them",
		Long: `For each generated directory (terraform, kubernetes, gitops):
1. Run git init in it, unless it is already a git repository.
2. After you confirm, create its GitHub repository under the given org with gh.
3. After you confirm, run git add, commit and push.

Type Y at each prompt to continue; N or anything else stops.

Requires gh, installed and logged in: gh auth login

Examples:
  blcli apply init-repos --org github.com/someone -d ./workspace/output
  blcli apply init-repos -o someone -d ./generated`,
		Example: `  blcli apply init-repos --org github.com/myorg -d ./workspace/output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if initReposDir == "" {
				return fmt.Errorf("workspace directory is required: pass -d with the directory that contains terraform, kubernetes and gitops")
			}
			if initReposOrg == "" {
				return fmt.Errorf("GitHub org is required: pass -o/--org, e.g. github.com/someone or someone")
			}

			tfDir := initReposTerraformDir
			if tfDir == "" {
				tfDir = fmt.Sprintf("%s/terraform", initReposDir)
			}
			k8sDir := initReposKubernetesDir
			if k8sDir == "" {
				k8sDir = fmt.Sprintf("%s/kubernetes", initReposDir)
			}
			gitopsDir := initReposGitOpsDir
			if gitopsDir == "" {
				gitopsDir = fmt.Sprintf("%s/gitops", initReposDir)
			}

			return bootstrap.ExecuteInitRepos(bootstrap.InitReposOptions{
				WorkspaceDir:    initReposDir,
				TerraformDir:    tfDir,
				KubernetesDir:   k8sDir,
				GitOpsDir:       gitopsDir,
				GitHubOrg:       initReposOrg,
			})
		},
	}

	cmd.Flags().StringVarP(&initReposDir, "dir", "d", "",
		"workspace root that contains the terraform, kubernetes and gitops directories (required)")
	cmd.Flags().StringVarP(&initReposOrg, "org", "o", "",
		"GitHub organization or user, e.g. github.com/someone or someone (required)")
	cmd.Flags().StringVar(&initReposTerraformDir, "terraform-dir", "",
		"terraform directory (default {workspace}/terraform)")
	cmd.Flags().StringVar(&initReposKubernetesDir, "kubernetes-dir", "",
		"kubernetes directory (default {workspace}/kubernetes)")
	cmd.Flags().StringVar(&initReposGitOpsDir, "gitops-dir", "",
		"gitops directory (default {workspace}/gitops)")

	return cmd
}
