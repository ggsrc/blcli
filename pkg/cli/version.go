package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCommand creates the version command
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  "Display version information for blcli",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s version %s\n", appName, getVersion())
		},
	}

	return cmd
}
