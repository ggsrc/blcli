package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"blcli/pkg/state"
)

func withFakeTerraformExecutor(t *testing.T, fake terraformCommandExecutorFunc) {
	t.Helper()
	previous := terraformCommandExecutor
	terraformCommandExecutor = fake
	t.Cleanup(func() {
		terraformCommandExecutor = previous
	})
}

func fakeTerraformResult(args []string, output string) TerraformCommandResult {
	return TerraformCommandResult{
		Command: terraformCommandString(args),
		Args:    append([]string(nil), args...),
		Output:  []byte(output),
	}
}

func TestTerraformCommandResultUsesFakeExecutor(t *testing.T) {
	var seenMode terraformCommandOutputMode
	withFakeTerraformExecutor(t, func(ctx context.Context, args []string, emulator *GCSEmulator, mode terraformCommandOutputMode) (TerraformCommandResult, error) {
		seenMode = mode
		return fakeTerraformResult(args, "planned"), nil
	})

	result, err := runTerraformCommandResult(
		context.Background(),
		[]string{"plan", "-var", "project name=demo"},
		nil,
		terraformCommandCapture,
	)
	if err != nil {
		t.Fatalf("runTerraformCommandResult() error = %v", err)
	}
	if seenMode != terraformCommandCapture {
		t.Fatalf("mode = %q", seenMode)
	}
	if result.Command != "terraform plan -var 'project name=demo'" {
		t.Fatalf("command = %q", result.Command)
	}
	if string(result.Output) != "planned" {
		t.Fatalf("output = %q", result.Output)
	}
}

func TestApplyTerraformDirectoryWithProgressRecordsCommandOutput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()

	tracker, err := NewProgressTracker("op-apply-terraform-step-log", "apply", true)
	if err != nil {
		t.Fatalf("NewProgressTracker() error = %v", err)
	}
	if err := tracker.StartOperation(); err != nil {
		t.Fatalf("StartOperation() error = %v", err)
	}
	if err := tracker.StartModule("terraform", 1); err != nil {
		t.Fatalf("StartModule() error = %v", err)
	}
	stepLogger := startTerraformApplyStep(tracker, "gcp/test", projectDir)

	var commands []string
	withFakeTerraformExecutor(t, func(ctx context.Context, args []string, emulator *GCSEmulator, mode terraformCommandOutputMode) (TerraformCommandResult, error) {
		commands = append(commands, terraformCommandString(args))
		switch args[0] {
		case "init":
			return fakeTerraformResult(args, "Terraform has been successfully initialized!"), nil
		case "validate":
			return fakeTerraformResult(args, "Success! The configuration is valid."), nil
		case "plan":
			return fakeTerraformResult(args, "Plan: 1 to add, 0 to change, 0 to destroy."), nil
		case "apply":
			return fakeTerraformResult(args, "Apply complete! Resources: 1 added, 0 changed, 0 destroyed."), nil
		default:
			return fakeTerraformResult(args, "unexpected command"), errors.New("unexpected terraform command")
		}
	})

	if err := applyTerraformDirectoryWithProgress(context.Background(), projectDir, true, true, nil, stepLogger); err != nil {
		t.Fatalf("applyTerraformDirectoryWithProgress() error = %v", err)
	}
	stepLogger.complete()

	if len(commands) != 4 {
		t.Fatalf("commands = %v", commands)
	}

	progress, err := state.LoadProgress("op-apply-terraform-step-log")
	if err != nil {
		t.Fatalf("LoadProgress() error = %v", err)
	}
	step := progress.Modules["terraform"].Steps[0]
	if step.Status != "completed" {
		t.Fatalf("status = %q", step.Status)
	}
	if step.Command != "terraform apply directory" {
		t.Fatalf("step command = %q", step.Command)
	}
	if !strings.Contains(step.OutputExcerpt, "$ terraform apply -input=false -auto-approve tfplan") {
		t.Fatalf("output excerpt missing apply command: %q", step.OutputExcerpt)
	}
	if !strings.Contains(step.OutputExcerpt, "Apply complete!") {
		t.Fatalf("output excerpt missing apply output: %q", step.OutputExcerpt)
	}
	if step.ErrorLocation != "" {
		t.Fatalf("error location = %q", step.ErrorLocation)
	}
}

func TestApplyTerraformDirectoryWithProgressRecordsFailureLocation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()

	tracker, err := NewProgressTracker("op-apply-terraform-step-log-failed", "apply", true)
	if err != nil {
		t.Fatalf("NewProgressTracker() error = %v", err)
	}
	if err := tracker.StartOperation(); err != nil {
		t.Fatalf("StartOperation() error = %v", err)
	}
	if err := tracker.StartModule("terraform", 1); err != nil {
		t.Fatalf("StartModule() error = %v", err)
	}
	stepLogger := startTerraformApplyStep(tracker, "gcp/test", projectDir)

	withFakeTerraformExecutor(t, func(ctx context.Context, args []string, emulator *GCSEmulator, mode terraformCommandOutputMode) (TerraformCommandResult, error) {
		switch args[0] {
		case "init":
			return fakeTerraformResult(args, "Terraform has been successfully initialized!"), nil
		case "validate":
			return fakeTerraformResult(args, "Error: Missing required provider"), errors.New("exit status 1")
		default:
			return fakeTerraformResult(args, "unexpected command"), errors.New("unexpected terraform command")
		}
	})

	err = applyTerraformDirectoryWithProgress(context.Background(), projectDir, true, true, nil, stepLogger)
	if err == nil {
		t.Fatal("applyTerraformDirectoryWithProgress() error = nil")
	}
	stepLogger.fail(err, projectDir)

	progress, loadErr := state.LoadProgress("op-apply-terraform-step-log-failed")
	if loadErr != nil {
		t.Fatalf("LoadProgress() error = %v", loadErr)
	}
	step := progress.Modules["terraform"].Steps[0]
	if step.Status != "failed" {
		t.Fatalf("status = %q", step.Status)
	}
	if step.ErrorLocation != projectDir {
		t.Fatalf("error location = %q", step.ErrorLocation)
	}
	if !strings.Contains(step.OutputExcerpt, "$ terraform validate") {
		t.Fatalf("output excerpt missing validate command: %q", step.OutputExcerpt)
	}
	if !strings.Contains(step.ErrorMessage, "terraform validate failed") {
		t.Fatalf("error message = %q", step.ErrorMessage)
	}
}
