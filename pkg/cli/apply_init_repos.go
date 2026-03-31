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
		Short: "为 terraform/kubernetes/gitops 目录执行 git init、创建 GitHub 仓库并提交推送",
		Long: `对生成的三个目录（terraform、kubernetes、gitops）分别：
1. 在该目录执行 git init（若尚未是 git 仓库）
2. 提示确认后，使用 gh 创建对应 GitHub 仓库（属于指定 org）
3. 提示确认后，执行 git add / commit / push

创建仓库与提交前需输入 Y 继续，输入 N 或其它字符则停止。

需要安装并登录 gh：gh auth login

Examples:
  blcli apply init-repos --org github.com/someone -d ./workspace/output
  blcli apply init-repos -o someone -d ./generated`,
		Example: `  blcli apply init-repos --org github.com/myorg -d ./workspace/output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if initReposDir == "" {
				return fmt.Errorf("工作目录必填，请使用 -d 指定（包含 terraform、kubernetes、gitops 的根目录）")
			}
			if initReposOrg == "" {
				return fmt.Errorf("GitHub org 必填，请使用 -o/--org 指定，如 github.com/someone 或 someone")
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
		"工作目录根路径（必填，其下包含 terraform、kubernetes、gitops 子目录）")
	cmd.Flags().StringVarP(&initReposOrg, "org", "o", "",
		"GitHub 组织或用户名，如 github.com/someone 或 someone（必填）")
	cmd.Flags().StringVar(&initReposTerraformDir, "terraform-dir", "",
		"terraform 目录路径（默认 {workspace}/terraform）")
	cmd.Flags().StringVar(&initReposKubernetesDir, "kubernetes-dir", "",
		"kubernetes 目录路径（默认 {workspace}/kubernetes）")
	cmd.Flags().StringVar(&initReposGitOpsDir, "gitops-dir", "",
		"gitops 目录路径（默认 {workspace}/gitops）")

	return cmd
}
