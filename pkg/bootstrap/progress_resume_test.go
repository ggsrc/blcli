package bootstrap

import (
	"strings"
	"testing"
	"time"

	"blcli/pkg/state"
)

func TestLatestIncompleteProgress(t *testing.T) {
	older := &state.Progress{OperationID: "old", UpdatedAt: time.Now().Add(-time.Hour)}
	newer := &state.Progress{OperationID: "new", UpdatedAt: time.Now()}
	got := latestIncompleteProgress([]*state.Progress{older, newer, nil})
	if got == nil || got.OperationID != "new" {
		t.Fatalf("expected newest progress, got %#v", got)
	}
}

func TestReadYesNo(t *testing.T) {
	yes, err := readYesNo(strings.NewReader("y\n"), false)
	if err != nil || !yes {
		t.Fatalf("expected yes, got %v err=%v", yes, err)
	}
	no, err := readYesNo(strings.NewReader("\n"), false)
	if err != nil || no {
		t.Fatalf("expected no, got %v err=%v", no, err)
	}
}

func TestModuleAlreadyCompleted(t *testing.T) {
	tracker := &ProgressTracker{
		progress: &state.Progress{
			Modules: map[string]state.ModuleProgress{
				"terraform": {Name: "terraform", Status: "completed"},
				"kubernetes": {Name: "kubernetes", Status: "in_progress"},
			},
		},
	}
	if !ModuleAlreadyCompleted(tracker, "terraform") {
		t.Fatal("expected terraform completed")
	}
	if ModuleAlreadyCompleted(tracker, "kubernetes") {
		t.Fatal("expected kubernetes not completed")
	}
}
