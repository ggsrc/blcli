package terraform

import (
	"strings"
	"testing"

	"blcli/pkg/renderer"
)

func TestParseProjectComponentNode(t *testing.T) {
	tests := []struct {
		s      string
		wantOK bool
		project string
		comp   string
	}{
		{"prd/vpc", true, "prd", "vpc"},
		{"corp/vpc-peering", true, "corp", "vpc-peering"},
		{"a/b", true, "a", "b"},
		{"", false, "", ""},
		{"nodelim", false, "", ""},
		{"/only", false, "", ""},
		{"only/", false, "", ""},
		{"/", false, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got, ok := ParseProjectComponentNode(tt.s)
			if ok != tt.wantOK {
				t.Errorf("ParseProjectComponentNode(%q) ok = %v, want %v", tt.s, ok, tt.wantOK)
				return
			}
			if tt.wantOK && (got.Project != tt.project || got.Component != tt.comp) {
				t.Errorf("ParseProjectComponentNode(%q) = %+v, want project=%q component=%q", tt.s, got, tt.project, tt.comp)
			}
		})
	}
}

func TestProjectComponentNode_String(t *testing.T) {
	n := ProjectComponentNode{Project: "corp", Component: "vpc-peering"}
	if got := n.String(); got != "corp/vpc-peering" {
		t.Errorf("String() = %q, want corp/vpc-peering", got)
	}
}

func TestIsComponentInSubdir(t *testing.T) {
	subdir := map[string][]string{
		"prd":  {"vpc-peering"},
		"stg":  {"vpc-peering"},
		"corp": {"vpc-peering"},
	}
	tests := []struct {
		project, comp string
		want          bool
	}{
		{"prd", "vpc-peering", true},
		{"prd", "vpc", false},
		{"corp", "vpc-peering", true},
		{"other", "vpc-peering", false},
	}
	for _, tt := range tests {
		got := IsComponentInSubdir(subdir, tt.project, tt.comp)
		if got != tt.want {
			t.Errorf("IsComponentInSubdir(%q, %q) = %v, want %v", tt.project, tt.comp, got, tt.want)
		}
	}
	if got := IsComponentInSubdir(nil, "prd", "vpc-peering"); got != false {
		t.Errorf("IsComponentInSubdir(nil, ...) = %v, want false", got)
	}
}

func TestBuildProjectDependencyPlan_empty(t *testing.T) {
	order, subdir, _, err := BuildProjectDependencyPlan(renderer.ArgsData{}, []string{"prd"})
	if err != nil {
		t.Fatalf("BuildProjectDependencyPlan: %v", err)
	}
	if len(order) != 0 {
		t.Errorf("dependencyOrder = %v, want empty", order)
	}
	if len(subdir) != 0 {
		t.Errorf("subdirComponents = %v, want empty", subdir)
	}
}

func TestBuildProjectDependencyPlan_noTerraform(t *testing.T) {
	args := renderer.ArgsData{"other": "x"}
	order, subdir, _, err := BuildProjectDependencyPlan(args, []string{"prd"})
	if err != nil {
		t.Fatalf("BuildProjectDependencyPlan: %v", err)
	}
	if len(order) != 0 || len(subdir) != 0 {
		t.Errorf("dependencyOrder=%v subdir=%v, want both empty", order, subdir)
	}
}

func TestBuildProjectDependencyPlan_withCrossProjectDeps(t *testing.T) {
	// prd has vpc and vpc-peering (vpc-peering depends on corp/vpc); corp has vpc and vpc-peering (depends on prd/vpc, stg/vpc)
	args := renderer.ArgsData{
		renderer.FieldTerraform: map[string]interface{}{
			renderer.FieldProjects: []interface{}{
				map[string]interface{}{
					renderer.FieldName: "prd",
					renderer.FieldComponents: []interface{}{
						map[string]interface{}{renderer.FieldName: "vpc", "parameters": map[string]interface{}{}},
						map[string]interface{}{
							renderer.FieldName: "vpc-peering",
							"parameters":       map[string]interface{}{},
							"depends_on": []interface{}{
								map[string]interface{}{"project": "corp", "component": "vpc"},
							},
						},
					},
				},
				map[string]interface{}{
					renderer.FieldName: "corp",
					renderer.FieldComponents: []interface{}{
						map[string]interface{}{renderer.FieldName: "vpc", "parameters": map[string]interface{}{}},
						map[string]interface{}{
							renderer.FieldName: "vpc-peering",
							"parameters":       map[string]interface{}{},
							"depends_on": []interface{}{
								map[string]interface{}{"project": "prd", "component": "vpc"},
								map[string]interface{}{"project": "stg", "component": "vpc"},
							},
						},
					},
				},
				map[string]interface{}{
					renderer.FieldName: "stg",
					renderer.FieldComponents: []interface{}{
						map[string]interface{}{renderer.FieldName: "vpc", "parameters": map[string]interface{}{}},
					},
				},
			},
		},
	}
	order, subdir, layers, err := BuildProjectDependencyPlan(args, []string{"prd", "stg", "corp"})
	if err != nil {
		t.Fatalf("BuildProjectDependencyPlan: %v", err)
	}
	// vpc nodes have no deps; vpc-peering nodes depend on other projects' vpc. So order should have vpc before vpc-peering where applicable.
	if len(order) < 4 {
		t.Errorf("dependencyOrder = %v, want at least 4 items", order)
	}
	// prd/vpc-peering and corp/vpc-peering have cross-project deps -> subdir
	if len(subdir["prd"]) != 1 || subdir["prd"][0] != "vpc-peering" {
		t.Errorf("subdir[prd] = %v, want [vpc-peering]", subdir["prd"])
	}
	if len(subdir["corp"]) != 1 || subdir["corp"][0] != "vpc-peering" {
		t.Errorf("subdir[corp] = %v, want [vpc-peering]", subdir["corp"])
	}
	if _, ok := subdir["stg"]; ok && len(subdir["stg"]) > 0 {
		t.Errorf("subdir[stg] = %v, want empty or absent", subdir["stg"])
	}
	// Topological order: dependencies first. So corp/vpc, prd/vpc, stg/vpc should appear before any vpc-peering.
	orderSet := make(map[string]int)
	for i, s := range order {
		orderSet[s] = i
	}
	if iCorpVpc, ok := orderSet["corp/vpc"]; !ok {
		t.Error("corp/vpc missing from order")
	} else if iCorpPeering, ok := orderSet["corp/vpc-peering"]; ok && iCorpPeering <= iCorpVpc {
		t.Errorf("corp/vpc-peering should come after corp/vpc in order")
	}
	// Subdir components should have layer 1 (depend on layer 0 vpc)
	if layers["prd/vpc-peering"] != 1 || layers["corp/vpc-peering"] != 1 {
		t.Errorf("subdir component layers = %v, expect vpc-peering at layer 1", layers)
	}
}

func TestBuildProjectDependencyPlan_sameProjectDep_notSubdir(t *testing.T) {
	// Component that only depends on same-project component should NOT be in subdir
	args := renderer.ArgsData{
		renderer.FieldTerraform: map[string]interface{}{
			renderer.FieldProjects: []interface{}{
				map[string]interface{}{
					renderer.FieldName: "prd",
					renderer.FieldComponents: []interface{}{
						map[string]interface{}{renderer.FieldName: "vpc", "parameters": map[string]interface{}{}},
						map[string]interface{}{
							renderer.FieldName: "firewall",
							"parameters":       map[string]interface{}{},
							"depends_on": []interface{}{
								map[string]interface{}{"project": "prd", "component": "vpc"},
							},
						},
					},
				},
			},
		},
	}
	_, subdir, _, err := BuildProjectDependencyPlan(args, []string{"prd"})
	if err != nil {
		t.Fatalf("BuildProjectDependencyPlan: %v", err)
	}
	if len(subdir["prd"]) != 0 {
		t.Errorf("subdir[prd] = %v, want empty (same-project dep only)", subdir["prd"])
	}
}

func TestBuildProjectDependencyPlan_cycleReturnsError(t *testing.T) {
	// A depends on B, B depends on A -> cycle
	args := renderer.ArgsData{
		renderer.FieldTerraform: map[string]interface{}{
			renderer.FieldProjects: []interface{}{
				map[string]interface{}{
					renderer.FieldName: "prd",
					renderer.FieldComponents: []interface{}{
						map[string]interface{}{
							renderer.FieldName: "a",
							"parameters":       map[string]interface{}{},
							"depends_on": []interface{}{
								map[string]interface{}{"project": "prd", "component": "b"},
							},
						},
						map[string]interface{}{
							renderer.FieldName: "b",
							"parameters":       map[string]interface{}{},
							"depends_on": []interface{}{
								map[string]interface{}{"project": "prd", "component": "a"},
							},
						},
					},
				},
			},
		},
	}
	_, _, _, err := BuildProjectDependencyPlan(args, []string{"prd"})
	if err == nil {
		t.Fatal("BuildProjectDependencyPlan: expected error for cycle, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %v, want containing 'cycle'", err)
	}
}
