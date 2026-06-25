package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"blcli/pkg/agent"
)

var (
	diagnoseMessage string
	diagnoseFile    string
	diagnoseFormat  string
)

// NewDiagnoseCommand creates the diagnose command.
func NewDiagnoseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnose [message]",
		Short: "Classify a failure and suggest repair steps",
		Long: `Classify a blcli, Terraform, Kubernetes, GitHub, or ArgoCD failure message.

The result includes a stable category, confidence, matched keywords, likely cause,
next steps, and repair commands. Use --format json for automation.`,
		Example: `  blcli diagnose --message "Error 409: already exists" --format json
  blcli diagnose --file execution_stage5.log
  blcli diagnose "kubectl: error: open ~/.kube/config.lock"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			message, err := readDiagnosisMessage(args)
			if err != nil {
				return err
			}

			diagnosis := agent.DiagnoseFailure(message)
			return writeDiagnosisOutput(cmd.OutOrStdout(), diagnosis, diagnoseFormat)
		},
	}

	cmd.Flags().StringVar(&diagnoseMessage, "message", "",
		"Failure message to classify")
	cmd.Flags().StringVar(&diagnoseFile, "file", "",
		"Path to a file containing command output or an error message")
	cmd.Flags().StringVar(&diagnoseFormat, "format", "table",
		"Output format: table, json, or yaml (default: table)")

	return cmd
}

func readDiagnosisMessage(args []string) (string, error) {
	sources := 0
	if diagnoseMessage != "" {
		sources++
	}
	if diagnoseFile != "" {
		sources++
	}
	if len(args) > 0 {
		sources++
	}
	if sources != 1 {
		return "", fmt.Errorf("provide exactly one failure message source: --message, --file, or positional message")
	}

	if diagnoseMessage != "" {
		return diagnoseMessage, nil
	}
	if diagnoseFile != "" {
		data, err := os.ReadFile(diagnoseFile)
		if err != nil {
			return "", fmt.Errorf("failed to read diagnosis file %s: %w", diagnoseFile, err)
		}
		return string(data), nil
	}
	return strings.Join(args, " "), nil
}

func writeDiagnosisOutput(w io.Writer, diagnosis agent.FailureDiagnosis, format string) error {
	switch strings.ToLower(format) {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(diagnosis)
	case "yaml":
		data, err := yaml.Marshal(diagnosis)
		if err != nil {
			return fmt.Errorf("failed to marshal diagnosis: %w", err)
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
	case "table":
		fmt.Fprintf(w, "Category: %s\n", diagnosis.Category)
		fmt.Fprintf(w, "Confidence: %s\n", diagnosis.Confidence)
		if len(diagnosis.MatchedKeywords) > 0 {
			fmt.Fprintf(w, "Matched: %s\n", strings.Join(diagnosis.MatchedKeywords, ", "))
		}
		fmt.Fprintf(w, "Summary: %s\n", diagnosis.Summary)
		fmt.Fprintf(w, "Likely cause: %s\n", diagnosis.LikelyCause)
		fmt.Fprintln(w, "Next steps:")
		for _, step := range diagnosis.NextSteps {
			fmt.Fprintf(w, "  - %s\n", step)
		}
		fmt.Fprintln(w, "Repair commands:")
		for _, command := range diagnosis.RepairCommands {
			fmt.Fprintf(w, "  - %s\n", command)
		}
		return nil
	default:
		return fmt.Errorf("unsupported format: %s. Use table, json, or yaml", format)
	}
}
