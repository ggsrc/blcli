package bootstrap

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InitReposRunner runs the git and gh commands; tests inject a fake.
type InitReposRunner interface {
	GitInit(dir string) error
	GhRepoCreate(dir, repo string) error
	GitAddCommitPush(dir string) error
}

// InitReposOptions holds options for apply init-repos command.
type InitReposOptions struct {
	WorkspaceDir       string // -d, root containing terraform/ kubernetes/ gitops/
	TerraformDir       string
	KubernetesDir      string
	GitOpsDir          string
	GitHubOrg          string // e.g. someone or github.com/someone
	RepoNameTerraform  string // optional override, default {org}/terraform
	RepoNameKubernetes string
	RepoNameGitOps     string
	Runner             InitReposRunner // optional; tests inject a fake
	Stdin              io.Reader       // optional source of prompt answers; default os.Stdin
}

// ExecuteInitRepos runs git init, prompts to create GitHub repo, and prompts to commit/push
// for each of terraform, kubernetes, gitops dirs that exist.
// User must type Y to continue at each prompt; N or any other input stops the flow.
// Tests can inject a fake through opts.Runner and answers through opts.Stdin.
func ExecuteInitRepos(opts InitReposOptions) error {
	owner := parseGitHubOwner(opts.GitHubOrg)
	if owner == "" {
		return fmt.Errorf("invalid GitHub org: %q (use e.g. someone or github.com/someone)", opts.GitHubOrg)
	}

	runner := opts.Runner
	if runner == nil {
		runner = defaultInitReposRunner{}
	}
	in := opts.Stdin
	if in == nil {
		in = os.Stdin
	}
	inBuf := bufio.NewReader(in)

	dirs := []struct {
		name string
		path string
		repo string // owner/name
	}{
		{"terraform", opts.TerraformDir, owner + "/terraform"},
		{"kubernetes", opts.KubernetesDir, owner + "/kubernetes"},
		{"gitops", opts.GitOpsDir, owner + "/gitops"},
	}
	if opts.RepoNameTerraform != "" {
		dirs[0].repo = opts.RepoNameTerraform
	}
	if opts.RepoNameKubernetes != "" {
		dirs[1].repo = opts.RepoNameKubernetes
	}
	if opts.RepoNameGitOps != "" {
		dirs[2].repo = opts.RepoNameGitOps
	}

	// Check every directory before touching git or GitHub: `git add .` would commit any
	// private key left inside it (older blcli versions generated the ArgoCD deploy key in
	// <kubernetes>/keys).
	if err := checkReposForKeyMaterial(opts, dirs[1].path, []string{dirs[0].path, dirs[1].path, dirs[2].path}); err != nil {
		return err
	}

	for _, d := range dirs {
		if d.path == "" {
			continue
		}
		abs, err := filepath.Abs(d.path)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			continue
		}

		fmt.Println()
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("📁 %s: %s\n", d.name, abs)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		if err := runner.GitInit(abs); err != nil {
			return fmt.Errorf("%s: git init: %w", d.name, err)
		}

		// Prompt: create repo
		fmt.Printf("Create the GitHub repository for %s at https://github.com/%s? [Y/n]: ", d.name, d.repo)
		if !confirmY(inBuf) {
			return fmt.Errorf("canceled by user: create repository")
		}
		if err := runner.GhRepoCreate(abs, d.repo); err != nil {
			return fmt.Errorf("%s: create GitHub repository: %w", d.name, err)
		}

		// Prompt: commit and push
		fmt.Printf("Commit and push to %s? [Y/n]: ", d.repo)
		if !confirmY(inBuf) {
			return fmt.Errorf("canceled by user: commit and push")
		}
		if err := runner.GitAddCommitPush(abs); err != nil {
			return fmt.Errorf("%s: commit and push: %w", d.name, err)
		}
		fmt.Printf("✅ %s done\n", d.name)
	}
	fmt.Println()
	return nil
}

// checkReposForKeyMaterial writes a .gitignore with keys/ into the kubernetes directory and
// returns an error listing every private-key file found in the directories to be committed.
func checkReposForKeyMaterial(opts InitReposOptions, kubernetesDir string, paths []string) error {
	keysWorkspace := opts.WorkspaceDir
	if keysWorkspace == "" && kubernetesDir != "" {
		keysWorkspace = filepath.Dir(kubernetesDir)
	}
	keysDir, _ := KeysDirForWorkspace(keysWorkspace)

	var errs []error
	for _, p := range paths {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			continue
		}
		if p == kubernetesDir {
			if err := ensureGitignoreEntry(abs, kubernetesGitignoreEntry); err != nil {
				return err
			}
		}
		findings, err := findPrivateKeyMaterial(abs)
		if err != nil {
			return fmt.Errorf("scan %s for private keys: %w", abs, err)
		}
		if len(findings) > 0 {
			errs = append(errs, &KeyMaterialError{Dir: abs, Findings: findings, KeysDir: keysDir})
		}
	}
	return errors.Join(errs...)
}

// parseGitHubOwner returns the owner from "github.com/someone" or "someone".
func parseGitHubOwner(org string) string {
	s := strings.TrimSpace(org)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "github.com/") {
		return strings.TrimPrefix(s, "github.com/")
	}
	return s
}

// confirmY reads one line from r; returns true only for Y/y, false otherwise.
// Pass the same *bufio.Reader to every call: a new reader would buffer ahead
// and later calls would miss the next line.
func confirmY(r *bufio.Reader) bool {
	line, err := r.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	line = strings.TrimSpace(strings.ToUpper(line))
	return line == "Y" || line == "YES"
}

// defaultInitReposRunner implements InitReposRunner with the real git and gh commands.
type defaultInitReposRunner struct{}

func (defaultInitReposRunner) GitInit(dir string) error {
	return ensureGitRepo(dir)
}
func (defaultInitReposRunner) GhRepoCreate(dir, repo string) error {
	return createGitHubRepo(dir, repo)
}
func (defaultInitReposRunner) GitAddCommitPush(dir string) error {
	return commitAndPush(dir)
}

// ensureGitRepo runs git init in dir if it is not already a git repo.
func ensureGitRepo(dir string) error {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return nil
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// createGitHubRepo creates repo via gh and sets origin. Repo is "owner/name".
func createGitHubRepo(dir, repo string) error {
	// gh repo create owner/name --private --source=. --remote=origin (run from dir)
	cmd := exec.Command("gh", "repo", "create", repo, "--private", "--source", ".", "--remote", "origin")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// commitAndPush does git add ., git commit -m "...", git push -u origin HEAD in dir.
func commitAndPush(dir string) error {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "commit", "-m", "chore: initial commit")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		os.Stderr.Write(out)
	}
	if err != nil {
		// "nothing to commit" is fine: the directory was committed before, so just push.
		if strings.Contains(string(out), "nothing to commit") || strings.Contains(string(out), "no changes added") {
			// continue to push
		} else {
			return err
		}
	}

	cmd = exec.Command("git", "push", "-u", "origin", "HEAD")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
