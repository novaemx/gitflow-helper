package ide

import (
	"os"
	"path/filepath"
	"strings"
)

const copilotSkillGuidanceMarker = "When to use the gitflow skill"

const copilotSkillGuidance = `
### When to use the gitflow skill

- Use the gitflow skill before any code modifications.
- Run ` + "`gitflow --json status`" + ` first and follow its branch/merge checks.
- Use ` + "`gitflow setup`" + ` after updating gitflow to refresh embedded skill content.
`

func copilotPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".github", "copilot-instructions.md")
}

func copilotRuleExists(projectRoot string) bool {
	path := copilotPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	return fileContainsGitflow(path) && strings.Contains(content, copilotSkillGuidanceMarker)
}

func generateCopilotInstructions(projectRoot string) (string, error) {
	path, err := ensureFileWithGitflow(copilotPath(projectRoot), "# Copilot Instructions\n\n", "compact")
	if err != nil {
		return "", err
	}
	if path == "" {
		path = copilotPath(projectRoot)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)
	if strings.Contains(content, copilotSkillGuidanceMarker) {
		return path, nil
	}

	content += "\n" + copilotSkillGuidance
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}
