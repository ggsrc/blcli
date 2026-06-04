package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"blcli/pkg/agent"
)

var (
	contractFormat  string
	contractCommand string
)

// NewContractCommand creates the contract command.
func NewContractCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract [command]",
		Short: "Print the AI-agent tool contract",
		Long: `Print the AI-agent tool contract for blcli.

The contract is intended for automation and AI agents that need stable command inputs,
outputs, examples, and operational guidance.`,
		Example: `  blcli contract --format json
  blcli contract init --format yaml
  blcli contract apply terraform`,
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := contractCommand
			if len(args) > 0 {
				if filter != "" {
					return fmt.Errorf("use either --command or positional command, not both")
				}
				filter = strings.Join(args, " ")
			}

			contract := agent.BuildToolContract(filter)
			if filter != "" && len(contract.Commands) == 0 {
				return fmt.Errorf("unknown command in contract: %s", filter)
			}

			return writeContractOutput(cmd.OutOrStdout(), contract, contractFormat)
		},
	}

	cmd.Flags().StringVar(&contractFormat, "format", "json",
		"Output format: table, json, or yaml (default: json)")
	cmd.Flags().StringVar(&contractCommand, "command", "",
		"Optional command filter, for example: init, status, or apply terraform")

	return cmd
}

func writeContractOutput(w io.Writer, contract agent.ToolContract, format string) error {
	switch strings.ToLower(format) {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(contract)
	case "yaml":
		data, err := yaml.Marshal(contract)
		if err != nil {
			return fmt.Errorf("failed to marshal contract: %w", err)
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	case "table":
		fmt.Fprintf(w, "blcli tool contract %s (%s)\n", contract.Contract.Version, contract.Contract.Stability)
		fmt.Fprintln(w, "Commands:")
		for _, command := range contract.Commands {
			fmt.Fprintf(w, "  %-18s %s\n", command.Name, command.Summary)
		}
		return nil
	default:
		return fmt.Errorf("unsupported format: %s. Use table, json, or yaml", format)
	}
}
