package gitrepo

import (
	"errors"
	"regexp"
	"strings"
)

type Commit struct {
	Hash    string   `json:"hash"`
	Parents []string `json:"parents"`
	Subject string   `json:"subject"`
	Author  string   `json:"author"`
	Date    string   `json:"date"`
	Refs    string   `json:"refs"`
}

func (r *Repo) History() ([]Commit, error) {
	args := []string{"log", "--all", "--topo-order", "--max-count=150", "--format=%H%x00%P%x00%s%x00%an%x00%as%x00%D", "-z"}
	if _, err := r.run("rev-parse", "--verify", "--quiet", "HEAD"); err == nil {
		args = append(args, "HEAD")
	} else if !exitCode(err, 1) {
		return nil, err
	}
	output, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	records := strings.Split(strings.TrimSuffix(output, "\x00"), "\x00")
	commits := []Commit{}
	if output == "" {
		return commits, nil
	}
	if len(records)%6 != 0 {
		return nil, errors.New("invalid Git history record")
	}
	for i := 0; i < len(records); i += 6 {
		commits = append(commits, Commit{
			Hash: records[i], Parents: strings.Fields(records[i+1]), Subject: records[i+2],
			Author: records[i+3], Date: records[i+4], Refs: records[i+5],
		})
	}
	return commits, nil
}

var objectID = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

func (r *Repo) CommitDiff(hash string) (string, error) {
	if !objectID.MatchString(hash) {
		return "", errors.New("invalid commit ID")
	}
	return r.run("show", "--format=", "--first-parent", "--diff-merges=first-parent", "--no-ext-diff", "--no-textconv", "--no-color", hash, "--")
}
