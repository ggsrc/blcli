package bootstrap

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitHubOwner(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"github.com/someone", "someone"},
		{"someone", "someone"},
		{"  github.com/org  ", "org"},
		{"  org  ", "org"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		got := parseGitHubOwner(tt.in)
		if got != tt.want {
			t.Errorf("parseGitHubOwner(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestConfirmY(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"Y", "Y\n", true},
		{"y", "y\n", true},
		{"yes", "yes\n", true},
		{"N", "N\n", false},
		{"n", "n\n", false},
		{"other", "x\n", false},
		{"empty line", "\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.in))
			got := confirmY(r)
			if got != tt.want {
				t.Errorf("confirmY(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestConfirmY_TwoReadsFromSameReader checks that two confirmY calls on a shared
// bufio.Reader each read their own line.
func TestConfirmY_TwoReadsFromSameReader(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("Y\nY\n"))
	if !confirmY(r) {
		t.Fatal("first confirmY wanted true")
	}
	if !confirmY(r) {
		t.Fatal("second confirmY wanted true")
	}
}

func TestExecuteInitRepos_WithFake(t *testing.T) {
	root := t.TempDir()
	tfDir := filepath.Join(root, "terraform")
	k8sDir := filepath.Join(root, "kubernetes")
	gitopsDir := filepath.Join(root, "gitops")
	for _, d := range []string{tfDir, k8sDir, gitopsDir} {
		if err := mkdirAll(d); err != nil {
			t.Fatal(err)
		}
	}

	fake := &FakeInitReposRunner{}
	// 2 prompts per directory, 3 directories => 6 answers, one per line.
	stdin := strings.NewReader("Y\nY\nY\nY\nY\nY\n")

	err := ExecuteInitRepos(InitReposOptions{
		WorkspaceDir:  root,
		TerraformDir:  tfDir,
		KubernetesDir: k8sDir,
		GitOpsDir:     gitopsDir,
		GitHubOrg:     "github.com/myorg",
		Runner:        fake,
		Stdin:         stdin,
	})
	if err != nil {
		t.Fatalf("ExecuteInitRepos: %v", err)
	}

	if n := len(fake.GitInits); n != 3 {
		t.Errorf("GitInits calls = %d, want 3", n)
	}
	if n := len(fake.GhRepoCreates); n != 3 {
		t.Errorf("GhRepoCreates calls = %d, want 3", n)
	}
	if n := len(fake.GitAddCommitPushs); n != 3 {
		t.Errorf("GitAddCommitPushs calls = %d, want 3", n)
	}
	wantRepos := []string{"myorg/terraform", "myorg/kubernetes", "myorg/gitops"}
	for i, r := range wantRepos {
		if i >= len(fake.GhRepoCreates) {
			break
		}
		if g := fake.GhRepoCreates[i].Repo; g != r {
			t.Errorf("GhRepoCreates[%d].Repo = %q, want %q", i, g, r)
		}
	}
}

func TestExecuteInitRepos_InvalidOrg(t *testing.T) {
	err := ExecuteInitRepos(InitReposOptions{
		GitHubOrg: "",
	})
	if err == nil {
		t.Fatal("expected error for empty org")
	}
	if !strings.Contains(err.Error(), "invalid GitHub org") {
		t.Errorf("error = %v, want contain 'invalid GitHub org'", err)
	}
}

func TestExecuteInitRepos_UserCancelCreateRepo(t *testing.T) {
	root := t.TempDir()
	tfDir := filepath.Join(root, "terraform")
	if err := mkdirAll(tfDir); err != nil {
		t.Fatal(err)
	}

	fake := &FakeInitReposRunner{}
	stdin := strings.NewReader("N\n") // answer N to the first prompt

	err := ExecuteInitRepos(InitReposOptions{
		WorkspaceDir: root,
		TerraformDir: tfDir,
		GitHubOrg:    "someone",
		Runner:       fake,
		Stdin:        stdin,
	})
	if err == nil {
		t.Fatal("expected error when user answers N")
	}
	if !strings.Contains(err.Error(), "canceled by user") {
		t.Errorf("error = %v, want it to contain 'canceled by user'", err)
	}
	// Only GitInit ran; GhRepoCreate did not.
	if len(fake.GitInits) != 1 || len(fake.GhRepoCreates) != 0 {
		t.Errorf("GitInits=%d GhRepoCreates=%d, want 1,0", len(fake.GitInits), len(fake.GhRepoCreates))
	}
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

const testPrivateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\n-----END OPENSSH PRIVATE KEY-----\n"

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newInitReposWorkspace(t *testing.T) (root, tfDir, k8sDir, gitopsDir string) {
	t.Helper()
	root = t.TempDir()
	tfDir = filepath.Join(root, "terraform")
	k8sDir = filepath.Join(root, "kubernetes")
	gitopsDir = filepath.Join(root, "gitops")
	for _, d := range []string{tfDir, k8sDir, gitopsDir} {
		if err := mkdirAll(d); err != nil {
			t.Fatal(err)
		}
	}
	return root, tfDir, k8sDir, gitopsDir
}

func TestExecuteInitRepos_RefusesPrivateKeyMaterial(t *testing.T) {
	tests := []struct {
		name     string
		dir      string // terraform, kubernetes or gitops
		file     string
		content  string
		wantPath string
	}{
		{name: "legacy ArgoCD key in kubernetes/keys", dir: "kubernetes", file: "keys/argocd_github", content: testPrivateKey, wantPath: "keys/argocd_github"},
		{name: "keys dir is reported", dir: "kubernetes", file: "keys/argocd_github", content: testPrivateKey, wantPath: "keys" + string(filepath.Separator)},
		{name: "key written by an install script", dir: "kubernetes", file: "prd/2-argocd/keys/argocd_github", content: testPrivateKey, wantPath: "prd/2-argocd/keys/argocd_github"},
		{name: "private key in any file", dir: "gitops", file: "stg/app/secret.yaml", content: "stringData:\n  key: |\n    " + testPrivateKey, wantPath: "stg/app/secret.yaml"},
		{name: "RSA key", dir: "terraform", file: "notes.txt", content: "-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----\n", wantPath: "notes.txt"},
		{name: "id_* file name", dir: "terraform", file: "id_ed25519", content: "not a pem", wantPath: "id_ed25519"},
		{name: "pem file name", dir: "terraform", file: "certs/server.pem", content: "x", wantPath: "certs/server.pem"},
		{name: "key file name", dir: "gitops", file: "tls.key", content: "x", wantPath: "tls.key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, tfDir, k8sDir, gitopsDir := newInitReposWorkspace(t)
			base := map[string]string{"terraform": tfDir, "kubernetes": k8sDir, "gitops": gitopsDir}[tt.dir]
			writeTestFile(t, filepath.Join(base, filepath.FromSlash(tt.file)), tt.content)

			fake := &FakeInitReposRunner{}
			err := ExecuteInitRepos(InitReposOptions{
				WorkspaceDir:  root,
				TerraformDir:  tfDir,
				KubernetesDir: k8sDir,
				GitOpsDir:     gitopsDir,
				GitHubOrg:     "myorg",
				Runner:        fake,
				Stdin:         strings.NewReader("Y\nY\nY\nY\nY\nY\n"),
			})
			if err == nil {
				t.Fatal("expected ExecuteInitRepos to refuse private-key material")
			}
			msg := err.Error()
			for _, want := range []string{"refusing to commit", base, filepath.FromSlash(tt.wantPath), filepath.Join(root, ".blcli-keys"), "rotate"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not mention %q", msg, want)
				}
			}
			if len(fake.GitInits) != 0 || len(fake.GhRepoCreates) != 0 || len(fake.GitAddCommitPushs) != 0 {
				t.Errorf("runner was called (init=%d create=%d push=%d); the check must run before any git or gh command",
					len(fake.GitInits), len(fake.GhRepoCreates), len(fake.GitAddCommitPushs))
			}
		})
	}
}

func TestExecuteInitRepos_AllowsPublicKeysAndSkipsGitDir(t *testing.T) {
	root, tfDir, k8sDir, gitopsDir := newInitReposWorkspace(t)
	writeTestFile(t, filepath.Join(k8sDir, "keys", "argocd_github.pub"), "ssh-ed25519 AAAA argocd-github\n")
	writeTestFile(t, filepath.Join(k8sDir, ".git", "objects", "id_rsa"), testPrivateKey)
	writeTestFile(t, filepath.Join(k8sDir, "prd", "2-argocd", "github-ssh-sealed-secret.yaml"), "kind: SealedSecret\nencryptedData:\n  sshPrivateKey: AgB4\n")
	writeTestFile(t, filepath.Join(root, ".blcli-keys", "argocd_github"), testPrivateKey)

	fake := &FakeInitReposRunner{}
	err := ExecuteInitRepos(InitReposOptions{
		WorkspaceDir:  root,
		TerraformDir:  tfDir,
		KubernetesDir: k8sDir,
		GitOpsDir:     gitopsDir,
		GitHubOrg:     "myorg",
		Runner:        fake,
		Stdin:         strings.NewReader("Y\nY\nY\nY\nY\nY\n"),
	})
	if err != nil {
		t.Fatalf("ExecuteInitRepos: %v", err)
	}
	if n := len(fake.GitAddCommitPushs); n != 3 {
		t.Errorf("GitAddCommitPushs calls = %d, want 3", n)
	}
}

func TestExecuteInitRepos_WritesKubernetesGitignore(t *testing.T) {
	root, tfDir, k8sDir, gitopsDir := newInitReposWorkspace(t)
	writeTestFile(t, filepath.Join(k8sDir, ".gitignore"), "*.tmp")

	fake := &FakeInitReposRunner{}
	if err := ExecuteInitRepos(InitReposOptions{
		WorkspaceDir:  root,
		TerraformDir:  tfDir,
		KubernetesDir: k8sDir,
		GitOpsDir:     gitopsDir,
		GitHubOrg:     "myorg",
		Runner:        fake,
		Stdin:         strings.NewReader("Y\nY\nY\nY\nY\nY\n"),
	}); err != nil {
		t.Fatalf("ExecuteInitRepos: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(k8sDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "*.tmp\nkeys/\n" {
		t.Errorf("kubernetes .gitignore = %q, want existing entries kept and keys/ appended", got)
	}
	if _, err := os.Stat(filepath.Join(tfDir, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("terraform .gitignore should not be written, stat err = %v", err)
	}

	// Idempotent: a second run does not add keys/ again.
	if err := ensureGitignoreEntry(k8sDir, "keys/"); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(filepath.Join(k8sDir, ".gitignore"))
	if string(again) != string(got) {
		t.Errorf(".gitignore changed on second run: %q", again)
	}
}
