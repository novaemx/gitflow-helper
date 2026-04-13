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