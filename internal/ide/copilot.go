package ide

import (
	"path/filepath"
)

func copilotPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".github", "copilot-instructions.md")
}

func copilotRuleExists(projectRoot string) bool {
	return fileContainsGitflow(copilotPath(projectRoot))
}

func generateCopilotInstructions(projectRoot string) (string, error) {
	return ensureFileWithGitflow(copilotPath(projectRoot), "# Copilot Instructions\n\n", "compact")
}
