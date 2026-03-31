package bootstrap

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InitReposRunner 用于执行 git/gh 的抽象，便于测试时注入 Fake。
type InitReposRunner interface {
	GitInit(dir string) error
	GhRepoCreate(dir, repo string) error
	GitAddCommitPush(dir string) error
}

// InitReposOptions holds options for apply init-repos command.
type InitReposOptions struct {
	WorkspaceDir         string // -d, root containing terraform/ kubernetes/ gitops/
	TerraformDir         string
	KubernetesDir        string
	GitOpsDir            string
	GitHubOrg            string // e.g. someone or github.com/someone
	RepoNameTerraform    string // optional override, default {org}/terraform
	RepoNameKubernetes  string
	RepoNameGitOps       string
	Runner               InitReposRunner // 可选，测试时注入 Fake
	Stdin                io.Reader       // 可选，确认提示的输入来源，默认 os.Stdin
}

// ExecuteInitRepos runs git init, prompts to create GitHub repo, and prompts to commit/push
// for each of terraform, kubernetes, gitops dirs that exist.
// User must type Y to continue at each prompt; N or any other input stops the flow.
// 测试时可通过 opts.Runner 注入 Fake、opts.Stdin 注入输入。
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
		fmt.Printf("为 %s 创建 GitHub 仓库 https://github.com/%s？[Y/n]: ", d.name, d.repo)
		if !confirmY(inBuf) {
			return fmt.Errorf("用户取消（创建仓库）")
		}
		if err := runner.GhRepoCreate(abs, d.repo); err != nil {
			return fmt.Errorf("%s: 创建 GitHub 仓库: %w", d.name, err)
		}

		// Prompt: commit and push
		fmt.Printf("提交并推送到 %s？[Y/n]: ", d.repo)
		if !confirmY(inBuf) {
			return fmt.Errorf("用户取消（提交推送）")
		}
		if err := runner.GitAddCommitPush(abs); err != nil {
			return fmt.Errorf("%s: 提交推送: %w", d.name, err)
		}
		fmt.Printf("✅ %s 完成\n", d.name)
	}
	fmt.Println()
	return nil
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
// 多次调用时须传入同一个 *bufio.Reader，否则会因 Scanner 预读导致后续读不到下一行。
func confirmY(r *bufio.Reader) bool {
	line, err := r.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	line = strings.TrimSpace(strings.ToUpper(line))
	return line == "Y" || line == "YES"
}

// defaultInitReposRunner 使用真实 git/gh 命令实现 InitReposRunner。
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
		// 允许 "nothing to commit"：已提交过则直接 push
		if strings.Contains(string(out), "nothing to commit") || strings.Contains(string(out), "no changes added") {
			// 继续执行 push
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
