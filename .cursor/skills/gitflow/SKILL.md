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

## Step 0 — Verify the helper exists

```bash
test -f scripts/gitflow.py && echo "available" || echo "not found"
```

If not found, fall back to manual `git` commands but still follow the
pre-flight logic described below.

The script requires only Python 3 (any version) and `git` with `git-flow`.
No venv or third-party packages needed.

---

## Step 1 — MANDATORY Pre-flight Check (run before ANY code change)

Before writing a single line of code, execute this analysis **every time**:

```bash
python3 scripts/gitflow.py --json status
```

Then evaluate the JSON response in this exact order:

### 1a. Is git-flow initialized?

If `git_flow_initialized` is `false`:

```bash
python3 scripts/gitflow.py --json init
```

### 1b. Is there a merge conflict?

If `merge.in_merge` is `true` → **STOP.** Report the conflict to the user.
Do not modify any code. The user must resolve conflicts first.

### 1c. Is there branch divergence?

If `main_ahead_of_develop > 0` → **STOP all other work.** Fix immediately:

```bash
python3 scripts/gitflow.py --json backmerge
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
python3 scripts/gitflow.py --json start feature <name>
python3 scripts/gitflow.py --json start bugfix <name>
python3 scripts/gitflow.py --json start hotfix <version>
```

### 1f. Are there uncommitted changes?

If `dirty` is `true` and we need to switch branches, the script handles
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
  python3 scripts/gitflow.py --json sync
  ```
- **Pull before pushing** to avoid conflicts:
  ```bash
  python3 scripts/gitflow.py --json pull
  ```

---

## Step 3 — Finishing Work

When the user's task is complete and code is committed:

```bash
python3 scripts/gitflow.py --json finish
```

For releases, this automatically generates release notes (see below).

---

## Step 4 — Release Notes (automatic on release finish)

When finishing a release, the script generates a `RELEASE_NOTES.md` file:

```bash
python3 scripts/gitflow.py --json releasenotes           # from last tag to HEAD
python3 scripts/gitflow.py --json releasenotes v0.5.1    # from specific tag
```

- Collects all commits between the previous release tag and now.
- Groups them by type: features, fixes, improvements, other.
- Writes a user-facing markdown file focused on what matters to end users
  (no commit hashes, no file paths, no internal jargon).
- Outputs the content both to the file and to stdout (JSON includes `content`
  and `file` fields).
- The file is created at the project root as `RELEASE_NOTES.md`.

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
python3 scripts/gitflow.py --json status                     # repo state
python3 scripts/gitflow.py --json pull                       # safe fetch + merge
python3 scripts/gitflow.py --json start feature my-feature   # start feature
python3 scripts/gitflow.py --json start bugfix fix-name      # start bugfix
python3 scripts/gitflow.py --json start release 1.2.0        # start release
python3 scripts/gitflow.py --json start hotfix 1.1.1         # start hotfix
python3 scripts/gitflow.py --json finish                     # finish current branch
python3 scripts/gitflow.py --json sync                       # sync with parent
python3 scripts/gitflow.py --json switch develop             # switch branch
python3 scripts/gitflow.py --json backmerge                  # merge main→develop
python3 scripts/gitflow.py --json cleanup                    # delete merged branches
python3 scripts/gitflow.py --json health                     # full repo audit
python3 scripts/gitflow.py --json doctor                     # validate prerequisites
python3 scripts/gitflow.py --json log -n 20                  # gitflow commit log
python3 scripts/gitflow.py --json undo                       # undo last operation
python3 scripts/gitflow.py --json releasenotes               # generate release notes
python3 scripts/gitflow.py --json init                       # initialize git-flow
```

Exit codes: `0` success, `1` error, `2` conflict needing human intervention.

---

## Interactive TUI Mode (for humans)

When the user wants to manage git-flow interactively, run without `--json`:

```bash
python3 scripts/gitflow.py
```

This launches a full-screen TUI (inspired by OpenCode) with:

- **Title bar**: project name, current branch (color-coded), version, tag, dirty indicator
- **Dashboard panel**: scrollable phase analysis, in-flight branches, divergence warnings
- **Action menu**: selectable actions with highlighted selection and recommended markers
- **Status bar**: key hints at the bottom row
- **Overlay dialogs**: confirmation, input, help, command palette, diff viewer

### Key Bindings

| Key | Action |
|-----|--------|
| `j` / `Down` | Move selection down |
| `k` / `Up` | Move selection up |
| `g` / `Home` | Jump to first item |
| `G` / `End` | Jump to last item |
| `Enter` | Execute selected action |
| `/` | Open command palette (type to filter) |
| `?` | Toggle help overlay |
| `r` | Refresh dashboard |
| `q` / `Ctrl+C` | Quit |

### Backend

The TUI uses Python's `curses` stdlib (macOS/Linux) for full-screen rendering.
On systems without curses (Windows), it falls back to ANSI escape codes with
raw terminal input -- same UX, slightly reduced visual fidelity.

When an action executes (e.g., `git flow feature start`), the TUI temporarily
yields the screen so git output is visible, then resumes on Enter.

---

## Agent Decision Flowchart

```
 USER ASKS TO MODIFY CODE
          │
          ▼
 ┌─── Run `status --json` ───┐
 │                            │
 ▼                            ▼
 git_flow_initialized?     merge conflict?
 NO → run `init`           YES → STOP, report to user
 │                            │
 ▼                            ▼
 main_ahead_of_develop?    On correct branch?
 YES → run `backmerge`     NO → `switch` to correct branch
 │                            │
 ▼                            ▼
 On a flow branch?         Has dirty state?
 NO → ask user intent      YES → warn about uncommitted changes
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

The script reads `.gitflow.json` in the project root for project-specific
settings (version file, bump commands, branch names). If absent, it
auto-detects common patterns and uses git tags for versioning.
