package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	appName = "blcli"
)

var (
	// appVersion is set at build time via -ldflags
	// Format: major.minor (e.g., "0.1")
	appVersion = "0.1"
	// buildTime is set at build time via -ldflags
	// Format: timestamp (e.g., "20240114195830")
	// If not set at build time, will be generated at runtime
	buildTime = ""
)

// getVersion returns the version string in format "major.minor+timestamp"
func getVersion() string {
	timestamp := buildTime
	if timestamp == "" {
		// If buildTime is not set at build time, use current time
		timestamp = time.Now().Format("20060102150405")
	}
	return fmt.Sprintf("%s+%s", appVersion, timestamp)
}

var (
	// Global flags
	templateRepo string
)

// NewRootCommand creates the root command for blcli
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "blcli",
		Short: "A command-line tool for bootstrapping and managing infrastructure projects",
		Long: `blcli helps you initialize and manage your project infrastructure with a set of simple commands.
It supports Terraform, Kubernetes, and GitOps configurations.`,
		Version: getVersion(),
	}

	// No global flags for now (args is handled per-command)

	// Add subcommands
	rootCmd.AddCommand(NewInitCommand())
	rootCmd.AddCommand(NewInitArgsCommand())
	rootCmd.AddCommand(NewExplainCommand())
	rootCmd.AddCommand(NewDestroyCommand())
	rootCmd.AddCommand(NewVersionCommand())
	rootCmd.AddCommand(NewCheckCommand())
	rootCmd.AddCommand(NewApplyCommand())
	rootCmd.AddCommand(NewStatusCommand())
	rootCmd.AddCommand(NewRollbackCommand())

	return rootCmd
}

// Execute runs the root command
func Execute() {
	rootCmd := NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
