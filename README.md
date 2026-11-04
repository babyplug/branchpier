# BranchPier

BranchPier is an MIT-licensed desktop Git client with a focused staging workflow, a visual commit graph, and support for private and self-hosted GitLab repositories. It also works with other Git hosts over standard Git transports.

## Run on macOS

Install Go 1.26, Node.js 22.12+ or 24 LTS, and Git. Run `xcode-select --install` if the Xcode Command Line Tools are missing.

```sh
make app
open build/BranchPier.app
```

For a terminal launch, run `make build` and then `./branchpier`. Run `make test` to exercise the Git backend against temporary repositories and a local bare remote.

## Workflow

1. Open an existing working tree or clone into a folder. Clone accepts HTTPS and SSH URLs.
2. Under **Changes**, choose a file to review its diff. Stage the files you want and enter a commit message.
3. Use **New branch** or the branch selector, then **Publish branch** or **Push**. The toolbar shows the remote and branch before sending changes.
4. Open **History** for the most recent 150 commits across refs. Select a commit to inspect its patch. Merge commit patches are shown against the first parent.
5. Use **Open remote in browser** to continue on GitLab or your other Git host.

BranchPier uses the installed Git executable, Git config, SSH agent, and configured HTTPS credential helper. For a private repository, get `git ls-remote <clone-url>` working in your terminal first. With SSH, make sure the host key is trusted and the key is available to your macOS session. With HTTPS, configure a helper that can authenticate without an interactive terminal prompt. Git signing and hooks continue to run through Git.

Push uses the branch's tracking remote when configured and requires a same-named upstream. For a new branch it uses `origin` or the sole configured remote and publishes to the same branch name. Pull uses fast-forward-only updates. When no sync target is shown, configure a tracking branch or add an `origin` remote with a single push URL. The browser link assumes an SSH remote uses the same hostname as its web UI.

BranchPier is developed for macOS first. This release stages whole files. Resolve conflicts in Git or your editor, then refresh BranchPier.
