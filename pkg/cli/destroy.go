package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"blcli/pkg/bootstrap"
)

var (
	destroyArgsPaths []string
)

// NewDestroyCommand creates the destroy command
func NewDestroyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [modules...]",
		Short: "Destroy projects that were initialized by blcli init",
		Long: `Destroy projects that were initialized by blcli init.

⚠️  WARNING: This command requires confirmation by typing 'yes' before proceeding.
This is a destructive operation that cannot be undone.`,
		Example: `  # Destroy all modules
  blcli destroy --args=args.toml

  # Destroy specific modules
  blcli destroy terraform --args=args.toml
  blcli destroy kubernetes gitops --args=args.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(destroyArgsPaths) == 0 {
				return fmt.Errorf("args file is required. Use --args to specify one or more args files (YAML or TOML)")
			}
			return bootstrap.ExecuteDestroy(bootstrap.DestroyOptions{
				Modules:   args,
				ArgsPaths: destroyArgsPaths,
			})
		},
	}

	cmd.Flags().StringArrayVar(&destroyArgsPaths, "args", nil,
		"Path to YAML or TOML file with blcli configuration (required, can be specified multiple times)")

	return cmd
}
