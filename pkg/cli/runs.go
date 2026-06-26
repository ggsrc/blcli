package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"blcli/pkg/state"
)

var (
	runsFormat string
	runsStatus string
)

// RunsResult is a machine-readable response for blcli runs list/show.
type RunsResult struct {
	SchemaVersion string            `json:"schema_version" yaml:"schema_version"`
	Runs          []*state.Progress `json:"runs" yaml:"runs"`
}

const runsSchemaVersion = "blcli.runs/v1"

// NewRunsCommand creates the runs command.
func NewRunsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List and inspect persisted blcli run records",
		Long: `List and inspect persisted blcli run records.

Run records are stored under ~/.blcli/progress and are keyed by operation id.`,
	}

	cmd.AddCommand(NewRunsListCommand())
	cmd.AddCommand(NewRunsShowCommand())
	return cmd
}

// NewRunsListCommand creates the runs list command.
func NewRunsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List persisted run records",
		Example: `  blcli runs list
  blcli runs list --status failed --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			runs, err := state.ListProgress()
			if err != nil {
				return err
			}
			runs = filterRunsByStatus(runs, runsStatus)
			sortRuns(runs)
			return writeRunsOutput(cmd.OutOrStdout(), RunsResult{
				SchemaVersion: runsSchemaVersion,
				Runs:          runs,
			}, runsFormat)
		},
	}

	cmd.Flags().StringVar(&runsFormat, "format", "table",
		"Output format: table, json, or yaml (default: table)")
	cmd.Flags().StringVar(&runsStatus, "status", "",
		"Filter by status: pending, in_progress, completed, failed, cancelled")

	return cmd
}

// NewRunsShowCommand creates the runs show command.
func NewRunsShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <operation-id>",
		Short: "Show one persisted run record",
		Args:  cobra.ExactArgs(1),
		Example: `  blcli runs show op-20260529-103000-app
  blcli runs show op-20260529-103000-app --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			progress, err := state.LoadProgress(args[0])
			if err != nil {
				return err
			}
			if progress == nil {
				return fmt.Errorf("run not found: %s", args[0])
			}

			return writeRunsOutput(cmd.OutOrStdout(), RunsResult{
				SchemaVersion: runsSchemaVersion,
				Runs:          []*state.Progress{progress},
			}, runsFormat)
		},
	}

	cmd.Flags().StringVar(&runsFormat, "format", "table",
		"Output format: table, json, or yaml (default: table)")

	return cmd
}

func filterRunsByStatus(runs []*state.Progress, status string) []*state.Progress {
	if status == "" {
		return runs
	}
	filtered := make([]*state.Progress, 0, len(runs))
	for _, run := range runs {
		if strings.EqualFold(run.Status, status) {
			filtered = append(filtered, run)
		}
	}
	return filtered
}

func sortRuns(runs []*state.Progress) {
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
}

func writeRunsOutput(w io.Writer, result RunsResult, format string) error {
	switch strings.ToLower(format) {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "yaml":
		data, err := yaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal runs: %w", err)
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	case "table":
		if len(result.Runs) == 0 {
			fmt.Fprintln(w, "No runs found.")
			return nil
		}
		fmt.Fprintln(w, "Run ID                         Type    Status       Steps     Started")
		for _, run := range result.Runs {
			fmt.Fprintf(w, "%-30s %-7s %-12s %d/%d     %s\n",
				run.OperationID,
				run.Type,
				run.Status,
				run.CompletedSteps,
				run.TotalSteps,
				run.StartedAt.Format("2006-01-02 15:04:05"),
			)
		}
		return nil
	default:
		return fmt.Errorf("unsupported format: %s. Use table, json, or yaml", format)
	}
}
