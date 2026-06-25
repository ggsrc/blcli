package cli

import (
	"testing"

	"blcli/pkg/renderer"
)

func TestApplyWizardOverrides(t *testing.T) {
	cfg := &ArgsConfig{
		Global: make(map[string]interface{}),
		Terraform: TerraformSection{
			Global: make(map[string]interface{}),
		},
	}
	applyWizardOverrides(cfg, wizardAnswers{
		Workspace:        "workspace/output",
		Domain:           "example.com",
		OrganizationID:   "123456789012",
		BillingAccountID: "01ABCD-2EFGH3-4IJKL5",
	})

	if cfg.Global["workspace"] != "workspace/output" {
		t.Fatalf("workspace = %v", cfg.Global["workspace"])
	}
	if cfg.Terraform.Global["OrganizationID"] != "123456789012" {
		t.Fatalf("OrganizationID = %v", cfg.Terraform.Global["OrganizationID"])
	}
}

func TestPrintArgsConfigPreview(t *testing.T) {
	cfg := &ArgsConfig{
		Global: map[string]interface{}{"workspace": "workspace/output"},
		Terraform: TerraformSection{
			Projects: []ProjectData{{Name: "app-prd", Global: map[string]interface{}{}}},
		},
	}
	// smoke test — should not panic
	printArgsConfigPreview(cfg, "args.yaml")
	_ = renderer.FieldGlobal
}
