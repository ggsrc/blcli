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

// TestConfirmY_TwoReadsFromSameReader 使用共享的 bufio.Reader 时，连续两次 confirmY 都能读到下一行。
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
	// 每个目录 2 个提示，共 3 个目录 => 6 个 Y（显式多行，避免 Reader 边界问题）
	stdin := strings.NewReader("Y\nY\nY\nY\nY\nY\n")

	err := ExecuteInitRepos(InitReposOptions{
		WorkspaceDir:   root,
		TerraformDir:   tfDir,
		KubernetesDir:  k8sDir,
		GitOpsDir:      gitopsDir,
		GitHubOrg:      "github.com/myorg",
		Runner:         fake,
		Stdin:          stdin,
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
	stdin := strings.NewReader("N\n") // 第一个提示答 N

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
	if !strings.Contains(err.Error(), "用户取消") {
		t.Errorf("error = %v, want contain '用户取消'", err)
	}
	// 应只调用了 GitInit，未调用 GhRepoCreate
	if len(fake.GitInits) != 1 || len(fake.GhRepoCreates) != 0 {
		t.Errorf("GitInits=%d GhRepoCreates=%d, want 1,0", len(fake.GitInits), len(fake.GhRepoCreates))
	}
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}
