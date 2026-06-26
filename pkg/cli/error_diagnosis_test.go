package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestExecuteWithDiagnosisWritesKnownDiagnosis(t *testing.T) {
	var stderr bytes.Buffer

	err := executeWithDiagnosis(func() error {
		return fmt.Errorf("terraform apply failed: Error 409: resource already exists")
	}, &stderr)

	if err == nil {
		t.Fatal("expected error")
	}
	output := stderr.String()
	if !containsAll(output, []string{
		"terraform apply failed",
		"Diagnosis: resource_already_exists",
		"Repair commands:",
		"blcli diagnose --file <captured-log> --format json",
	}) {
		t.Fatalf("diagnosis output missing expected content:\n%s", output)
	}
}

func TestExecuteWithDiagnosisSkipsUnknownDiagnosis(t *testing.T) {
	var stderr bytes.Buffer

	err := executeWithDiagnosis(func() error {
		return fmt.Errorf("plain validation error")
	}, &stderr)

	if err == nil {
		t.Fatal("expected error")
	}
	output := stderr.String()
	if output != "plain validation error\n" {
		t.Fatalf("stderr = %q, want only the original error", output)
	}
}

func containsAll(output string, substrings []string) bool {
	for _, substring := range substrings {
		if !strings.Contains(output, substring) {
			return false
		}
	}
	return true
}
