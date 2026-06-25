package agent_test

import (
	"testing"

	"blcli/pkg/agent"
)

func TestDiagnoseFailureClassifiesKnownFailure(t *testing.T) {
	diagnosis := agent.DiagnoseFailure("terraform apply failed: Error 409: resource already exists")

	if diagnosis.SchemaVersion != agent.FailureDiagnosisSchemaVersion {
		t.Fatalf("schema version = %q, want %q", diagnosis.SchemaVersion, agent.FailureDiagnosisSchemaVersion)
	}
	if diagnosis.Category != "resource_already_exists" {
		t.Fatalf("category = %q, want resource_already_exists", diagnosis.Category)
	}
	if diagnosis.Confidence == "low" {
		t.Fatalf("confidence = %q, want medium or high", diagnosis.Confidence)
	}
	if len(diagnosis.RepairCommands) == 0 {
		t.Fatal("expected repair commands")
	}
}

func TestDiagnoseFailureFallsBackToUnknown(t *testing.T) {
	diagnosis := agent.DiagnoseFailure("something unexpected happened")

	if diagnosis.Category != "unknown" {
		t.Fatalf("category = %q, want unknown", diagnosis.Category)
	}
	if diagnosis.Confidence != "low" {
		t.Fatalf("confidence = %q, want low", diagnosis.Confidence)
	}
}
