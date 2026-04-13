# Copilot Instructions

## Skills As Rules (Load Only When Needed)

Use skills from `.agents/skills/` as contextual rules. Do not preload all skills.

### Rule Modes

- Always apply: Use only for mandatory workflows.
- Intelligent apply: Default mode. Load when the user request clearly matches a skill `description` and trigger phrases.
- File-scoped apply: Load when the task targets files or areas explicitly covered by a skill.
- Manual apply: Load when the user explicitly asks for a skill by name.

### Required Always-Apply Rule

- `gitflow` skill is mandatory as preflight before any code or tracked-file modification.

### Skill Selection Procedure

1. Identify task intent from the user request.
2. Match intent against skill `description` text and trigger phrases.
3. Load only the minimal set of relevant skills.
4. If no skill clearly applies, continue without loading extra skills.
5. If multiple skills overlap, keep only the smallest set required to complete the task.

### Best Practices

- Keep skill usage focused and composable.
- Prefer specific skills over broad always-on behavior to protect context window.
- Avoid loading skills for rare edge cases unless the user request requires them.
- Revisit skill selection when task scope changes during execution.
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
