# gitflow-helper

A single static binary that enforces the [git-flow branching model](https://nvie.com/posts/a-successful-git-branching-model/) with an interactive TUI for humans and a `--json` mode for AI agents. Detects your IDE (Cursor / VS Code Copilot) and generates pre-flight rules automatically.

![Homebrew](https://img.shields.io/badge/homebrew-v0.5.40-green)
![Winget](https://img.shields.io/badge/winget-v0.5.40-green)

## Features

- **15 CLI commands** covering the full gitflow lifecycle: status, pull, push, start, finish, sync, switch, backmerge, cleanup, health, doctor, log, undo, releasenotes, init
- **Interactive TUI** with dashboard, action menu, command palette, and help overlay
- **JSON mode** (`--json`) for seamless integration with AI agents (Cursor, Copilot, Claude Code, etc.)
- **IDE detection** — automatically generates `.cursor/rules/`, `.github/copilot-instructions.md`, or `AGENTS.md`
- **Zero runtime dependencies** — static binary, only needs `git` (no git-flow extensions required)
- **Cross-platform** — Linux x86_64, Windows x86_64, macOS universal (x86 + ARM)
- **Embedded gitflow skill** — `gitflow setup` installs or updates `SKILL.md` in `.agents/skills/gitflow/` or `~/.agents/skills/gitflow/`

## Install

### Homebrew (Formula)

`gitflow-helper` is distributed as a Homebrew Formula (CLI tool), not a Cask.

```bash
brew tap novaemx/tap
brew install gitflow-helper
```

The formula installs shell completions for `bash`, `zsh`, and `fish` automatically.

### Debian / Ubuntu (APT)

The project ships a flat APT source file that installs the `.deb` packages directly from GitHub Releases.

Supported CPU architectures:

- `amd64` for `x86_64`
- `arm64` for `aarch64`

Check your Debian-family architecture with:

```bash
dpkg --print-architecture
```

Add the source and install:

```bash
sudo curl -fsSL https://github.com/novaemx/gitflow-helper/releases/latest/download/gitflow-helper.sources \
  -o /etc/apt/sources.list.d/gitflow-helper.sources
sudo apt update
sudo apt install gitflow-helper
```

### Rocky Linux 9 (DNF / YUM)

Rocky Linux uses a DNF/YUM repository definition plus static `repodata` tracked in this repository.

Supported CPU architectures:

- `x86_64`
- `aarch64`

Check your RPM-family architecture with:

```bash
uname -m
```

Add the repository and install:

```bash
sudo curl -fsSL https://github.com/novaemx/gitflow-helper/releases/latest/download/gitflow-helper-rocky.repo \
  -o /etc/yum.repos.d/gitflow-helper.repo
sudo dnf makecache
sudo dnf install gitflow-helper
```

### From GitHub Releases

Download the latest binary for your platform from the [Releases](../../releases) page.

```bash
# macOS (Intel + Apple Silicon)
curl -LO https://github.com/novaemx/gitflow-helper/releases/latest/download/gitflow-<version>-darwin-universal.tar.gz
tar -xzf gitflow-<version>-darwin-universal.tar.gz
sudo install -m 755 gitflow /usr/local/bin/gitflow

# Linux x86_64 (tarball fallback)
curl -LO https://github.com/novaemx/gitflow-helper/releases/latest/download/gitflow-<version>-linux-amd64.tar.gz
tar -xzf gitflow-<version>-linux-amd64.tar.gz
sudo install -m 755 gitflow /usr/local/bin/gitflow

# Linux arm64 / aarch64 (tarball fallback)
curl -LO https://github.com/novaemx/gitflow-helper/releases/latest/download/gitflow-<version>-linux-aarch64.tar.gz
tar -xzf gitflow-<version>-linux-aarch64.tar.gz
sudo install -m 755 gitflow /usr/local/bin/gitflow

# Windows x86_64
# Download gitflow-<version>-windows-amd64.zip and add gitflow.exe to PATH
```

If you prefer native Linux packages instead of tarballs, use the APT or DNF/YUM repo setup above.

### From Source

```bash
go install github.com/novaemx/gitflow-helper/cmd/gitflow@latest
```

### Build Locally

```bash
git clone https://github.com/novaemx/gitflow-helper.git
cd gitflow-helper
bash scripts/install-hooks.sh  # install git hooks (enforces gitflow policy)
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

Use `--log` to write troubleshooting output to `.gitflow/logs/gitflow.log`. Add `--debug` to include verbose debug details in the same file.

```bash
gitflow status                     # repo state dashboard
gitflow status --log               # write troubleshooting logs to .gitflow/logs/gitflow.log
gitflow status --log --debug       # include verbose debug details in the log file
gitflow pull                       # safe fetch + fast-forward merge
gitflow push                       # push current branch (target-aware)
gitflow push develop               # push current branch to target branch with validation
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
gitflow --json push feature/add-csv-export
gitflow --json start feature add-csv-export
gitflow --json finish
```

Exit codes: `0` success, `1` error, `2` conflict needing human intervention.

## IDE Setup

Run `gitflow setup` once per project. It auto-detects your IDE and creates:

| IDE               | Generated files                               |
|-------------------|-----------------------------------------------|
| Cursor            | `.cursor/rules/gitflow-preflight.mdc`, `.cursor/mcp.json`, `.agents/skills/gitflow/SKILL.md` |
| VS Code + Copilot | `.github/copilot-instructions.md`, `.vscode/mcp.json`, `.agents/skills/gitflow/SKILL.md` |
| Claude Code / Windsurf / Cline | IDE-specific rule file, IDE MCP config, `.agents/skills/gitflow/SKILL.md` |
| Zed / Neovim / JetBrains / Unknown | IDE-specific files if applicable, `AGENTS.md`, `~/.agents/skills/gitflow/SKILL.md` |

These files instruct the AI agent to run `gitflow --json status` before modifying any code, and the embedded skill is auto-updated if its content changes in newer gitflow binaries.

### What AGENTS.md Is For

`AGENTS.md` is a repository-level instruction file for AI coding agents. It defines mandatory workflow rules (for example, gitflow pre-flight checks, branch routing, and safety constraints) that the agent should follow before making code changes.

`gitflow setup` generates `AGENTS.md` when an IDE/agent does not use a dedicated rules format as its primary instruction source, or when a generic fallback instruction file is needed.

### IDEs/Agents That Use AGENTS.md

In this project, `AGENTS.md` is primarily used (or used as fallback) by these environments:

- Zed
- Neovim-based agent setups
- JetBrains-based agent setups
- Unknown or generic agent-compatible IDEs

For Cursor and VS Code Copilot, the primary files are `.cursor/rules/gitflow-preflight.mdc` and `.github/copilot-instructions.md` respectively, while `AGENTS.md` remains the compatibility fallback for other tools.

### Copilot-Specific Notes

To ensure the embedded gitflow skill works in Copilot end-to-end:

1. Install `gitflow` binary and make sure it is in PATH.
2. Run `gitflow setup --ide copilot` in your repository root.
3. Verify these files exist:
  - `.github/copilot-instructions.md`
  - `.vscode/mcp.json` (contains `"gitflow"` server using `gitflow serve`)
  - `.agents/skills/gitflow/SKILL.md`

If `gitflow` is not in PATH when setup runs, MCP config still gets generated, but command execution will fail until PATH is fixed.

## Embedded Skill

`gitflow setup` now installs the gitflow skill from the binary itself.

- Project-local install for supported IDEs: `.agents/skills/gitflow/SKILL.md`
- User-level fallback for unsupported IDEs: `~/.agents/skills/gitflow/SKILL.md`

If the embedded skill content changes in a newer binary, `gitflow setup` updates the installed `SKILL.md` automatically.

## Configuration

Create a `.gitflow/config.json` in your project root for custom settings:

```json
{
  "remote": "origin",
  "main_branch": "main",
  "develop_branch": "develop",
  "integration_mode": "local-merge",
  "version_file": "package.json",
  "version_pattern": "\"version\"\\s*:\\s*\"([^\"]+)\"",
  "tag_prefix": "v",
  "bump_command": "npm version {part} --no-git-tag-version",
  "ai_integration": {
    "enabled": true,
    "version": "0.5.28"
  }
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
make release-local-github             # upload local artifacts to the latest existing GitHub release tag
make publish-github TAG=v0.5.12       # create/update GitHub release and upload local artifacts
make publish-homebrew TAG=v0.5.12     # upload artifacts and sync ../homebrew-tap/Formula (also updates packaging/homebrew on main/release/hotfix)
make publish-winget TAG=v0.5.12       # upload artifacts, then update Winget version/installer/defaultLocale manifests
make push-winget TAG=v0.5.12          # submit/update Winget package in microsoft/winget-pkgs via wingetcreate
make publish-all TAG=v0.5.12          # upload once and update all package manifests
make install      # install to GOPATH/bin
```

## Testing & Coverage

Coverage profiles generated by the Makefile are written into the `test/` directory.

- Run all tests and produce an aggregated coverage profile:

```bash
make cover
# writes `test/coverage.out`
```

- Run tests for a single package and produce a package-specific profile:

```bash
make cover-package PKG=./internal/commands
# writes `test/commands.cov`
```

Keep `test/` out of releases and CI artifacts; these files are intended for local inspection and CI coverage steps.

## Local-Only Release Policy

This repository does not use GitHub Actions to compile binaries.

- All release binaries are built locally on maintainer machines.
- GitHub Releases are used only as artifact hosting/distribution.
- Homebrew and Winget manifests point to those GitHub Release artifacts.

### Publish Flow (No Cloud Build)

```bash
# Upload local artifacts to the latest existing release tag
make release-local-github

# Create/update the GitHub release and upload locally-built binaries
make publish-github TAG=v0.5.12

# Refresh package manifests to point at that GitHub release.
# Each target now depends on publish-github, so artifacts are uploaded first.
make publish-homebrew TAG=v0.5.12
make publish-winget TAG=v0.5.12
make push-winget TAG=v0.5.12

# Note: when run from develop/feature/bugfix branches, publish-homebrew keeps
# tracked release metadata protected and only syncs ../homebrew-tap/Formula.

# Or do everything in one shot
make publish-all TAG=v0.5.12
```

## License

MIT. See [LICENSE](LICENSE).

## Author

Luis Lozano
