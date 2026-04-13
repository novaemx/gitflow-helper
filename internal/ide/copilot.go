package ide

import (
	"os"
	"path/filepath"
	"strings"
)

const copilotSection = `
## Gitflow Pre-flight Check

**MANDATORY: Before modifying ANY code, run the pre-flight check.**

Before writing a single line of code, execute:

` + "```bash" + `
gitflow --json status
` + "```" + `

Then evaluate the JSON response:

1. If ` + "`git_flow_initialized`" + ` is false → run ` + "`gitflow --json init`" + `
2. If ` + "`merge.in_merge`" + ` is true → STOP, report conflict to user
3. If ` + "`main_ahead_of_develop > 0`" + ` → run ` + "`gitflow --json backmerge`" + ` first
4. Ensure correct branch for the task:
   - Feature: ` + "`gitflow --json start feature <name>`" + `
   - Bugfix: ` + "`gitflow --json start bugfix <name>`" + `
   - Hotfix: ` + "`gitflow --json start hotfix <version>`" + `
   - Release: ` + "`gitflow --json start release <version>`" + `
5. NEVER modify code on main. NEVER commit directly to develop.
6. When done: ` + "`gitflow --json finish`" + `

### CLI Reference

` + "```" + `
gitflow --json status|pull|init|sync|switch|backmerge|cleanup|health|doctor|log|undo|releasenotes|finish
gitflow --json start feature|bugfix|release|hotfix <name>
` + "```" + `

Exit codes: 0=success, 1=error, 2=conflict-needs-human
`

func generateCopilotInstructions(projectRoot string) (string, error) {
	dir := filepath.Join(projectRoot, ".github")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "copilot-instructions.md")

	existing, err := os.ReadFile(path)
	if err == nil {
		content := string(existing)
		if strings.Contains(content, "Gitflow Pre-flight Check") {
			return path, nil
		}
		content += "\n" + copilotSection
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", err
		}
		return path, nil
	}

	content := "# Copilot Instructions\n" + copilotSection
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}
