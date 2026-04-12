package ide

import (
	"os"
	"path/filepath"
	"strings"
)

const agentsSection = `
## Gitflow Enforcement

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

### Full CLI

` + "```" + `
gitflow --json status|pull|init|sync|switch|backmerge|cleanup|health|doctor|log|undo|releasenotes|finish
gitflow --json start feature|bugfix|release|hotfix <name>
gitflow setup [--ide cursor|copilot|both]
` + "```" + `
`

func generateAgentsMD(projectRoot string) (string, error) {
	path := filepath.Join(projectRoot, "AGENTS.md")

	existing, err := os.ReadFile(path)
	if err == nil {
		content := string(existing)
		if strings.Contains(content, "Gitflow Enforcement") {
			return path, nil
		}
		content += "\n" + agentsSection
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", err
		}
		return path, nil
	}

	content := "# Agent Instructions\n" + agentsSection
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}
