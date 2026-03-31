package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"blcli/pkg/config"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

func TestBootstrapKubernetesRunsInitScript(t *testing.T) {
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "template")
	workspaceDir := filepath.Join(tmpDir, "workspace")

	if err := os.MkdirAll(filepath.Join(templateDir, "kubernetes"), 0o755); err != nil {
		t.Fatalf("mkdir template dir: %v", err)
	}

	configContent := "components: []\n"
	if err := os.WriteFile(filepath.Join(templateDir, "kubernetes", "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	scriptContent := "#!/bin/bash\nset -e\nmkdir -p \"$BLCLI_KEYS_DIR\"\nprintf 'ok' > \"$BLCLI_KEYS_DIR/marker.txt\"\n"
	if err := os.WriteFile(filepath.Join(templateDir, "kubernetes", "init.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("write init script: %v", err)
	}

	loader := template.NewLoader(templateDir)
	args := renderer.ArgsData{}
	global := config.GlobalConfig{Workspace: workspaceDir}

	if err := BootstrapKubernetes(global, &config.ProjectConfig{}, loader, args, false); err != nil {
		t.Fatalf("BootstrapKubernetes returned error: %v", err)
	}

	markerPath := filepath.Join(workspaceDir, "kubernetes", "keys", "marker.txt")
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if string(content) != "ok" {
		t.Fatalf("unexpected marker content: %q", string(content))
	}
}

func TestBootstrapKubernetesRunsInitScriptWithRelativeWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tmpdir: %v", err)
	}

	templateDir := filepath.Join(tmpDir, "template")
	workspaceDir := filepath.Join("workspace")

	if err := os.MkdirAll(filepath.Join(templateDir, "kubernetes"), 0o755); err != nil {
		t.Fatalf("mkdir template dir: %v", err)
	}

	configContent := "components: []\n"
	if err := os.WriteFile(filepath.Join(templateDir, "kubernetes", "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	scriptContent := "#!/bin/bash\nset -e\nmkdir -p \"$BLCLI_KEYS_DIR\"\nprintf 'ok' > \"$BLCLI_KEYS_DIR/marker.txt\"\n"
	if err := os.WriteFile(filepath.Join(templateDir, "kubernetes", "init.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("write init script: %v", err)
	}

	loader := template.NewLoader(templateDir)
	args := renderer.ArgsData{}
	global := config.GlobalConfig{Workspace: workspaceDir}

	if err := BootstrapKubernetes(global, &config.ProjectConfig{}, loader, args, false); err != nil {
		t.Fatalf("BootstrapKubernetes returned error: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "workspace", "kubernetes", "keys", "marker.txt")
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if string(content) != "ok" {
		t.Fatalf("unexpected marker content: %q", string(content))
	}
}
