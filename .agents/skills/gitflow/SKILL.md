---
name: gitflow
description: >-
  MANDATORY pre-flight check before ANY code modification. Enforces gitflow,
  auto-detects task type, auto-generates branch names, and forbids direct code
  edits on main/develop.
---

# Git Flow Skill

## CRITICAL

Run pre-flight before any code/track-file change.

```bash
gitflow --json status
```

Do not edit files until branch workflow is valid.

## Step 1 - Pre-flight order

1. If `git_flow_initialized=false` -> `gitflow --json init`
2. If `merge.in_merge=true` -> STOP and report conflict
3. If `main_ahead_of_develop>0` -> run `gitflow --json backmerge` first
4. If current branch is `main` or `develop` and task modifies code -> create flow branch first

## Step 2 - Branch policy (strict)

Never modify code on `main` or `develop`.

Required coding branches:

- Feature work -> `feature/*`
- Non-urgent bug fix -> `bugfix/*`
- Production urgent fix -> `hotfix/*`
- Release prep -> `release/*`

## Step 3 - Auto type inference

Infer task type from user request:

- release: words like release, cut release, tag release
- hotfix: words like prod, production, outage, urgent, critical fix
- bugfix: words like bug, fix, error, regression
- feature: default for enhancement/refactor/new capability

Only ask user when intent is truly ambiguous between bugfix vs hotfix.

## Step 4 - Auto branch naming (no prompt)

Do not ask user how to name branch unless user explicitly requests custom name.

Feature/Bugfix naming:

- Build slug from user request text
- Lowercase
- Convert spaces/underscores to `-`
- Keep `a-z`, `0-9`, `-`
- Remove common stopwords (`the`, `a`, `an`, `to`, `for`, `and`, `or`, `de`, `la`, `el`)
- Keep first 5 meaningful tokens
- Max 48 chars
- Fallback: `auto-YYYYMMDD-HHMM`

Hotfix/Release naming:

- If version provided, use it
- Else use `auto`

Commands:

```bash
gitflow --json start feature <auto-name>
gitflow --json start bugfix <auto-name>
gitflow --json start hotfix <version-or-auto>
gitflow --json start release <version-or-auto>
```

## Step 5 - Work and finish

During work:

```bash
gitflow --json sync
gitflow --json pull
```

Finish when done:

```bash
gitflow --json finish
```

## Guardrails

- Never commit directly on `main`/`develop`
- Never bypass backmerge when `main_ahead_of_develop>0`
- Always use `--json` in agent mode
- Exit codes: `0` success, `1` error, `2` conflict-needs-human
