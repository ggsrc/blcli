package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"blcli/pkg/bootstrap"
	"blcli/pkg/internal"
)

var (
	checkRepoArgsPaths []string
	checkRepoTimeout   time.Duration
	checkRepoProject   string

	// Kubernetes check flags
	checkKubernetesDir          string
	checkKubernetesTimeout      time.Duration
	checkKubernetesKubeconfig   string
	checkKubernetesContext      string
	checkKubernetesNamespace    string
	checkKubernetesTemplateRepo string
)

// NewCheckCommand creates the check command with subcommands
func NewCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check tools and repository compliance",
		Long: `Check command provides subcommands:
  - plugin:    Check if required external tools (terraform, kubectl) are installed
  - repo:      Check if generated terraform code is compliant using terratest
  - kubernetes: Check if generated kubernetes manifests are valid`,
	}

	// Add subcommands
	cmd.AddCommand(NewCheckPluginCommand())
	cmd.AddCommand(NewCheckRepoCommand())
	cmd.AddCommand(NewCheckKubernetesCommand())

	return cmd
}

// NewCheckPluginCommand creates the check plugin subcommand
func NewCheckPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Check required and suggested external tools with install hints",
		Long: `Check if required and suggested external tools are installed.

必须安装 (Required): terraform, kubectl
建议安装 (Suggested): argocd, gh, istioctl

Install hints are shown by platform (mac / linux) when a tool is missing.`,
		Run: func(cmd *cobra.Command, args []string) {
			internal.CheckTools()
		},
	}

	return cmd
}

// NewCheckRepoCommand creates the check repo subcommand
func NewCheckRepoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Check if generated terraform code is compliant",
		Long: `Check if generated terraform code is compliant using terratest.

This command validates that the generated Terraform code can:
- Initialize successfully (terraform init)
- Validate syntax (terraform validate)
- Generate execution plan (terraform plan)

The check runs in the following order:
1. First, executes init directories in numeric order (e.g., 0-codestore, 1-my-org-projects)
2. Then, executes gcp project directories

Examples:
  # Check all projects
  blcli check repo --args=args.yaml

  # Check a specific project
  blcli check repo --args=args.yaml --project=my-project

  # Check with custom timeout
  blcli check repo --args=args.yaml --timeout=1h`,
		Example: `  # Check all projects
  blcli check repo --args=args.yaml

  # Check specific project
  blcli check repo --args=args.yaml --project=my-project

  # Check with custom timeout
  blcli check repo --args=args.yaml --timeout=1h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(checkRepoArgsPaths) == 0 {
				return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
			}

			projectName := checkRepoProject
			if len(args) > 0 {
				projectName = args[0]
			}

			return bootstrap.ExecuteCheckRepo(bootstrap.CheckRepoOptions{
				ArgsPaths:   checkRepoArgsPaths,
				Timeout:     checkRepoTimeout,
				ProjectName: projectName,
			})
		},
	}

	cmd.Flags().StringArrayVar(&checkRepoArgsPaths, "args", nil,
		"Path to YAML or TOML file with blcli configuration (required, can be specified multiple times)")
	cmd.Flags().DurationVar(&checkRepoTimeout, "timeout", 30*time.Minute,
		"Check timeout duration (e.g., 30m, 1h). Default: 30m")
	cmd.Flags().StringVarP(&checkRepoProject, "project", "p", "",
		"Specific terraform project name to check (if not specified, checks all projects)")

	return cmd
}

// NewCheckKubernetesCommand creates the check kubernetes subcommand
func NewCheckKubernetesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubernetes",
		Short: "Check if generated kubernetes manifests are valid",
		Long: `Check if generated kubernetes manifests are valid.

This command validates Kubernetes manifests based on their installType:
- kubectl: Uses 'kubectl apply --dry-run=client' to validate manifests
- helm: Uses 'helm lint' or 'helm template' to validate helm charts
- custom: Uses the 'check' command specified in config.yaml (if provided)

The check runs in the following order:
1. Namespace (if exists)
2. Init components (in dependency order)
3. Optional components (in dependency order)

Examples:
  # Check all kubernetes resources
  blcli check kubernetes -d ./workspace/output/kubernetes

  # Check with specific context
  blcli check kubernetes -d ./workspace/output/kubernetes --context my-cluster

  # Check with template repo (to load installType from config.yaml)
  blcli check kubernetes -d ./workspace/output/kubernetes -r github.com/user/repo`,
		Example: `  # Check all kubernetes resources
  blcli check kubernetes -d ./workspace/output/kubernetes

  # Check with specific context
  blcli check kubernetes -d ./workspace/output/kubernetes --context my-cluster

  # Check with template repo
  blcli check kubernetes -d ./workspace/output/kubernetes -r github.com/user/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if checkKubernetesDir == "" {
				return fmt.Errorf("kubernetes directory is required. Use -d to specify the directory")
			}

			// Parse timeout
			timeout := 30 * time.Minute
			if checkKubernetesTimeout > 0 {
				timeout = checkKubernetesTimeout
			}

			return bootstrap.ExecuteCheckKubernetes(bootstrap.CheckKubernetesOptions{
				KubernetesDir: checkKubernetesDir,
				Kubeconfig:    checkKubernetesKubeconfig,
				Context:       checkKubernetesContext,
				Namespace:     checkKubernetesNamespace,
				Timeout:       timeout,
				TemplateRepo:  checkKubernetesTemplateRepo,
			})
		},
	}

	cmd.Flags().StringVarP(&checkKubernetesDir, "dir", "d", "",
		"Kubernetes directory path (required, generated by 'blcli init kubernetes')")
	cmd.Flags().StringVar(&checkKubernetesKubeconfig, "kubeconfig", "",
		"kubeconfig file path (default: ~/.kube/config)")
	cmd.Flags().StringVar(&checkKubernetesContext, "context", "",
		"Kubernetes context name")
	cmd.Flags().StringVar(&checkKubernetesNamespace, "namespace", "",
		"Target namespace (if not specified, uses namespace from manifests)")
	cmd.Flags().DurationVar(&checkKubernetesTimeout, "timeout", 30*time.Minute,
		"Check timeout duration (e.g., 30m, 1h). Default: 30m")
	cmd.Flags().StringVarP(&checkKubernetesTemplateRepo, "template-repo", "r", "",
		"Template repository URL or local path (optional, used to load installType from config.yaml)")

	return cmd
}
