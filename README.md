# gitflow-helper

A single static binary that enforces the [git-flow branching model](https://nvie.com/posts/a-successful-git-branching-model/) with an interactive TUI for humans and a `--json` mode for AI agents. Detects your IDE (Cursor / VS Code Copilot) and generates pre-flight rules automatically.

## Features

- **14 CLI commands** covering the full gitflow lifecycle: status, pull, start, finish, sync, switch, backmerge, cleanup, health, doctor, log, undo, releasenotes, init
- **Interactive TUI** with dashboard, action menu, command palette, and help overlay
- **JSON mode** (`--json`) for seamless integration with AI agents (Cursor, Copilot, Claude Code, etc.)
- **IDE detection** — automatically generates `.cursor/rules/`, `.github/copilot-instructions.md`, or `AGENTS.md`
- **Zero runtime dependencies** — static binary, only needs `git` (no git-flow extensions required)
- **Cross-platform** — Linux x86_64, Windows x86_64, macOS universal (x86 + ARM)
- **Publishable as an [Agent Skill](https://skills.sh/)** via `npx skills add`

## Install

### From GitHub Releases

Download the latest binary for your platform from the [Releases](../../releases) page.

```bash
# macOS (universal binary — works on Intel and Apple Silicon)
curl -Lo gitflow https://github.com/novaemx/gitflow-helper/releases/latest/download/gitflow-darwin-universal
chmod +x gitflow
sudo mv gitflow /usr/local/bin/

# Linux x86_64
curl -Lo gitflow https://github.com/novaemx/gitflow-helper/releases/latest/download/gitflow-linux-amd64
chmod +x gitflow
sudo mv gitflow /usr/local/bin/

# Windows x86_64
# Download gitflow-windows-amd64.exe and add to PATH
```

### From Source

```bash
go install github.com/novaemx/gitflow-helper/cmd/gitflow@latest
```

### Build Locally

```bash
git clone https://github.com/novaemx/gitflow-helper.git
cd gitflow-helper
make build          # current platform
make build-all      # all platforms
make universal      # macOS universal binary
```

## Quick Start

```bash
# Initialize gitflow in your project
gitflow init

# Auto-detect your IDE and generate rules
gitflow setup

# Launch the interactive TUI
gitflow
```

## Usage

### Interactive TUI

Run `gitflow` without arguments to launch the full-screen dashboard:

- `j`/`k` or arrow keys to navigate
- `Enter` to execute an action
- `/` to open the command palette (type to filter)
- `?` for help
- `r` to refresh
- `q` to quit

### CLI Commands

All commands support `--json` for machine-readable output.

```bash
gitflow status                     # repo state dashboard
gitflow pull                       # safe fetch + fast-forward merge
gitflow start feature my-feature   # start a feature branch
gitflow start bugfix fix-name      # start a bugfix branch
gitflow start release 1.2.0        # start a release
gitflow start hotfix 1.1.1         # start a hotfix
gitflow finish                     # finish current flow branch
gitflow sync                       # sync branch with parent
gitflow switch develop             # switch to a branch
gitflow backmerge                  # merge main into develop
gitflow cleanup                    # delete merged branches
gitflow health                     # full repo health audit
gitflow doctor                     # validate prerequisites
gitflow log -n 20                  # gitflow-aware commit log
gitflow undo                       # undo last operation (reflog)
gitflow releasenotes               # generate RELEASE_NOTES.md
gitflow init                       # initialize git-flow
gitflow setup                      # detect IDE & generate rules
gitflow setup --ide cursor         # force Cursor rules only
gitflow setup --ide copilot        # force Copilot instructions only
```

### JSON Mode (for AI Agents)

```bash
gitflow --json status
gitflow --json start feature add-csv-export
gitflow --json finish
```

Exit codes: `0` success, `1` error, `2` conflict needing human intervention.

## IDE Setup

Run `gitflow setup` once per project. It auto-detects your IDE and creates:

| IDE               | Generated files                               |
|-------------------|-----------------------------------------------|
| Cursor            | `.cursor/rules/gitflow-preflight.mdc`, `.cursor/mcp.json` |
| VS Code + Copilot | `.github/copilot-instructions.md`, `.vscode/mcp.json` |
| Both / Unknown    | Cursor + Copilot files + `AGENTS.md`          |

These files instruct the AI agent to run `gitflow --json status` before modifying any code, enforcing the pre-flight check automatically.

### Copilot-Specific Notes

To ensure the gitflow skill works in Copilot end-to-end:

1. Install `gitflow` binary and make sure it is in PATH.
2. Run `gitflow setup --ide copilot` in your repository root.
3. Verify these files exist:
  - `.github/copilot-instructions.md`
  - `.vscode/mcp.json` (contains `"gitflow"` server using `gitflow serve`)

If `gitflow` is not in PATH when setup runs, MCP config still gets generated, but command execution will fail until PATH is fixed.

## Agent Skill (skills.sh)

This project is publishable as an [Agent Skill](https://skills.sh/). The skill file lives at `skills/gitflow/SKILL.md`.

### Install the Skill

```bash
npx skills add <owner>/gitflow-helper
```

This installs the gitflow pre-flight enforcement skill into your project so any compatible AI agent (Cursor, Copilot, Claude Code, Cline, etc.) will automatically follow gitflow discipline.

## Configuration

Create a `.gitflow.json` in your project root for custom settings:

```json
{
  "remote": "origin",
  "main_branch": "main",
  "develop_branch": "develop",
  "version_file": "package.json",
  "version_pattern": "\"version\"\\s*:\\s*\"([^\"]+)\"",
  "tag_prefix": "v",
  "bump_command": "npm version {part} --no-git-tag-version"
}
```

If absent, the tool auto-detects common version files (package.json, pyproject.toml, Cargo.toml, etc.) and uses git tags for versioning.

## Building

```bash
make build        # build for current platform
make build-all    # cross-compile linux/windows/macOS
make universal    # create macOS universal binary with lipo
make test         # run tests
make vet          # run go vet
make release-local                    # build release artifacts locally (no publish)
make publish-github TAG=v0.5.12       # create/update GitHub release and upload local artifacts
make publish-homebrew TAG=v0.5.12     # update Homebrew formula checksums/URLs
make publish-winget TAG=v0.5.12       # update Winget manifest checksum/URL
make publish-choco TAG=v0.5.12        # update Chocolatey metadata checksum/URL
make publish-all TAG=v0.5.12          # do all of the above in one command
make install      # install to GOPATH/bin
```

## Local-Only Release Policy

This repository does not use GitHub Actions to compile binaries.

- All release binaries are built locally on maintainer machines.
- GitHub Releases are used only as artifact hosting/distribution.
- Homebrew, Winget, and Chocolatey manifests point to those GitHub Release artifacts.

### Publish Flow (No Cloud Build)

```bash
# Create/update the GitHub release and upload locally-built binaries
make publish-github TAG=v0.5.12

# Refresh package manifests to point at that GitHub release
make publish-homebrew TAG=v0.5.12
make publish-winget TAG=v0.5.12
make publish-choco TAG=v0.5.12

# Or do everything in one shot
make publish-all TAG=v0.5.12
```

## License

MIT. See [LICENSE](LICENSE).

## Author

Luis Lozano
