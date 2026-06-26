package agent_test

import (
	"testing"

	"blcli/pkg/agent"
)

func TestBuildToolContractCanFilterCommand(t *testing.T) {
	contract := agent.BuildToolContract("apply terraform")

	if contract.SchemaVersion != agent.ToolContractSchemaVersion {
		t.Fatalf("schema version = %q, want %q", contract.SchemaVersion, agent.ToolContractSchemaVersion)
	}
	if len(contract.Commands) != 1 {
		t.Fatalf("commands length = %d, want 1", len(contract.Commands))
	}
	if contract.Commands[0].Name != "apply terraform" {
		t.Fatalf("command name = %q, want apply terraform", contract.Commands[0].Name)
	}
	if len(contract.Commands[0].Inputs) == 0 {
		t.Fatal("expected command inputs")
	}
	if contract.Compatibility.Strategy == "" {
		t.Fatal("expected compatibility strategy")
	}
	if contract.Commands[0].InputSchema.Properties["dir"].Type != "string" {
		t.Fatalf("dir schema type = %q, want string", contract.Commands[0].InputSchema.Properties["dir"].Type)
	}
	if !contains(contract.Commands[0].InputSchema.Required, "dir") {
		t.Fatalf("required = %v, want dir", contract.Commands[0].InputSchema.Required)
	}
	if contract.Commands[0].InputSchema.Properties["dry_run"].Default != false {
		t.Fatalf("dry_run default = %#v, want false", contract.Commands[0].InputSchema.Properties["dry_run"].Default)
	}
	if contract.Commands[0].OutputSchema.Properties["execution_plan"].Type != "string" {
		t.Fatalf("execution_plan output schema type = %q, want string", contract.Commands[0].OutputSchema.Properties["execution_plan"].Type)
	}
}

func TestBuildToolContractUnknownFilterReturnsNoCommands(t *testing.T) {
	contract := agent.BuildToolContract("does-not-exist")

	if len(contract.Commands) != 0 {
		t.Fatalf("commands length = %d, want 0", len(contract.Commands))
	}
}

func TestBuildToolContractRepeatableInputsUseArraySchema(t *testing.T) {
	contract := agent.BuildToolContract("status")
	command := contract.Commands[0]
	argsSchema := command.InputSchema.Properties["args"]

	if argsSchema.Type != "array" {
		t.Fatalf("args schema type = %q, want array", argsSchema.Type)
	}
	if argsSchema.Items == nil || argsSchema.Items.Type != "string" {
		t.Fatalf("args item schema = %#v, want string items", argsSchema.Items)
	}
}

func TestBuildToolContractDiagnoseOutputSchemaIsStrict(t *testing.T) {
	contract := agent.BuildToolContract("diagnose")
	command := contract.Commands[0]

	if command.OutputSchema.Properties["schema_version"].Enum[0] != agent.FailureDiagnosisSchemaVersion {
		t.Fatalf("diagnose schema_version enum = %v, want %s", command.OutputSchema.Properties["schema_version"].Enum, agent.FailureDiagnosisSchemaVersion)
	}
	if command.OutputSchema.AdditionalProperties == nil || *command.OutputSchema.AdditionalProperties {
		t.Fatal("expected diagnose output schema to disallow additional properties")
	}
	if !contains(command.OutputSchema.Required, "repair_commands") {
		t.Fatalf("required = %v, want repair_commands", command.OutputSchema.Required)
	}
}

func TestBuildToolContractRunsOutputSchemaIsVersioned(t *testing.T) {
	contract := agent.BuildToolContract("runs list")
	command := contract.Commands[0]

	if command.OutputSchema.Properties["schema_version"].Enum[0] != "blcli.runs/v1" {
		t.Fatalf("runs schema_version enum = %v, want blcli.runs/v1", command.OutputSchema.Properties["schema_version"].Enum)
	}
	runsSchema := command.OutputSchema.Properties["runs"]
	if runsSchema.Type != "array" || runsSchema.Items == nil {
		t.Fatalf("runs schema = %#v, want array with items", runsSchema)
	}
	if !contains(runsSchema.Items.Required, "operation_id") {
		t.Fatalf("run item required = %v, want operation_id", runsSchema.Items.Required)
	}
	moduleSchema := runsSchema.Items.Properties["modules"].Properties["<module_name>"]
	stepSchema := moduleSchema.Properties["steps"].Items
	if stepSchema == nil || stepSchema.Properties["command"].Type != "string" {
		t.Fatalf("runs step schema missing command: %#v", stepSchema)
	}
	if stepSchema.Properties["output_excerpt"].Type != "string" {
		t.Fatalf("runs step schema missing output_excerpt: %#v", stepSchema)
	}
	if stepSchema.Properties["error_location"].Type != "string" {
		t.Fatalf("runs step schema missing error_location: %#v", stepSchema)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
