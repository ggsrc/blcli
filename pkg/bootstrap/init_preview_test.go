package bootstrap

import (
	"os"
	"testing"
	"time"
)

func TestPrintInitPreview(t *testing.T) {
	tmp := t.TempDir()
	argsPath := tmp + "/args.yaml"
	content := `global:
  workspace: workspace/output
terraform:
  projects:
    - name: app-prd
`
	if err := os.WriteFile(argsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := PrintInitPreview(InitOptions{
		TemplateRepo: "./bl-template",
		ArgsPaths:    []string{argsPath},
		CacheExpiry:  time.Hour,
	})
	if err != nil {
		t.Fatalf("PrintInitPreview: %v", err)
	}
}
