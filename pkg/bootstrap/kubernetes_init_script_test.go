package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blcli/pkg/config"
	"blcli/pkg/renderer"
	"blcli/pkg/template"
)

func TestBootstrapKubernetesRunsInitScript(t *testing.T) {
	t.Setenv(KeysDirEnv, "")
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

	markerPath := filepath.Join(workspaceDir, ".blcli-keys", "marker.txt")
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if string(content) != "ok" {
		t.Fatalf("unexpected marker content: %q", string(content))
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "kubernetes", "keys")); !os.IsNotExist(err) {
		t.Fatalf("keys must not be created inside the kubernetes dir, stat err = %v", err)
	}
	info, err := os.Stat(filepath.Join(workspaceDir, ".blcli-keys"))
	if err != nil {
		t.Fatalf("stat keys dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("keys dir mode = %o, want 700", perm)
	}
	ignore, err := os.ReadFile(filepath.Join(workspaceDir, ".blcli-keys", ".gitignore"))
	if err != nil || !strings.Contains(string(ignore), "\n*\n") {
		t.Fatalf("keys dir .gitignore = %q, %v; want it to ignore everything", ignore, err)
	}
	kubeIgnore, err := os.ReadFile(filepath.Join(workspaceDir, "kubernetes", ".gitignore"))
	if err != nil || !strings.Contains(string(kubeIgnore), "\nkeys/\n") {
		t.Fatalf("kubernetes .gitignore = %q, %v; want a keys/ entry", kubeIgnore, err)
	}
}

func TestBootstrapKubernetesRunsInitScriptWithRelativeWorkspace(t *testing.T) {
	t.Setenv(KeysDirEnv, "")
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

	markerPath := filepath.Join(tmpDir, "workspace", ".blcli-keys", "marker.txt")
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if string(content) != "ok" {
		t.Fatalf("unexpected marker content: %q", string(content))
	}
}

// writeKeysTemplate creates a template repo whose kubernetes/init.sh writes a marker into $BLCLI_KEYS_DIR.
func writeKeysTemplate(t *testing.T, templateDir string) *template.Loader {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(templateDir, "kubernetes"), 0o755); err != nil {
		t.Fatalf("mkdir template dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "kubernetes", "config.yaml"), []byte("components: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	script := "#!/bin/bash\nset -e\nmkdir -p \"$BLCLI_KEYS_DIR\"\nprintf 'ok' > \"$BLCLI_KEYS_DIR/marker.txt\"\n"
	if err := os.WriteFile(filepath.Join(templateDir, "kubernetes", "init.sh"), []byte(script), 0o644); err != nil {
		t.Fatalf("write init script: %v", err)
	}
	return template.NewLoader(templateDir)
}

func TestBootstrapKubernetesHonorsKeysDirEnv(t *testing.T) {
	tmpDir := t.TempDir()
	keysDir := filepath.Join(tmpDir, "secure-keys")
	t.Setenv(KeysDirEnv, keysDir)

	loader := writeKeysTemplate(t, filepath.Join(tmpDir, "template"))
	global := config.GlobalConfig{Workspace: filepath.Join(tmpDir, "workspace")}
	if err := BootstrapKubernetes(global, &config.ProjectConfig{}, loader, renderer.ArgsData{}, false); err != nil {
		t.Fatalf("BootstrapKubernetes returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(keysDir, "marker.txt")); err != nil {
		t.Fatalf("marker not written to %s: %v", keysDir, err)
	}
}

func TestBootstrapKubernetesMovesLegacyKeysOutOfKubernetesDir(t *testing.T) {
	t.Setenv(KeysDirEnv, "")
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	legacyDir := filepath.Join(workspaceDir, "kubernetes", "keys")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	privateKey := "-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n-----END OPENSSH PRIVATE KEY-----\n"
	if err := os.WriteFile(filepath.Join(legacyDir, "argocd_github"), []byte(privateKey), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "argocd_github.pub"), []byte("ssh-ed25519 AAAA argocd-github\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := writeKeysTemplate(t, filepath.Join(tmpDir, "template"))
	global := config.GlobalConfig{Workspace: workspaceDir}
	if err := BootstrapKubernetes(global, &config.ProjectConfig{}, loader, renderer.ArgsData{}, false); err != nil {
		t.Fatalf("BootstrapKubernetes returned error: %v", err)
	}

	moved, err := os.ReadFile(filepath.Join(workspaceDir, ".blcli-keys", "argocd_github"))
	if err != nil || string(moved) != privateKey {
		t.Fatalf("legacy private key not moved: %q, %v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, ".blcli-keys", "argocd_github.pub")); err != nil {
		t.Fatalf("legacy public key not moved: %v", err)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("legacy keys dir still exists: %v", err)
	}
}
