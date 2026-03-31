package bootstrap

import "sync"

// FakeInitReposRunner 用于测试的 InitReposRunner 实现，记录每次调用且不执行真实命令。
type FakeInitReposRunner struct {
	mu sync.Mutex

	GitInits          []string       // GitInit(dir) 的 dir 列表
	GhRepoCreates     []GhRepoCreate // GhRepoCreate(dir, repo) 的调用
	GitAddCommitPushs []string       // GitAddCommitPush(dir) 的 dir 列表

	// 可选：按调用返回错误，用于测错路径
	GitInitErr          error
	GhRepoCreateErr     error
	GitAddCommitPushErr error
}

// GhRepoCreate 记录一次 GhRepoCreate(dir, repo) 调用。
type GhRepoCreate struct {
	Dir  string
	Repo string
}

// GitInit 记录调用并返回 FakeInitReposRunner.GitInitErr。
func (f *FakeInitReposRunner) GitInit(dir string) error {
	f.mu.Lock()
	f.GitInits = append(f.GitInits, dir)
	err := f.GitInitErr
	f.mu.Unlock()
	return err
}

// GhRepoCreate 记录调用并返回 FakeInitReposRunner.GhRepoCreateErr。
func (f *FakeInitReposRunner) GhRepoCreate(dir, repo string) error {
	f.mu.Lock()
	f.GhRepoCreates = append(f.GhRepoCreates, GhRepoCreate{Dir: dir, Repo: repo})
	err := f.GhRepoCreateErr
	f.mu.Unlock()
	return err
}

// GitAddCommitPush 记录调用并返回 FakeInitReposRunner.GitAddCommitPushErr。
func (f *FakeInitReposRunner) GitAddCommitPush(dir string) error {
	f.mu.Lock()
	f.GitAddCommitPushs = append(f.GitAddCommitPushs, dir)
	err := f.GitAddCommitPushErr
	f.mu.Unlock()
	return err
}
