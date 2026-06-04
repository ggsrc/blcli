package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blcli/pkg/state"
)

func withFakeExternalExecutor(t *testing.T, fake externalCommandExecutorFunc) {
	t.Helper()
	previous := externalCommandExecutor
	externalCommandExecutor = fake
	t.Cleanup(func() {
		externalCommandExecutor = previous
	})
}

func fakeExternalResult(spec ExternalCommandSpec, output string) ExternalCommandResult {
	return ExternalCommandResult{
		Name:    spec.Name,
		Command: externalCommandString(spec.Name, spec.Args),
		Args:    append([]string(nil), spec.Args...),
		Dir:     spec.Dir,
		Output:  []byte(output),
	}
}

func newQuietProgressStep(t *testing.T, operationID, module, step, command, location string) (*ProgressTracker, progressStepLogger) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	tracker, err := NewProgressTracker(operationID, "apply", true)
	if err != nil {
		t.Fatalf("NewProgressTracker() error = %v", err)
	}
	if err := tracker.StartOperation(); err != nil {
		t.Fatalf("StartOperation() error = %v", err)
	}
	if err := tracker.StartModule(module, 1); err != nil {
		t.Fatalf("StartModule() error = %v", err)
	}
	return tracker, startProgressStep(tracker, module, step, command, location)
}

func TestApplyKubernetesKubectlWithProgressRecordsCommandOutput(t *testing.T) {
	componentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(componentDir, "kustomization.yaml"), []byte("resources: []\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, stepLogger := newQuietProgressStep(
		t,
		"op-apply-kubernetes-step-log",
		"kubernetes",
		"stg/argocd",
		"apply kubernetes component",
		componentDir,
	)

	var command string
	withFakeExternalExecutor(t, func(ctx context.Context, spec ExternalCommandSpec, mode externalCommandOutputMode) (ExternalCommandResult, error) {
		command = externalCommandString(spec.Name, spec.Args)
		return fakeExternalResult(spec, "deployment.apps/argocd configured"), nil
	})

	if err := applyWithKubectlWithProgress(context.Background(), ApplyKubernetesOptions{}, componentDir, stepLogger); err != nil {
		t.Fatalf("applyWithKubectlWithProgress() error = %v", err)
	}
	stepLogger.complete()

	if !strings.HasPrefix(command, "kubectl apply -k ") {
		t.Fatalf("command = %q", command)
	}

	progress, err := state.LoadProgress("op-apply-kubernetes-step-log")
	if err != nil {
		t.Fatalf("LoadProgress() error = %v", err)
	}
	step := progress.Modules["kubernetes"].Steps[0]
	if step.Status != "completed" {
		t.Fatalf("status = %q", step.Status)
	}
	if step.Command != "apply kubernetes component" {
		t.Fatalf("step command = %q", step.Command)
	}
	if !strings.Contains(step.OutputExcerpt, "$ kubectl apply -k") {
		t.Fatalf("output excerpt missing kubectl command: %q", step.OutputExcerpt)
	}
	if !strings.Contains(step.OutputExcerpt, "deployment.apps/argocd configured") {
		t.Fatalf("output excerpt missing kubectl output: %q", step.OutputExcerpt)
	}
}

func TestApplyKubernetesWithProgressRecordsFailureLocation(t *testing.T) {
	componentDir := t.TempDir()
	_, stepLogger := newQuietProgressStep(
		t,
		"op-apply-kubernetes-step-log-failed",
		"kubernetes",
		"stg/argocd",
		"apply kubernetes component",
		componentDir,
	)

	withFakeExternalExecutor(t, func(ctx context.Context, spec ExternalCommandSpec, mode externalCommandOutputMode) (ExternalCommandResult, error) {
		return fakeExternalResult(spec, "error: failed to create resource"), errors.New("exit status 1")
	})

	err := applyWithKubectlWithProgress(context.Background(), ApplyKubernetesOptions{}, componentDir, stepLogger)
	if err == nil {
		t.Fatal("applyWithKubectlWithProgress() error = nil")
	}
	stepLogger.fail(err, componentDir)

	progress, loadErr := state.LoadProgress("op-apply-kubernetes-step-log-failed")
	if loadErr != nil {
		t.Fatalf("LoadProgress() error = %v", loadErr)
	}
	step := progress.Modules["kubernetes"].Steps[0]
	if step.Status != "failed" {
		t.Fatalf("status = %q", step.Status)
	}
	if step.ErrorLocation != componentDir {
		t.Fatalf("error location = %q", step.ErrorLocation)
	}
	if !strings.Contains(step.OutputExcerpt, "error: failed to create resource") {
		t.Fatalf("output excerpt missing error output: %q", step.OutputExcerpt)
	}
}

func TestApplyGitOpsKubectlWithProgressRecordsCommandOutput(t *testing.T) {
	appYaml := filepath.Join(t.TempDir(), "app.yaml")
	_, stepLogger := newQuietProgressStep(
		t,
		"op-apply-gitops-step-log",
		"gitops",
		"stg/api",
		"kubectl apply argocd application",
		appYaml,
	)

	withFakeExternalExecutor(t, func(ctx context.Context, spec ExternalCommandSpec, mode externalCommandOutputMode) (ExternalCommandResult, error) {
		return fakeExternalResult(spec, "application.argoproj.io/api configured"), nil
	})

	if err := runKubectlApplyWithProgress(context.Background(), ApplyGitOpsOptions{}, appYaml, stepLogger); err != nil {
		t.Fatalf("runKubectlApplyWithProgress() error = %v", err)
	}
	stepLogger.complete()

	progress, err := state.LoadProgress("op-apply-gitops-step-log")
	if err != nil {
		t.Fatalf("LoadProgress() error = %v", err)
	}
	step := progress.Modules["gitops"].Steps[0]
	if step.Status != "completed" {
		t.Fatalf("status = %q", step.Status)
	}
	if !strings.Contains(step.OutputExcerpt, "$ kubectl apply -f") {
		t.Fatalf("output excerpt missing kubectl command: %q", step.OutputExcerpt)
	}
	if !strings.Contains(step.OutputExcerpt, "application.argoproj.io/api configured") {
		t.Fatalf("output excerpt missing gitops output: %q", step.OutputExcerpt)
	}
}
