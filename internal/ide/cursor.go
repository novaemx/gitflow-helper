package ide

import (
	"os"
	"path/filepath"
)

const cursorRuleContent = `---
description: >-
  MANDATORY pre-flight check before ANY code modification. Enforces gitflow
  discipline throughout the entire development cycle.
alwaysApply: true
---

# Gitflow Pre-flight Check

**Before modifying ANY code, run the pre-flight check.**

## Step 1 — Run status (EVERY TIME before code changes)

` + "```bash" + `
gitflow --json status
` + "```" + `

Evaluate the JSON response in order:

### 1a. Is git-flow initialized?
If ` + "`git_flow_initialized`" + ` is false:
` + "```bash" + `
gitflow --json init
` + "```" + `

### 1b. Is there a merge conflict?
If ` + "`merge.in_merge`" + ` is true → **STOP.** Report to user.

### 1c. Is there branch divergence?
If ` + "`main_ahead_of_develop > 0`" + ` → **STOP.** Fix immediately:
` + "```bash" + `
gitflow --json backmerge
` + "```" + `

### 1d. Are we on the right branch?

| User wants to...          | Correct branch           | If wrong, run                              |
|---------------------------|-------------------------|--------------------------------------------|
| Add a new feature         | feature/* or develop     | ` + "`gitflow --json switch develop`" + `, then ` + "`gitflow --json start feature <name>`" + ` |
| Fix a bug (non-urgent)    | bugfix/* or develop      | ` + "`gitflow --json switch develop`" + `, then ` + "`gitflow --json start bugfix <name>`" + `  |
| Fix a production bug      | hotfix/* or main         | ` + "`gitflow --json switch main`" + `, then ` + "`gitflow --json start hotfix <ver>`" + `     |
| Prepare a release         | release/* or develop     | ` + "`gitflow --json switch develop`" + `, then ` + "`gitflow --json start release <ver>`" + `  |

**NEVER modify code on main. NEVER commit directly to develop.**

### 1e. Ask user intent
If no flow branch exists, ask the user:
1. Feature — new functionality
2. Bugfix — fix a non-urgent bug
3. Hotfix — urgent fix for production
4. Continue — already on the right branch

### 1f. Only NOW proceed with code changes

## Step 2 — During Development
` + "```bash" + `
gitflow --json sync    # sync with parent
gitflow --json pull    # pull before pushing
` + "```" + `

## Step 3 — Finishing Work
` + "```bash" + `
gitflow --json finish
` + "```" + `

## CLI Reference
` + "```bash" + `
gitflow --json status
gitflow --json pull
gitflow --json start feature my-feature
gitflow --json start bugfix fix-name
gitflow --json start release 1.2.0
gitflow --json start hotfix 1.1.1
gitflow --json finish
gitflow --json sync
gitflow --json switch develop
gitflow --json backmerge
gitflow --json cleanup
gitflow --json health
gitflow --json doctor
gitflow --json log -n 20
gitflow --json undo
gitflow --json releasenotes
gitflow --json init
` + "```" + `

Exit codes: 0 success, 1 error, 2 conflict needing human intervention.
`

func generateCursorRule(projectRoot string) (string, error) {
	dir := filepath.Join(projectRoot, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "gitflow-preflight.mdc")
	if err := os.WriteFile(path, []byte(cursorRuleContent), 0644); err != nil {
		return "", err
	}
	return path, nil
}
