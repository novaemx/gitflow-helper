---
name: gitflow
description: >-
  MANDATORY pre-flight check before ANY code modification. Analyzes git state,
  ensures correct branch, asks user intent (feature/bugfix/hotfix/release),
  and enforces gitflow discipline throughout the entire development cycle.
  Triggers on: any task that will modify code, gitflow, branching, feature,
  bugfix, hotfix, release, pull, merge, or branch workflow.
---

# Git Flow Skill

## CRITICAL: This skill activates BEFORE any code is modified

**Every time the user asks you to write, modify, fix, refactor, or delete code,
you MUST run the pre-flight check below FIRST.** Do not touch a single file
until you have confirmed the gitflow state is correct and the user has chosen
a workflow.

---

## Step 0 — Choose transport: MCP or CLI

### 0a. Check if gitflow MCP tools are available

If you are running inside an IDE with MCP support (Cursor, Claude Code,
VS Code + Copilot, Windsurf), check whether the `gitflow` MCP server tools
are registered. MCP tools have these names: `status`, `init`, `pull`, `sync`,
`switch`, `backmerge`, `cleanup`, `health`, `doctor`, `log`, `undo`,
`releasenotes`, `start`, `finish`.

**If MCP tools are available → use them for all gitflow operations.**
MCP is preferred because it returns structured JSON directly without
spawning shell processes, and the interactive TUI shows MCP activity
in real time.

### 0b. Fall back to CLI

If MCP tools are NOT available (running in an external terminal, or
IDE does not support MCP), verify the CLI binary exists:

```bash
command -v gitflow && echo "available" || echo "not found"
```

If not found, install via `make install` from the project root, or
download from GitHub releases. Then use `gitflow --json <command>`
for all operations.

The binary requires **only `git`** — no git-flow extensions needed. It
implements the full nvie gitflow model using raw git commands.

---

## Step 1 — MANDATORY Pre-flight Check (run before ANY code change)

Before writing a single line of code, execute this analysis **every time**:

```bash
gitflow --json status
```

Then evaluate the JSON response in this exact order:

### 1a. Is git-flow initialized?

If `git_flow_initialized` is `false`:

```bash
gitflow --json init
```

### 1b. Is there a merge conflict?

If `merge.in_merge` is `true` → **STOP.** Report the conflict to the user.
Do not modify any code. The user must resolve conflicts first.

### 1c. Is there branch divergence?

If `main_ahead_of_develop > 0` → **STOP all other work.** Fix immediately:

```bash
gitflow --json backmerge
```

This is a gitflow invariant violation. Nothing else should happen until
develop contains all of main.

### 1d. Are we on the right branch?

Evaluate `current` (the active branch) against the user's intent:

| User wants to...          | Correct branch                | If wrong, run                              |
|---------------------------|-------------------------------|--------------------------------------------|
| Add a new feature         | `feature/*` or `develop`      | `switch develop`, then `start feature`     |
| Fix a bug (non-urgent)    | `bugfix/*` or `develop`       | `switch develop`, then `start bugfix`      |
| Fix a production bug      | `hotfix/*` or `main`          | `switch main`, then `start hotfix`         |
| Prepare a release         | `release/*` or `develop`      | `switch develop`, then `start release`     |
| Continue existing work    | The correct `feature/bugfix/hotfix/release` branch | `switch <branch>` |

**If the user is on `main` and wants to write feature/bugfix code → NEVER
modify code on main. Switch to develop or a feature branch first.**

**If the user is on `develop` and wants to write code → ask them to create a
feature or bugfix branch first. Do not commit directly to develop.**

### 1e. Ask the user what they want to do

If no flow branch exists for the user's task, **ask the user** before creating
one. Present the options clearly:

> "Before I make changes, I need to set up the correct git branch.
> What type of work is this?
> 1. **Feature** — new functionality
> 2. **Bugfix** — fix a non-urgent bug on develop
> 3. **Hotfix** — urgent fix for production (main)
> 4. **Continue** — I'm already on the right branch"

Then execute the appropriate `start` command:

```bash
gitflow --json start feature <name>
gitflow --json start bugfix <name>
gitflow --json start hotfix <version>
```

### 1f. Are there uncommitted changes?

If `dirty` is `true` and we need to switch branches, the tool handles
auto-stashing. But warn the user if they have uncommitted work that might
belong to a different task.

### 1g. Only NOW proceed with code changes

Once all checks pass and you are on the correct branch, you may begin
modifying code.

---

## Step 2 — During Development

While making code changes on the flow branch:

- **Sync regularly** if the branch is long-lived:
  ```bash
  gitflow --json sync
  ```
- **Pull before pushing** to avoid conflicts:
  ```bash
  gitflow --json pull
  ```

---

## Step 3 — Finishing Work

When the user's task is complete and code is committed:

```bash
gitflow --json finish
```

For releases, this automatically generates release notes.

---

## Step 4 — Release Notes (automatic on release finish)

When finishing a release, the tool generates a `RELEASE_NOTES.md` file:

```bash
gitflow --json releasenotes           # from last tag to HEAD
gitflow --json releasenotes v0.5.1    # from specific tag
```

---

## Canonical Gitflow Model

Based on [nvie](https://nvie.com/posts/a-successful-git-branching-model/) and
[Atlassian](https://www.atlassian.com/git/tutorials/comparing-workflows/gitflow-workflow).

### Permanent branches

| Branch    | Purpose                                                    |
|-----------|------------------------------------------------------------|
| `main`    | Production-ready code. Every merge here IS a release.      |
| `develop` | Integration branch. Latest delivered development changes.  |

### Supporting branches

| Type      | From      | Merges into               | Naming           |
|-----------|-----------|---------------------------|------------------|
| `feature` | `develop` | `develop`                 | `feature/*`      |
| `bugfix`  | `develop` | `develop`                 | `bugfix/*`       |
| `release` | `develop` | `main` AND `develop`      | `release/*`      |
| `hotfix`  | `main`    | `main` AND `develop`*     | `hotfix/*`       |

*Hotfix exception: if a release branch exists, hotfix merges into the
release branch instead of develop.

### Guardrails

- **Always `--no-ff`** on merges.
- **Tag every merge into main.**
- **Never commit directly to `main` or `develop`** — use flow branches.
- **No new features during an active release.**
- **develop must always be a superset of main.**

---

## CLI Reference (always use `--json` for agent mode)

```bash
gitflow --json status                     # repo state
gitflow --json pull                       # safe fetch + merge
gitflow --json start feature my-feature   # start feature
gitflow --json start bugfix fix-name      # start bugfix
gitflow --json start release 1.2.0        # start release
gitflow --json start hotfix 1.1.1         # start hotfix
gitflow --json finish                     # finish current branch
gitflow --json sync                       # sync with parent
gitflow --json switch develop             # switch branch
gitflow --json backmerge                  # merge main→develop
gitflow --json cleanup                    # delete merged branches
gitflow --json health                     # full repo audit
gitflow --json doctor                     # validate prerequisites
gitflow --json log -n 20                  # gitflow commit log
gitflow --json undo                       # undo last operation
gitflow --json releasenotes               # generate release notes
gitflow --json init                       # initialize git-flow
gitflow serve                             # start MCP server (stdio)
gitflow setup                             # detect IDE & generate rules
gitflow setup --ide cursor                # force Cursor rules
gitflow setup --ide copilot               # force Copilot instructions
```

Exit codes: `0` success, `1` error, `2` conflict needing human intervention.

## MCP Tool Reference (when MCP server is available)

All 14 tools return JSON. Use MCP tools when available instead of CLI.

| MCP Tool       | Parameters                                 | Equivalent CLI                    |
|----------------|--------------------------------------------|------------------------------------|
| `status`       | (none)                                     | `gitflow --json status`            |
| `init`         | (none)                                     | `gitflow --json init`              |
| `pull`         | (none)                                     | `gitflow --json pull`              |
| `sync`         | (none)                                     | `gitflow --json sync`              |
| `backmerge`    | (none)                                     | `gitflow --json backmerge`         |
| `cleanup`      | (none)                                     | `gitflow --json cleanup`           |
| `health`       | (none)                                     | `gitflow --json health`            |
| `doctor`       | (none)                                     | `gitflow --json doctor`            |
| `log`          | `{"count": 20}`                            | `gitflow --json log -n 20`         |
| `undo`         | (none)                                     | `gitflow --json undo`              |
| `releasenotes` | `{"from_tag": "v0.5.1"}` (optional)        | `gitflow --json releasenotes`      |
| `start`        | `{"type": "feature", "name": "my-feat"}`   | `gitflow --json start feature ...` |
| `finish`       | `{"name": ""}` (optional)                  | `gitflow --json finish`            |
| `switch`       | `{"branch": "develop"}`                    | `gitflow --json switch develop`    |

---

## IDE Setup

Run `gitflow setup` to auto-detect your IDE and generate the appropriate
pre-flight enforcement rules **and MCP configuration**:

- **Cursor**: Creates `.cursor/rules/gitflow-preflight.mdc` + `.cursor/mcp.json`
- **VSCode / Copilot**: Creates `.github/copilot-instructions.md` + `.vscode/mcp.json`
- **Claude Code**: Creates `CLAUDE.md` + `.claude/mcp.json`
- **Windsurf**: Creates `.windsurfrules` + `.windsurf/mcp.json`
- **Both/Unknown**: Creates all of the above plus `AGENTS.md`

MCP config is auto-generated only when `gitflow` is in PATH. The config
points to `gitflow serve` which starts the MCP server over stdio.

Detected IDEs (shown in TUI title bar): Cursor, VS Code, VS Code + Copilot,
Claude Code, Windsurf, Cline, Zed, Neovim, JetBrains.

---

## Interactive TUI Mode (for humans)

Run `gitflow` without arguments to launch the full-screen TUI with:

- Title bar with project name, branch, version, tag, dirty indicator, and detected IDE
- Dashboard panel with phase analysis and in-flight branches
- Action menu with highlighted selection and recommended markers
- Command palette (`/` to search), help (`?`), and refresh (`r`)

---

## Agent Decision Flowchart

```
 USER ASKS TO MODIFY CODE
          │
          ▼
 ┌─── Run `gitflow --json status` ───┐
 │                                     │
 ▼                                     ▼
 git_flow_initialized?              merge conflict?
 NO → run `init`                    YES → STOP, report to user
 │                                     │
 ▼                                     ▼
 main_ahead_of_develop?             On correct branch?
 YES → run `backmerge`              NO → `switch` to correct branch
 │                                     │
 ▼                                     ▼
 On a flow branch?                  Has dirty state?
 NO → ask user intent               YES → warn about uncommitted changes
      then `start`
 │
 ▼
 ✅ NOW safe to modify code
 │
 ▼ (when done)
 `finish` → for releases, generates RELEASE_NOTES.md
```

---

## Configuration

The tool reads `.gitflow.json` in the project root for project-specific
settings (version file, bump commands, branch names). If absent, it
auto-detects common patterns and uses git tags for versioning.
