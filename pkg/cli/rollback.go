package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"blcli/pkg/bootstrap"
)

var (
	rollbackModule       string
	rollbackComponent    string
	rollbackProject      string
	rollbackArgsPaths    []string
	rollbackWorkspace    string
	rollbackTemplateRepo string
	rollbackDryRun       bool
	rollbackAutoApprove  bool
	rollbackKubeconfig   string
	rollbackContext      string
)

// NewRollbackCommand creates the rollback command
func NewRollbackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback [terraform|kubernetes|gitops]",
		Short: "Rollback (delete) infrastructure resources",
		Long: `Rollback infrastructure resources by deleting them.

This command supports rolling back:
  - Terraform: Destroy Terraform resources
  - Kubernetes: Delete Kubernetes resources
  - GitOps: Delete ArgoCD Applications

The command supports custom rollback commands defined in config.yaml.
If no custom rollback command is specified, default deletion commands are used.

Examples:
  # Rollback all terraform resources
  blcli rollback terraform --args args.yaml

  # Rollback specific terraform project
  blcli rollback terraform --args args.yaml --project prd

  # Rollback specific kubernetes component
  blcli rollback kubernetes --args args.yaml --component istio

  # Rollback gitops application
  blcli rollback gitops --args args.yaml --component my-app

  # Preview rollback plan without executing
  blcli rollback terraform --args args.yaml --dry-run`,
		Example: `  # Rollback all terraform resources
  blcli rollback terraform --args args.yaml

  # Rollback specific terraform project
  blcli rollback terraform --args args.yaml --project prd

  # Rollback specific kubernetes component
  blcli rollback kubernetes --args args.yaml --component istio --kubeconfig ~/.kube/config

  # Preview rollback plan
  blcli rollback terraform --args args.yaml --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(rollbackArgsPaths) == 0 {
				return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
			}

			// Determine module from args or default
			module := rollbackModule
			if len(args) > 0 {
				module = args[0]
			}
			if module == "" {
				return fmt.Errorf("module type is required. Specify: terraform, kubernetes, or gitops")
			}

			// Validate module type
			validModules := map[string]bool{
				"terraform":  true,
				"kubernetes": true,
				"gitops":     true,
			}
			if !validModules[module] {
				return fmt.Errorf("invalid module type: %s. Valid types: terraform, kubernetes, gitops", module)
			}

			return bootstrap.ExecuteRollback(bootstrap.RollbackOptions{
				Module:       module,
				Component:    rollbackComponent,
				Project:      rollbackProject,
				Workspace:    rollbackWorkspace,
				ArgsPaths:    rollbackArgsPaths,
				TemplateRepo: rollbackTemplateRepo,
				DryRun:       rollbackDryRun,
				AutoApprove:  rollbackAutoApprove,
				Kubeconfig:   rollbackKubeconfig,
				Context:      rollbackContext,
			})
		},
	}

	cmd.Flags().StringVar(&rollbackModule, "module", "",
		"Module type: terraform, kubernetes, or gitops (can also be specified as positional argument)")
	cmd.Flags().StringVar(&rollbackComponent, "component", "",
		"Component name to rollback (optional, if not specified, all components will be rolled back)")
	cmd.Flags().StringVar(&rollbackProject, "project", "",
		"Project name to rollback (for terraform, optional)")
	cmd.Flags().StringArrayVar(&rollbackArgsPaths, "args", nil,
		"Path to YAML or TOML file with blcli configuration (required, can be specified multiple times)")
	cmd.Flags().StringVar(&rollbackWorkspace, "workspace", "",
		"Workspace directory path (default: from args.yaml global.workspace)")
	cmd.Flags().StringVarP(&rollbackTemplateRepo, "template-repo", "r", "",
		"Template repository URL or local path (optional, used to load rollback config from config.yaml)")
	cmd.Flags().BoolVar(&rollbackDryRun, "dry-run", false,
		"Preview rollback plan without executing")
	cmd.Flags().BoolVar(&rollbackAutoApprove, "auto-approve", false,
		"Auto approve rollback (skip confirmation prompts)")
	cmd.Flags().StringVar(&rollbackKubeconfig, "kubeconfig", "",
		"kubeconfig file path (for kubernetes/gitops, default: ~/.kube/config)")
	cmd.Flags().StringVar(&rollbackContext, "context", "",
		"Kubernetes context name (for kubernetes/gitops)")

	return cmd
}
