package ide

// GitflowInstructionsFull is the complete gitflow preflight instruction set,
// reusable across all IDE rule generators. It uses pure markdown (no IDE-specific
// frontmatter) so each generator can wrap it as needed.
const GitflowInstructionsFull = `## Gitflow Pre-flight Check

**MANDATORY: Before modifying ANY code, run the pre-flight check.**

### Step 1 — Run status (EVERY TIME before code changes)

` + "```bash" + `
gitflow --json status
` + "```" + `

Evaluate the JSON response in order:

#### 1a. Is git-flow initialized?
If ` + "`git_flow_initialized`" + ` is false:
` + "```bash" + `
gitflow --json init
` + "```" + `

#### 1b. Is there a merge conflict?
If ` + "`merge.in_merge`" + ` is true → **STOP.** Report to user.

#### 1c. Is there branch divergence?
If ` + "`main_ahead_of_develop > 0`" + ` → **STOP.** Fix immediately:
` + "```bash" + `
gitflow --json backmerge
` + "```" + `

#### 1d. Are we on the right branch?

| User wants to...          | Correct branch           | If wrong, run                              |
|---------------------------|-------------------------|--------------------------------------------|
| Add a new feature         | feature/* or develop     | ` + "`gitflow --json switch develop`" + `, then ` + "`gitflow --json start feature <name>`" + ` |
| Fix a bug (non-urgent)    | bugfix/* or develop      | ` + "`gitflow --json switch develop`" + `, then ` + "`gitflow --json start bugfix <name>`" + `  |
| Fix a production bug      | hotfix/* or main         | ` + "`gitflow --json switch main`" + `, then ` + "`gitflow --json start hotfix <ver>`" + `     |
| Prepare a release         | release/* or develop     | ` + "`gitflow --json switch develop`" + `, then ` + "`gitflow --json start release <ver>`" + `  |

**NEVER modify code on main. NEVER commit directly to develop.**

#### 1e. Ask user intent
If no flow branch exists, ask the user:
1. Feature — new functionality
2. Bugfix — fix a non-urgent bug
3. Hotfix — urgent fix for production
4. Continue — already on the right branch

#### 1f. Only NOW proceed with code changes

### Step 2 — During Development
` + "```bash" + `
gitflow --json sync    # sync with parent
gitflow --json pull    # pull before pushing
` + "```" + `

### Step 3 — Finishing Work
` + "```bash" + `
gitflow --json finish
` + "```" + `

### Skill Activation (Homologated)

- Use the gitflow skill before any code modifications.
- Run ` + "`gitflow --json status`" + ` first and follow branch/merge checks.
- Keep branch routing aligned with the skill decision tree.

### LLM Activity Routing (Command Selection)

Use this mapping to choose commands according to model interaction intent.

| Interaction intent | Primary command(s) | Use when |
|---|---|---|
| Understand current gitflow state | ` + "`gitflow --json status`" + ` | Start of every coding task; validate branch/divergence/merge state |
| Initialize git-flow in repo | ` + "`gitflow --json init`" + ` | ` + "`git_flow_initialized`" + ` is false |
| Resolve main/develop divergence | ` + "`gitflow --json backmerge`" + ` | ` + "`main_ahead_of_develop > 0`" + ` |
| Start feature work | ` + "`gitflow --json start feature <name>`" + ` | New capability or enhancement |
| Start non-urgent bug fix | ` + "`gitflow --json start bugfix <name>`" + ` | Defect that is not production-emergency |
| Start production fix | ` + "`gitflow --json start hotfix <version>`" + ` | Urgent issue in production/main |
| Start release preparation | ` + "`gitflow --json start release <version>`" + ` | Cut release candidate and stabilize |
| Sync active work branch | ` + "`gitflow --json sync`" + ` | Before big changes and before final validation |
| Pull latest updates | ` + "`gitflow --json pull`" + ` | Keep branch current before push/finish |
| Run health diagnostics | ` + "`gitflow --json health`" + `, ` + "`gitflow --json doctor`" + ` | User reports workflow inconsistency/conflicts |
| Review timeline / history | ` + "`gitflow --json log`" + ` | Need operational trace or audit trail |
| Undo last gitflow action | ` + "`gitflow --json undo`" + ` | Last operation was wrong and needs rollback |
| Finish current flow branch | ` + "`gitflow --json finish`" + ` | Tests pass and branch is ready to merge |

### CLI Reference
` + "```" + `
gitflow --json status|pull|init|sync|switch|backmerge|cleanup|health|doctor|log|undo|releasenotes|finish
gitflow --json start feature|bugfix|release|hotfix <name>
gitflow setup [--ide cursor|copilot|both]
` + "```" + `

Exit codes: 0=success, 1=error, 2=conflict-needs-human
`

// GitflowInstructionsCompact is a shorter version for IDEs with limited
// instruction space (e.g. appended sections in existing files).
const GitflowInstructionsCompact = `## Gitflow Enforcement

**Before modifying ANY code, run the gitflow pre-flight check.**

` + "```bash" + `
gitflow --json status
` + "```" + `

### Pre-flight sequence

1. Check ` + "`git_flow_initialized`" + ` → if false, run ` + "`gitflow --json init`" + `
2. Check ` + "`merge.in_merge`" + ` → if true, STOP and report to user
3. Check ` + "`main_ahead_of_develop`" + ` → if > 0, run ` + "`gitflow --json backmerge`" + `
4. Ensure you are on the correct branch for the task type
5. NEVER modify code on main or develop directly — use flow branches
6. When done: ` + "`gitflow --json finish`" + `

### Branch routing

| Task type    | Start command                                |
|-------------|----------------------------------------------|
| Feature     | ` + "`gitflow --json start feature <name>`" + `      |
| Bugfix      | ` + "`gitflow --json start bugfix <name>`" + `       |
| Hotfix      | ` + "`gitflow --json start hotfix <version>`" + `    |
| Release     | ` + "`gitflow --json start release <version>`" + `   |

### Skill Activation (Homologated)

- Use the gitflow skill before any code modifications.
- Always begin with ` + "`gitflow --json status`" + `.
- Keep command selection aligned with task intent and branch type.

### LLM Activity Routing (Compact)

- discovery/state -> ` + "`gitflow --json status`" + `
- branch divergence -> ` + "`gitflow --json backmerge`" + `
- new work -> ` + "`gitflow --json start feature <name>`" + `
- bug fix -> ` + "`gitflow --json start bugfix <name>`" + `
- prod urgent fix -> ` + "`gitflow --json start hotfix <version>`" + `
- release prep -> ` + "`gitflow --json start release <version>`" + `
- branch sync/update -> ` + "`gitflow --json sync`" + ` / ` + "`gitflow --json pull`" + `
- diagnostics -> ` + "`gitflow --json health`" + ` / ` + "`gitflow --json doctor`" + `
- rollback last flow action -> ` + "`gitflow --json undo`" + `
- close flow branch -> ` + "`gitflow --json finish`" + `

### Full CLI

` + "```" + `
gitflow --json status|pull|init|sync|switch|backmerge|cleanup|health|doctor|log|undo|releasenotes|finish
gitflow --json start feature|bugfix|release|hotfix <name>
` + "```" + `

Exit codes: 0=success, 1=error, 2=conflict-needs-human
`
