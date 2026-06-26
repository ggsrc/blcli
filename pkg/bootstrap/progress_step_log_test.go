package bootstrap_test

import (
	"strings"
	"testing"

	"blcli/pkg/bootstrap"
	"blcli/pkg/state"
)

func TestProgressTrackerRecordsMachineReadableStepLog(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tracker, err := bootstrap.NewProgressTracker("op-test-step-log", "init", true)
	if err != nil {
		t.Fatalf("NewProgressTracker() error = %v", err)
	}
	if err := tracker.StartOperation(); err != nil {
		t.Fatalf("StartOperation() error = %v", err)
	}
	if err := tracker.StartModule("terraform", 1); err != nil {
		t.Fatalf("StartModule() error = %v", err)
	}
	if err := tracker.StartStepWithCommand("terraform", "projects", "render terraform project directories"); err != nil {
		t.Fatalf("StartStepWithCommand() error = %v", err)
	}
	if err := tracker.RecordStepOutput("terraform", "projects", strings.Repeat("x", 4100)); err != nil {
		t.Fatalf("RecordStepOutput() error = %v", err)
	}
	if err := tracker.FailStepWithContext("terraform", "projects", "render failed", "workspace/terraform/gcp"); err != nil {
		t.Fatalf("FailStepWithContext() error = %v", err)
	}

	progress, err := state.LoadProgress("op-test-step-log")
	if err != nil {
		t.Fatalf("LoadProgress() error = %v", err)
	}
	step := progress.Modules["terraform"].Steps[0]
	if step.Command != "render terraform project directories" {
		t.Fatalf("command = %q", step.Command)
	}
	if step.ErrorLocation != "workspace/terraform/gcp" {
		t.Fatalf("error location = %q", step.ErrorLocation)
	}
	if step.OutputExcerpt == "" || !strings.HasSuffix(step.OutputExcerpt, "...(truncated)") {
		t.Fatalf("output excerpt was not truncated as expected: len=%d", len(step.OutputExcerpt))
	}
	if step.ErrorMessage != "render failed" {
		t.Fatalf("error message = %q", step.ErrorMessage)
	}
}
