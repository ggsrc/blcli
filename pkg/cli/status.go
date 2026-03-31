package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"blcli/pkg/bootstrap"
)

var (
	statusType         string
	statusArgsPaths    []string
	statusWorkspace    string
	statusFormat       string
	statusKubeconfig   string
	statusContext      string
	statusTemplateRepo string
)

// NewStatusCommand creates the status command
func NewStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [terraform|kubernetes|gitops|all]",
		Short: "Check infrastructure resources status",
		Long: `Check the status of deployed infrastructure resources.

This command checks:
  - Terraform: Resource status using 'terraform show'
  - Kubernetes: Resource status using 'kubectl get'
  - GitOps: ArgoCD Application sync and health status

Examples:
  # Check all modules status
  blcli status --args args.yaml

  # Check specific module
  blcli status terraform --args args.yaml
  blcli status kubernetes --args args.yaml --kubeconfig ~/.kube/config

  # Output in JSON format
  blcli status --args args.yaml --format json

  # Check with specific workspace
  blcli status --args args.yaml --workspace ./workspace/output`,
		Example: `  # Check all modules
  blcli status --args args.yaml

  # Check terraform only
  blcli status terraform --args args.yaml

  # Check kubernetes with kubeconfig
  blcli status kubernetes --args args.yaml --kubeconfig ~/.kube/config --context my-cluster

  # Output in JSON format
  blcli status --args args.yaml --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(statusArgsPaths) == 0 {
				return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
			}

			// Determine type from args or default to "all"
			statusTypeValue := statusType
			if len(args) > 0 {
				statusTypeValue = args[0]
			}
			if statusTypeValue == "" {
				statusTypeValue = "all"
			}

			// Validate type
			validTypes := map[string]bool{
				"terraform":  true,
				"kubernetes": true,
				"gitops":     true,
				"all":        true,
			}
			if !validTypes[statusTypeValue] {
				return fmt.Errorf("invalid type: %s. Valid types: terraform, kubernetes, gitops, all", statusTypeValue)
			}

			return bootstrap.ExecuteStatus(bootstrap.StatusOptions{
				Type:         statusTypeValue,
				ArgsPaths:    statusArgsPaths,
				Workspace:    statusWorkspace,
				Format:       statusFormat,
				Kubeconfig:   statusKubeconfig,
				Context:      statusContext,
				TemplateRepo: statusTemplateRepo,
			})
		},
	}

	cmd.Flags().StringVar(&statusType, "type", "",
		"Type to check: terraform, kubernetes, gitops, or all (default: all)")
	cmd.Flags().StringArrayVar(&statusArgsPaths, "args", nil,
		"Path to YAML or TOML file with blcli configuration (required, can be specified multiple times)")
	cmd.Flags().StringVar(&statusWorkspace, "workspace", "",
		"Workspace directory path (default: from args.yaml global.workspace)")
	cmd.Flags().StringVar(&statusFormat, "format", "table",
		"Output format: table, json, or yaml (default: table)")
	cmd.Flags().StringVar(&statusKubeconfig, "kubeconfig", "",
		"kubeconfig file path (default: ~/.kube/config)")
	cmd.Flags().StringVar(&statusContext, "context", "",
		"Kubernetes context name")
	cmd.Flags().StringVarP(&statusTemplateRepo, "template-repo", "r", "",
		"Template repository URL or local path (optional, used to load component configs)")

	return cmd
}
