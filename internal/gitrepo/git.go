package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const outputLimit = 8 << 20

var urlCredentials = regexp.MustCompile(`(https?://|ssh://)[^\s/@]+@`)

type Client struct {
	binary string
	env    []string
}

type Repo struct {
	git  *Client
	path string
}

func New() (*Client, error) {
	env := os.Environ()
	if runtime.GOOS == "darwin" {
		path := strings.Join([]string{os.Getenv("PATH"), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"}, string(os.PathListSeparator))
		env = append(env, "PATH="+path)
	}
	binary, err := exec.LookPath("git")
	if err != nil && runtime.GOOS == "darwin" {
		binary, err = exec.LookPath("/usr/bin/git")
	}
	if err != nil {
		return nil, errors.New("Git was not found. On macOS, install the Xcode Command Line Tools or Homebrew Git")
	}
	return &Client{binary: binary, env: env}, nil
}

func (c *Client) Open(path string) (*Repo, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	output, err := c.run(absolute, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("open Git working tree: %w", err)
	}
	return &Repo{git: c, path: strings.TrimSuffix(output, "\n")}, nil
}

func (c *Client) Clone(remote, parent string) (*Repo, error) {
	remote = strings.TrimSpace(remote)
	if webURL(remote) == "" {
		return nil, errors.New("enter an HTTPS or SSH clone URL without embedded credentials")
	}
	name := strings.TrimSuffix(filepath.Base(remoteProject(remote)), ".git")
	if name == "." || name == ".." || name == "" || strings.ContainsAny(name, "/\\") {
		return nil, errors.New("could not determine the repository folder name from the clone URL")
	}
	target := filepath.Join(parent, name)
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("clone destination is unavailable: %s", target)
	}
	if _, err := c.run(parent, "clone", "--", remote, target); err != nil {
		return nil, err
	}
	return c.Open(target)
}

func (r *Repo) run(args ...string) (string, error) {
	return r.git.run(r.path, args...)
}

func (c *Client) run(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, c.binary, append([]string{"--no-pager", "-c", "protocol.ext.allow=never"}, args...)...)
	command.Dir = dir
	command.WaitDelay = 2 * time.Second
	for _, item := range c.env {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
			"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_PREFIX", "GIT_NAMESPACE", "GIT_GLOB_PATHSPECS",
			"GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS":
			continue
		}
		command.Env = append(command.Env, item)
	}
	command.Env = append(command.Env, "GIT_TERMINAL_PROMPT=0", "GIT_LITERAL_PATHSPECS=1", "LC_ALL=C")
	var stdout, stderr cappedOutput
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("git %s timed out", args[0])
	}
	if err != nil {
		detail := strings.TrimSpace(redact(stderr.String()))
		if detail == "" {
			detail = err.Error()
		}
		return stdout.String(), &commandError{command: args[0], detail: detail, cause: err}
	}
	if stdout.truncated {
		return "", fmt.Errorf("git %s output exceeded %d MiB", args[0], outputLimit>>20)
	}
	return stdout.String(), nil
}

type commandError struct {
	command string
	detail  string
	cause   error
}

func (e *commandError) Error() string { return "git " + e.command + ": " + e.detail }
func (e *commandError) Unwrap() error { return e.cause }

func exitCode(err error, code int) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == code
}

type cappedOutput struct {
	buffer    bytes.Buffer
	truncated bool
}

func (b *cappedOutput) String() string { return b.buffer.String() }

func (b *cappedOutput) Write(p []byte) (int, error) {
	size := len(p)
	remaining := outputLimit - b.buffer.Len()
	if size > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buffer.Write(p)
	return size, nil
}

func redact(text string) string { return urlCredentials.ReplaceAllString(text, "${1}") }

var scpRemote = regexp.MustCompile(`^(?:[^@/:\s]+@)?([a-zA-Z0-9][a-zA-Z0-9.-]*):([^:\s][^\s]*)$`)

func scpParts(remote string) []string {
	if strings.Contains(remote, "://") {
		return nil
	}
	return scpRemote.FindStringSubmatch(remote)
}

func remoteProject(remote string) string {
	if match := scpParts(remote); match != nil {
		return strings.TrimPrefix(match[2], "/")
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Path, "/")
}

func webURL(remote string) string {
	if match := scpParts(remote); match != nil {
		return projectURL("https", match[1], match[2])
	}
	parsed, err := url.Parse(remote)
	if err != nil || parsed.Hostname() == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	switch parsed.Scheme {
	case "https":
		if parsed.User == nil {
			return projectURL("https", parsed.Host, parsed.Path)
		}
	case "ssh":
		if parsed.User != nil {
			if _, hasPassword := parsed.User.Password(); hasPassword {
				return ""
			}
		}
		host := parsed.Hostname()
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		return projectURL("https", host, parsed.Path)
	}
	return ""
}

func projectURL(scheme, host, path string) string {
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if path == "" || strings.Contains(path, "\\") {
		return ""
	}
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "." || segment == ".." {
			return ""
		}
	}
	return (&url.URL{Scheme: scheme, Host: host, Path: "/" + path}).String()
}
