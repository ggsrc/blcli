package agent_test

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"blcli/pkg/agent"
)

type failureFixtureMetadata struct {
	Fixtures []failureFixture `yaml:"fixtures"`
}

type failureFixture struct {
	File             string `yaml:"file"`
	ExpectedCategory string `yaml:"expected_category"`
}

func TestFailureFixturesMatchExpectedCategories(t *testing.T) {
	metadataPath := filepath.Join("..", "..", "integration", "fixtures", "failures", "metadata.yaml")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var metadata failureFixtureMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if len(metadata.Fixtures) == 0 {
		t.Fatal("expected at least one failure fixture")
	}

	for _, fixture := range metadata.Fixtures {
		fixture := fixture
		t.Run(fixture.File, func(t *testing.T) {
			logPath := filepath.Join("..", "..", "integration", "fixtures", "failures", fixture.File)
			logData, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			diagnosis := agent.DiagnoseFailure(string(logData))
			if diagnosis.Category != fixture.ExpectedCategory {
				t.Fatalf("category = %q, want %q", diagnosis.Category, fixture.ExpectedCategory)
			}
			if len(diagnosis.NextSteps) == 0 {
				t.Fatal("expected next steps")
			}
			if len(diagnosis.RepairCommands) == 0 {
				t.Fatal("expected repair commands")
			}
		})
	}
}
