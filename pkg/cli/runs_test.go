package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"blcli/pkg/cli"
	"blcli/pkg/state"
)

func TestRunsListAndShowJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	started := time.Date(2026, 5, 29, 10, 30, 0, 0, time.UTC)
	progress := &state.Progress{
		OperationID:    "op-20260529-103000-app",
		Type:           "apply",
		StartedAt:      started,
		UpdatedAt:      started.Add(2 * time.Minute),
		Status:         "failed",
		TotalSteps:     2,
		CompletedSteps: 1,
		CurrentStep:    2,
		Modules: map[string]state.ModuleProgress{
			"terraform": {
				Name:           "terraform",
				Status:         "failed",
				TotalSteps:     2,
				CompletedSteps: 1,
				Steps: []state.StepProgress{
					{Name: "init", Status: "completed"},
					{Name: "apply", Status: "failed", ErrorMessage: "Error 409: resource already exists"},
				},
				ErrorMessage: "Error 409: resource already exists",
			},
		},
	}
	if err := state.SaveProgress(progress); err != nil {
		t.Fatalf("SaveProgress() error = %v", err)
	}

	cmd := cli.NewRunsListCommand()
	var listOutput bytes.Buffer
	cmd.SetOut(&listOutput)
	cmd.SetArgs([]string{"--status", "failed", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("runs list error = %v", err)
	}
	var listResult cli.RunsResult
	if err := json.Unmarshal(listOutput.Bytes(), &listResult); err != nil {
		t.Fatalf("runs list JSON error = %v\n%s", err, listOutput.String())
	}
	if len(listResult.Runs) != 1 {
		t.Fatalf("runs length = %d, want 1", len(listResult.Runs))
	}
	if listResult.Runs[0].OperationID != progress.OperationID {
		t.Fatalf("operation id = %q, want %q", listResult.Runs[0].OperationID, progress.OperationID)
	}

	showCmd := cli.NewRunsShowCommand()
	var showOutput bytes.Buffer
	showCmd.SetOut(&showOutput)
	showCmd.SetArgs([]string{progress.OperationID, "--format", "json"})
	if err := showCmd.Execute(); err != nil {
		t.Fatalf("runs show error = %v", err)
	}
	var showResult cli.RunsResult
	if err := json.Unmarshal(showOutput.Bytes(), &showResult); err != nil {
		t.Fatalf("runs show JSON error = %v\n%s", err, showOutput.String())
	}
	if len(showResult.Runs) != 1 || showResult.Runs[0].Modules["terraform"].ErrorMessage == "" {
		t.Fatalf("runs show missing expected module error: %#v", showResult.Runs)
	}

	progressPath := filepath.Join(home, ".blcli", "progress", progress.OperationID+".yaml")
	if _, err := os.Stat(progressPath); err != nil {
		t.Fatalf("expected progress file at %s: %v", progressPath, err)
	}
}

func TestRunsShowMissingRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := cli.NewRunsShowCommand()
	cmd.SetArgs([]string{"missing-run"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing run error")
	}
}
