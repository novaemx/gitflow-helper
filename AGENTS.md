# Agent Instructions

## Gitflow Enforcement

**Before modifying ANY code, run the gitflow pre-flight check.**

```bash
gitflow --json status
```

### Pre-flight sequence

1. Check `git_flow_initialized` → if false, run `gitflow --json init`
2. Check `merge.in_merge` → if true, STOP and report to user
3. Check `main_ahead_of_develop` → if > 0, run `gitflow --json backmerge`
4. Ensure you are on the correct branch for the task type
5. NEVER modify code on main or develop directly — use flow branches
6. When done: `gitflow --json finish`

### Branch routing

| Task type    | Start command                                |
|-------------|----------------------------------------------|
| Feature     | `gitflow --json start feature <name>`      |
| Bugfix      | `gitflow --json start bugfix <name>`       |
| Hotfix      | `gitflow --json start hotfix <version>`    |
| Release     | `gitflow --json start release <version>`   |

### Full CLI

```
gitflow --json status|pull|init|sync|switch|backmerge|cleanup|health|doctor|log|undo|releasenotes|finish
gitflow --json start feature|bugfix|release|hotfix <name>
```

Exit codes: 0=success, 1=error, 2=conflict-needs-human
