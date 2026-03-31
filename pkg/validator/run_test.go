package validator

import (
	"os"
	"path/filepath"
	"testing"

	"blcli/pkg/renderer"
)

func TestRun_ProjectName_InvalidLength(t *testing.T) {
	// ProjectName "ab" fails stringLength min 6 and pattern
	data := renderer.ArgsData{
		"global": map[string]interface{}{"GlobalName": "my-org"},
		"terraform": map[string]interface{}{
			"global": map[string]interface{}{
				"BillingAccountID": "01ABCD-2EFGH3-4IJKL5",
				"OrganizationID":   "123456789012",
			},
			"projects": []interface{}{
				map[string]interface{}{
					"name": "short",
					"global": map[string]interface{}{
						"ProjectName": "ab",
					},
				},
			},
		},
	}
	loader := NewTemplateLoader(func(path string) (string, error) {
		if path == "terraform/project/args.yaml" {
			return `version: 1.0.0
parameters:
  global:
    ProjectName:
      type: string
      required: true
      validation:
        - kind: required
        - kind: stringLength
          min: 6
          max: 30
        - kind: pattern
          value: "^[a-z][a-z0-9-]{4,28}[a-z0-9]$"
`, nil
		}
		return "", nil
	})
	err := Run(data, loader)
	if err == nil {
		t.Fatal("expected validation error for ProjectName \"ab\", got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestRun_ProjectName_Valid(t *testing.T) {
	data := renderer.ArgsData{
		"global": map[string]interface{}{"GlobalName": "my-org"},
		"terraform": map[string]interface{}{
			"projects": []interface{}{
				map[string]interface{}{
					"name": "myproject",
					"global": map[string]interface{}{
						"ProjectName": "my-project",
					},
				},
			},
		},
	}
	loader := NewTemplateLoader(func(path string) (string, error) {
		if path == "terraform/project/args.yaml" {
			return `version: 1.0.0
parameters:
  global:
    ProjectName:
      type: string
      required: true
      validation:
        - kind: stringLength
          min: 6
          max: 30
`, nil
		}
		return "", nil
	})
	err := Run(data, loader)
	if err != nil {
		t.Fatalf("expected no error for valid ProjectName, got: %v", err)
	}
}

// TestRun_WithYAMLUnmarshaledStructure ensures that when data comes from YAML (possibly
// map[interface{}]interface{} in nested values), project validation still runs.
func TestRun_WithYAMLUnmarshaledStructure(t *testing.T) {
	// Simulate what we get from yaml.Unmarshal: nested maps can be map[interface{}]interface{}
	data := renderer.ArgsData{
		"global": map[string]interface{}{"GlobalName": "my-org"},
		"terraform": map[interface{}]interface{}{
			"version": "1.0.0",
			"global": map[interface{}]interface{}{
				"BillingAccountID": "01ABCD-2EFGH3-4IJKL5",
				"OrganizationID":   "123456789012",
			},
			"projects": []interface{}{
				map[interface{}]interface{}{
					"name": "short",
					"global": map[interface{}]interface{}{
						"ProjectName": "ab",
					},
				},
			},
		},
	}
	loader := NewTemplateLoader(func(path string) (string, error) {
		if path == "terraform/project/args.yaml" {
			return `version: 1.0.0
parameters:
  global:
    ProjectName:
      type: string
      required: true
      validation:
        - kind: stringLength
          min: 6
          max: 30
`, nil
		}
		return "", nil
	})
	err := Run(data, loader)
	if err == nil {
		t.Fatal("expected validation error when ProjectName is \"ab\" (YAML-like nested maps), got nil")
	}
	t.Logf("got expected error: %v", err)
}

// TestRun_RealFileLoad loads real YAML and template to mimic production; skipped if files missing.
func TestRun_RealFileLoad(t *testing.T) {
	// From blcli-go/pkg/validator: repo root (blcli) is ../../..
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	argsPath := filepath.Join(repoRoot, "workspace", "validation-test-invalid.yaml")
	templatePath := filepath.Join(repoRoot, "bl-template")
	if _, err := os.Stat(argsPath); os.IsNotExist(err) {
		t.Skipf("args file not found: %s", argsPath)
	}
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skipf("template not found: %s", templatePath)
	}
	data, err := renderer.LoadArgs(argsPath)
	if err != nil {
		t.Fatalf("load args: %v", err)
	}
	loader, err := newLocalTemplateLoader(templatePath)
	if err != nil {
		t.Fatalf("loader: %v", err)
	}
	err = Run(data, loader)
	if err == nil {
		t.Fatal("expected validation error for ProjectName \"ab\", got nil")
	}
	t.Logf("got expected error: %v", err)
}

// newLocalTemplateLoader creates a TemplateLoader that reads from a local directory.
func newLocalTemplateLoader(root string) (TemplateLoader, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return NewTemplateLoader(func(path string) (string, error) {
		full := filepath.Join(abs, path)
		b, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}), nil
}
