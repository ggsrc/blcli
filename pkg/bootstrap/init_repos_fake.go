package bootstrap

import "sync"

// FakeInitReposRunner is an InitReposRunner for tests: it records every call
// and runs no commands.
type FakeInitReposRunner struct {
	mu sync.Mutex

	GitInits          []string       // dir of each GitInit(dir) call
	GhRepoCreates     []GhRepoCreate // each GhRepoCreate(dir, repo) call
	GitAddCommitPushs []string       // dir of each GitAddCommitPush(dir) call

	// Optional errors returned by each call, to test failure paths.
	GitInitErr          error
	GhRepoCreateErr     error
	GitAddCommitPushErr error
}

// GhRepoCreate records one GhRepoCreate(dir, repo) call.
type GhRepoCreate struct {
	Dir  string
	Repo string
}

// GitInit records the call and returns FakeInitReposRunner.GitInitErr.
func (f *FakeInitReposRunner) GitInit(dir string) error {
	f.mu.Lock()
	f.GitInits = append(f.GitInits, dir)
	err := f.GitInitErr
	f.mu.Unlock()
	return err
}

// GhRepoCreate records the call and returns FakeInitReposRunner.GhRepoCreateErr.
func (f *FakeInitReposRunner) GhRepoCreate(dir, repo string) error {
	f.mu.Lock()
	f.GhRepoCreates = append(f.GhRepoCreates, GhRepoCreate{Dir: dir, Repo: repo})
	err := f.GhRepoCreateErr
	f.mu.Unlock()
	return err
}

// GitAddCommitPush records the call and returns FakeInitReposRunner.GitAddCommitPushErr.
func (f *FakeInitReposRunner) GitAddCommitPush(dir string) error {
	f.mu.Lock()
	f.GitAddCommitPushs = append(f.GitAddCommitPushs, dir)
	err := f.GitAddCommitPushErr
	f.mu.Unlock()
	return err
}
