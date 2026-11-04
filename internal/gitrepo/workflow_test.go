package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	client, err := New()
	if err != nil {
		t.Fatal(err)
	}
	client.env = append(client.env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	dir := t.TempDir()
	if _, err := client.run(dir, "init", "--initial-branch=main"); err != nil {
		t.Fatal(err)
	}
	repo, err := client.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "config", "user.name", "BranchPier Test")
	runGit(t, repo, "config", "user.email", "branchpier@example.test")
	return repo
}

func runGit(t *testing.T, repo *Repo, args ...string) string {
	t.Helper()
	result, err := repo.run(args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(result, "\n")
}

func status(t *testing.T, repo *Repo) State {
	t.Helper()
	state, err := repo.Status()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func writeFile(t *testing.T, repo *Repo, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo.path, name), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCommitAndPublish(t *testing.T) {
	repo := newTestRepo(t)
	remoteDir := t.TempDir()
	if _, err := repo.git.run(remoteDir, "init", "--bare", "--initial-branch=main"); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "remote", "add", "origin", remoteDir)
	writeFile(t, repo, "hello.txt", "hello\n")
	state := status(t, repo)
	if state.Head != "" || len(state.Files) != 1 || !state.Files[0].Untracked {
		t.Fatalf("unexpected initial status: %+v", state)
	}
	file := state.Files[0]
	diff, err := repo.Diff(file.ID, false)
	if err != nil || !strings.Contains(diff, "+hello") {
		t.Fatalf("untracked diff = %q, %v", diff, err)
	}
	if err := repo.SetStaged(file.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetStaged(file.ID, false); err != nil {
		t.Fatal(err)
	}
	if !status(t, repo).Files[0].Untracked {
		t.Fatal("unstaging before the first commit must keep the working file")
	}
	if err := repo.SetStaged(file.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.Commit("Initial commit"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Switch("feature/first-push", true); err != nil {
		t.Fatal(err)
	}
	state = status(t, repo)
	if state.Target == nil || state.Target.Remote != "origin" || state.Target.Branch != "feature/first-push" {
		t.Fatalf("wrong publish target: %+v", state.Target)
	}
	if err := repo.Sync("push", state.Branch, "stale-head", *state.Target); err == nil {
		t.Fatal("push accepted a stale head")
	}
	if err := repo.Sync("push", state.Branch, state.Head, *state.Target); err != nil {
		t.Fatal(err)
	}
	if upstream := status(t, repo).Upstream; upstream != "origin/feature/first-push" {
		t.Fatalf("upstream = %q", upstream)
	}
	remoteHead, err := repo.git.run(remoteDir, "rev-parse", "refs/heads/feature/first-push")
	if err != nil || strings.TrimSpace(remoteHead) != state.Head {
		t.Fatalf("published head = %q, %v", remoteHead, err)
	}
	state = status(t, repo)
	for _, action := range []string{"fetch", "pull", "push"} {
		if err := repo.Sync(action, state.Branch, state.Head, *state.Target); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	commits, err := repo.History()
	if err != nil || len(commits) != 1 || commits[0].Hash != state.Head {
		t.Fatalf("history = %+v, %v", commits, err)
	}
	runGit(t, repo, "config", "branch.feature/first-push.merge", "refs/heads/main")
	state = status(t, repo)
	if err := repo.Sync("push", state.Branch, state.Head, *state.Target); err == nil || !strings.Contains(err.Error(), "different name") {
		t.Fatalf("expected push to a differently named upstream to be blocked, got %v", err)
	}
	patch, err := repo.CommitDiff(state.Head)
	if err != nil || !strings.Contains(patch, "+hello") {
		t.Fatalf("commit patch = %q, %v", patch, err)
	}
}

func TestUnstageRenameWithLiteralPaths(t *testing.T) {
	repo := newTestRepo(t)
	old, renamed := "original.txt", "[:foo]* with space\n.txt"
	writeFile(t, repo, old, "first line\nsecond line\n")
	runGit(t, repo, "add", "--", old)
	runGit(t, repo, "commit", "-m", "Add file")
	runGit(t, repo, "mv", "--", old, renamed)
	files := status(t, repo).Files
	if len(files) != 1 || files[0].Original != old || files[0].Path != renamed || files[0].Index != "R" {
		t.Fatalf("unexpected rename status: %+v", files)
	}
	writeFile(t, repo, renamed, "first line\nsecond line\nthird line\n")
	if err := repo.SetStaged(files[0].ID, true); err != nil {
		t.Fatalf("stage edits to an already staged rename: %v", err)
	}
	for _, file := range status(t, repo).Files {
		if file.Worktree != "." {
			t.Fatalf("rename still has unstaged edits: %+v", file)
		}
	}
	if err := repo.SetStaged(files[0].ID, false); err != nil {
		t.Fatal(err)
	}
	for _, file := range status(t, repo).Files {
		if file.Index != "." {
			t.Fatalf("rename path remained staged: %+v", file)
		}
	}
	if _, err := os.Stat(filepath.Join(repo.path, renamed)); err != nil {
		t.Fatalf("unstaging changed working tree: %v", err)
	}
}

func TestRemoteBrowserURL(t *testing.T) {
	for _, test := range []struct{ remote, want string }{
		{"git@gitlab.example.com:group/sub/repo.git", "https://gitlab.example.com/group/sub/repo"},
		{"ssh://git@gitlab.example.com:2222/group/repo.git", "https://gitlab.example.com/group/repo"},
		{"https://gitlab.example.com:8443/group/repo.git", "https://gitlab.example.com:8443/group/repo"},
		{"https://token@gitlab.example.com/group/repo.git", ""},
		{"ext::sh -c something", ""},
	} {
		if got := webURL(test.remote); got != test.want {
			t.Errorf("webURL(%q) = %q, want %q", test.remote, got, test.want)
		}
	}
}
