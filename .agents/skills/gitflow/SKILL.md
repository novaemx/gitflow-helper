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

### 0b. Fall back to CLI

If MCP tools are NOT available, verify the CLI binary exists:

```bash
command -v gitflow && echo "available" || echo "not found"
```

If not found, install via `make install` from the project root, or
download from GitHub releases. Then use `gitflow --json <command>`.

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

### 1c. Is there branch divergence?

If `main_ahead_of_develop > 0` → **STOP all other work.** Fix immediately:

```bash
gitflow --json backmerge
```

### 1d. Are we on the right branch?

| User wants to...          | Correct branch                | If wrong, run                              |
|---------------------------|-------------------------------|--------------------------------------------|
| Add a new feature         | `feature/*` or `develop`      | `switch develop`, then `start feature`     |
| Fix a bug (non-urgent)    | `bugfix/*` or `develop`       | `switch develop`, then `start bugfix`      |
| Fix a production bug      | `hotfix/*` or `main`          | `switch main`, then `start hotfix`         |
| Prepare a release         | `release/*` or `develop`      | `switch develop`, then `start release`     |

**NEVER modify code on main. NEVER commit directly to develop.**

### 1e. Only NOW proceed with code changes

---

## Step 2 — During Development

```bash
gitflow --json sync
gitflow --json pull
```

## Step 3 — Finishing Work

```bash
gitflow --json finish
```

## IDE Setup

Run `gitflow setup` to install or update the embedded gitflow skill.

- Project-capable IDEs: `.agents/skills/gitflow/SKILL.md`
- Fallback location: `~/.agents/skills/gitflow/SKILL.md`

## CLI Reference

```bash
gitflow --json status|pull|init|sync|switch|backmerge|cleanup|health|doctor|log|undo|releasenotes|finish
gitflow --json start feature|bugfix|release|hotfix <name>
gitflow setup
```
