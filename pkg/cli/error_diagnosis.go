package cli

import (
	"fmt"
	"io"

	"blcli/pkg/agent"
)

func executeWithDiagnosis(cmdRunner func() error, stderr io.Writer) error {
	err := cmdRunner()
	if err == nil {
		return nil
	}

	fmt.Fprintln(stderr, err)
	writeRuntimeDiagnosis(stderr, err.Error())
	return err
}

func writeRuntimeDiagnosis(w io.Writer, message string) {
	diagnosis := agent.DiagnoseFailure(message)
	if diagnosis.Category == "unknown" {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Diagnosis: %s (%s confidence)\n", diagnosis.Category, diagnosis.Confidence)
	fmt.Fprintf(w, "Summary: %s\n", diagnosis.Summary)
	fmt.Fprintf(w, "Likely cause: %s\n", diagnosis.LikelyCause)
	fmt.Fprintln(w, "Next steps:")
	for _, step := range diagnosis.NextSteps {
		fmt.Fprintf(w, "  - %s\n", step)
	}
	if len(diagnosis.RepairCommands) > 0 {
		fmt.Fprintln(w, "Repair commands:")
		for _, command := range diagnosis.RepairCommands {
			fmt.Fprintf(w, "  - %s\n", command)
		}
	}
	fmt.Fprintln(w, "Machine-readable diagnosis: blcli diagnose --file <captured-log> --format json")
}
