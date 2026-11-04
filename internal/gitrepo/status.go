package gitrepo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type File struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Original   string `json:"original,omitempty"`
	Index      string `json:"index"`
	Worktree   string `json:"worktree"`
	Untracked  bool   `json:"untracked"`
	Conflicted bool   `json:"conflicted"`
}

type Target struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	URL    string `json:"url"`
	WebURL string `json:"webUrl"`
}

type State struct {
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Branch   string   `json:"branch"`
	Head     string   `json:"head"`
	Upstream string   `json:"upstream"`
	Ahead    int      `json:"ahead"`
	Behind   int      `json:"behind"`
	Files    []File   `json:"files"`
	Branches []string `json:"branches"`
	Target   *Target  `json:"target"`
}

func (r *Repo) Status() (State, error) {
	out, err := r.run("status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all")
	if err != nil {
		return State{}, err
	}
	state, err := parseStatus(out)
	if err != nil {
		return State{}, err
	}
	state.Path, state.Name = r.path, filepath.Base(r.path)
	branches, err := r.run("for-each-ref", "--format=%(refname:lstrip=2)", "refs/heads/")
	if err != nil {
		return State{}, err
	}
	state.Branches = strings.Fields(branches)
	state.Target, err = r.target(state)
	return state, err
}

func parseStatus(out string) (State, error) {
	state := State{Files: []File{}}
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		line := records[i]
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			key, value, _ := strings.Cut(line[2:], " ")
			switch key {
			case "branch.oid":
				if value != "(initial)" {
					state.Head = value
				}
			case "branch.head":
				if value != "(detached)" {
					state.Branch = value
				}
			case "branch.upstream":
				state.Upstream = value
			case "branch.ab":
				ahead, behind, _ := strings.Cut(value, " ")
				state.Ahead, _ = strconv.Atoi(strings.TrimPrefix(ahead, "+"))
				state.Behind, _ = strconv.Atoi(strings.TrimPrefix(behind, "-"))
			}
			continue
		}
		var file File
		var fields int
		switch line[0] {
		case '?':
			if len(line) < 3 {
				return State{}, errors.New("invalid untracked status record")
			}
			file = File{Path: line[2:], Index: ".", Worktree: "?", Untracked: true}
		case '1':
			fields = 9
		case '2':
			fields = 10
			i++
			if i >= len(records) || records[i] == "" {
				return State{}, errors.New("missing original path in Git rename status")
			}
			file.Original = records[i]
		case 'u':
			fields = 11
			file.Conflicted = true
		default:
			continue
		}
		if fields != 0 {
			parts := strings.SplitN(line, " ", fields)
			if len(parts) != fields || len(parts[1]) != 2 {
				return State{}, errors.New("invalid porcelain status record")
			}
			file.Index, file.Worktree, file.Path = parts[1][:1], parts[1][1:], parts[fields-1]
		}
		file.ID = base64.RawURLEncoding.EncodeToString([]byte(file.Path))
		state.Files = append(state.Files, file)
	}
	return state, nil
}

func (r *Repo) target(state State) (*Target, error) {
	if state.Branch == "" {
		return nil, nil
	}
	remote := r.config("branch." + state.Branch + ".remote")
	branch := r.config("branch." + state.Branch + ".merge")
	if state.Upstream == "" && remote == "" {
		out, err := r.run("remote")
		if err != nil {
			return nil, err
		}
		names := strings.Fields(out)
		for _, name := range names {
			if name == "origin" || len(names) == 1 {
				remote = name
				break
			}
		}
	}
	if remote == "" || remote == "." {
		return nil, nil
	}
	if branch == "" {
		branch = "refs/heads/" + state.Branch
	}
	if !strings.HasPrefix(branch, "refs/heads/") {
		return nil, nil
	}
	out, err := r.run("remote", "get-url", "--push", "--all", "--", remote)
	if err != nil {
		return nil, nil
	}
	urls := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(urls) != 1 {
		return nil, nil
	}
	return &Target{Remote: remote, Branch: strings.TrimPrefix(branch, "refs/heads/"), URL: redact(urls[0]), WebURL: webURL(urls[0])}, nil
}

func (r *Repo) config(key string) string {
	value, _ := r.run("config", "--get", key)
	return strings.TrimSuffix(value, "\n")
}

func (r *Repo) file(id string) (File, error) {
	out, err := r.run("status", "--porcelain=v2", "-z", "--untracked-files=all")
	if err != nil {
		return File{}, err
	}
	state, err := parseStatus(out)
	if err != nil {
		return File{}, err
	}
	for _, file := range state.Files {
		if file.ID == id {
			return file, nil
		}
	}
	return File{}, errors.New("the file changed outside BranchPier; refresh and try again")
}

func (f File) paths() []string {
	if f.Original != "" && (f.Index == "R" || f.Worktree == "R") {
		return []string{f.Original, f.Path}
	}
	return []string{f.Path}
}

func (r *Repo) Diff(id string, staged bool) (string, error) {
	file, err := r.file(id)
	if err != nil {
		return "", err
	}
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color"}
	if !staged && file.Untracked {
		output, err := r.run(append(args, "--no-index", "--", "/dev/null", "./"+file.Path)...)
		if exitCode(err, 1) {
			if len(output) >= outputLimit {
				return "", errors.New("diff is too large to display")
			}
			return output, nil
		}
		return output, err
	}
	if staged {
		args = append(args, "--cached")
	}
	return r.run(append(append(args, "--"), file.paths()...)...)
}

func (r *Repo) SetStaged(id string, staged bool) error {
	file, err := r.file(id)
	if err != nil {
		return err
	}
	if file.Conflicted {
		return errors.New("resolve this conflict using Git or your editor before staging it")
	}
	paths := file.paths()
	var args []string
	if staged {
		if file.Worktree == "." {
			return nil
		}
		args = []string{"add", "-A", "--"}
		if file.Worktree != "R" {
			paths = []string{file.Path}
		}
	} else {
		if file.Index == "." {
			return nil
		}
		if _, err := r.run("rev-parse", "--verify", "--quiet", "HEAD"); err == nil {
			args = []string{"restore", "--staged", "--"}
		} else if exitCode(err, 1) {
			args = []string{"rm", "--cached", "-f", "--"}
		} else {
			return err
		}
	}
	_, err = r.run(append(args, paths...)...)
	return err
}

func (r *Repo) Commit(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("enter a commit message")
	}
	if _, err := r.run("commit", "-m", message); err != nil {
		return err
	}
	return nil
}

func (r *Repo) Switch(branch string, create bool) error {
	check, err := r.run("check-ref-format", "--branch", branch)
	if err != nil || strings.TrimSuffix(check, "\n") != branch {
		return errors.New("enter a valid branch name")
	}
	args := []string{"switch", "--", branch}
	if create {
		args = []string{"switch", "--no-track", "-c", branch, "--"}
	}
	_, err = r.run(args...)
	return err
}

func (r *Repo) Sync(action, expectedBranch, expectedHead string, expectedTarget Target) error {
	state, err := r.Status()
	if err != nil {
		return err
	}
	if state.Target == nil || *state.Target != expectedTarget || state.Branch != expectedBranch || state.Head != expectedHead {
		return errors.New("the branch or remote changed; refresh before syncing")
	}
	target := *state.Target
	switch action {
	case "fetch":
		_, err = r.run("fetch", "--", target.Remote)
	case "pull":
		if state.Upstream == "" {
			return errors.New("publish this branch before pulling")
		}
		_, err = r.run("pull", "--ff-only", "--no-rebase", "--", target.Remote, "refs/heads/"+target.Branch)
	case "push":
		if state.Head == "" {
			return errors.New("create a commit before pushing")
		}
		if target.Branch != state.Branch {
			return errors.New("the upstream branch has a different name; push explicitly in Git")
		}
		args := []string{"push", "--no-follow-tags", "--no-recurse-submodules", "--no-mirror"}
		if state.Upstream == "" {
			args = append(args, "--set-upstream")
		}
		_, err = r.run(append(args, "--", target.Remote, "refs/heads/"+state.Branch+":refs/heads/"+target.Branch)...)
	default:
		return fmt.Errorf("unknown Git action %q", action)
	}
	return err
}
