package terraform

import (
	"testing"

	"blcli/pkg/renderer"
)

func TestExtractProjectComponentArgs_aliases(t *testing.T) {
	projectArgs := renderer.ArgsData{
		renderer.FieldComponents: map[string]interface{}{
			"firewall": map[string]interface{}{
				"rule-direction": "EGRESS",
				"priority":       900,
				"nat_name":       "egress-nat",
			},
		},
	}

	got := ExtractProjectComponentArgs(projectArgs, "firewall")
	for key, want := range map[string]interface{}{
		"rule-direction": "EGRESS",
		"RuleDirection":  "EGRESS",
		"ruleDirection":  "EGRESS",
		"priority":       900,
		"Priority":       900,
		"nat_name":       "egress-nat",
		"natName":        "egress-nat",
		"NatName":        "egress-nat",
	} {
		if got[key] != want {
			t.Errorf("ExtractProjectComponentArgs()[%q] = %v, want %v", key, got[key], want)
		}
	}
}

func TestExtractInitItemArgsForComponent_aliases(t *testing.T) {
	templateArgs := renderer.ArgsData{
		renderer.FieldTerraform: map[string]interface{}{
			renderer.FieldGlobal: map[string]interface{}{"BillingAccountID": "000000-000000-000000"},
			renderer.FieldInit: map[string]interface{}{
				renderer.FieldComponents: map[string]interface{}{
					"atlantis": map[string]interface{}{"atlantis-name": "atlantis"},
				},
			},
		},
	}

	got := ExtractInitItemArgsForComponent(templateArgs, nil, "atlantis")
	if got["AtlantisName"] != "atlantis" || got["atlantisName"] != "atlantis" || got["atlantis-name"] != "atlantis" {
		t.Errorf("init component arg not readable under every spelling: %v", got)
	}
	if got["BillingAccountID"] != "000000-000000-000000" {
		t.Errorf("terraform.global not flattened: %v", got["BillingAccountID"])
	}
}
