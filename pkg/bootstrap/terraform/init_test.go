package terraform

import (
	"testing"

	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

func TestBuildInitPlan_splitsByPrepare(t *testing.T) {
	terraformConfig := &template.TerraformConfig{
		Init: []template.InitItem{
			{
				Name:        "backend",
				Prepare:     true,
				Destination: "terraform/init/0-terraform-statestore/",
				Path:        []string{"terraform/init/main.tf.tmpl"},
			},
			{
				Name:        "projects",
				Prepare:     false,
				Destination: "terraform/init/1-{{.GlobalName}}-projects/",
				Path:        []string{"terraform/init/main.tf.tmpl"},
			},
			{
				Name:        "atlantis",
				Prepare:     false,
				Destination: "terraform/init/2-atlantis/",
				Path:        []string{"terraform/init/main.tf.tmpl"},
			},
		},
	}
	templateArgs := renderer.ArgsData{
		renderer.FieldTerraform: map[string]interface{}{
			renderer.FieldInit: map[string]interface{}{
				renderer.FieldComponents: map[string]interface{}{
					"backend":  map[string]interface{}{},
					"projects": map[string]interface{}{},
					"atlantis": map[string]interface{}{},
				},
			},
		},
	}
	data := map[string]interface{}{
		"GlobalName": "my-org",
	}

	prepareDirs, initDirs, err := BuildInitPlan(terraformConfig, templateArgs, data)
	if err != nil {
		t.Fatalf("BuildInitPlan: %v", err)
	}
	if len(prepareDirs) != 1 || prepareDirs[0] != "init/0-terraform-statestore" {
		t.Errorf("prepareDirs = %v, want [init/0-terraform-statestore]", prepareDirs)
	}
	if len(initDirs) != 2 {
		t.Errorf("initDirs len = %d, want 2", len(initDirs))
	}
	if initDirs[0] != "init/1-my-org-projects" || initDirs[1] != "init/2-atlantis" {
		t.Errorf("initDirs = %v, want [init/1-my-org-projects init/2-atlantis]", initDirs)
	}
}

func TestBuildInitPlan_skipsUnavailableComponents(t *testing.T) {
	terraformConfig := &template.TerraformConfig{
		Init: []template.InitItem{
			{Name: "backend", Prepare: true, Destination: "terraform/init/0-backend/", Path: []string{"x.tmpl"}},
			{Name: "other", Prepare: false, Destination: "terraform/init/1-other/", Path: []string{"x.tmpl"}},
		},
	}
	// Only "backend" in components; "other" is skipped
	templateArgs := renderer.ArgsData{
		renderer.FieldTerraform: map[string]interface{}{
			renderer.FieldInit: map[string]interface{}{
				renderer.FieldComponents: map[string]interface{}{
					"backend": map[string]interface{}{},
				},
			},
		},
	}
	data := map[string]interface{}{}

	prepareDirs, initDirs, err := BuildInitPlan(terraformConfig, templateArgs, data)
	if err != nil {
		t.Fatalf("BuildInitPlan: %v", err)
	}
	if len(prepareDirs) != 1 || prepareDirs[0] != "init/0-backend" {
		t.Errorf("prepareDirs = %v", prepareDirs)
	}
	if len(initDirs) != 0 {
		t.Errorf("initDirs = %v, want []", initDirs)
	}
}
