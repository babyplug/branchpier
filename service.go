package main

import (
	"errors"
	"sync"

	"github.com/babyplug/branchpier/internal/gitrepo"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type GitService struct {
	mu   sync.Mutex
	repo *gitrepo.Repo
	path string
}

func (s *GitService) Open() (*gitrepo.State, error) {
	return s.chooseRepo("Open Git repository", (*gitrepo.Client).Open)
}

func (s *GitService) Clone(remote string) (*gitrepo.State, error) {
	return s.chooseRepo("Choose a parent folder for the clone", func(git *gitrepo.Client, path string) (*gitrepo.Repo, error) {
		return git.Clone(remote, path)
	})
}

func (s *GitService) chooseRepo(title string, open func(*gitrepo.Client, string) (*gitrepo.Repo, error)) (*gitrepo.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	git, err := gitrepo.New()
	if err != nil {
		return nil, err
	}
	path, err := application.Get().Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return nil, err
	}
	repo, err := open(git, path)
	if err != nil {
		return nil, err
	}
	state, err := repo.Status()
	if err != nil {
		return nil, err
	}
	s.repo, s.path = repo, state.Path
	return &state, nil
}

func (s *GitService) Status(path string) (gitrepo.State, error) {
	return withRepo(s, path, (*gitrepo.Repo).Status)
}

func (s *GitService) Diff(path, file string, staged bool) (string, error) {
	return withRepo(s, path, func(repo *gitrepo.Repo) (string, error) {
		return repo.Diff(file, staged)
	})
}

func (s *GitService) SetStaged(path, file string, staged bool) error {
	return s.change(path, func(repo *gitrepo.Repo) error { return repo.SetStaged(file, staged) })
}

func (s *GitService) Commit(path, message string) error {
	return s.change(path, func(repo *gitrepo.Repo) error { return repo.Commit(message) })
}

func (s *GitService) Switch(path, branch string, create bool) error {
	return s.change(path, func(repo *gitrepo.Repo) error { return repo.Switch(branch, create) })
}

func (s *GitService) Sync(path, action, branch, head string, target gitrepo.Target) error {
	return s.change(path, func(repo *gitrepo.Repo) error { return repo.Sync(action, branch, head, target) })
}

func (s *GitService) History(path string) ([]gitrepo.Commit, error) {
	return withRepo(s, path, (*gitrepo.Repo).History)
}

func (s *GitService) CommitDiff(path, hash string) (string, error) {
	return withRepo(s, path, func(repo *gitrepo.Repo) (string, error) { return repo.CommitDiff(hash) })
}

func (s *GitService) OpenRemote(path string) error {
	return s.change(path, func(repo *gitrepo.Repo) error {
		state, err := repo.Status()
		if err != nil {
			return err
		}
		if state.Target == nil || state.Target.WebURL == "" {
			return errors.New("this remote has no browser URL")
		}
		return application.Get().Browser.OpenURL(state.Target.WebURL)
	})
}

func (s *GitService) change(path string, run func(*gitrepo.Repo) error) error {
	_, err := withRepo(s, path, func(repo *gitrepo.Repo) (struct{}, error) {
		return struct{}{}, run(repo)
	})
	return err
}

func withRepo[T any](s *GitService, path string, run func(*gitrepo.Repo) (T, error)) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.repo == nil || path != s.path {
		var zero T
		return zero, errors.New("the active repository changed; open the repository and try again")
	}
	return run(s.repo)
}
