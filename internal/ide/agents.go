package ide

import (
	"path/filepath"
)

func agentsPath(projectRoot string) string {
	return filepath.Join(projectRoot, "AGENTS.md")
}

func agentsRuleExists(projectRoot string) bool {
	return fileContainsGitflow(agentsPath(projectRoot))
}

func generateAgentsMD(projectRoot string) (string, error) {
	return ensureFileWithGitflow(agentsPath(projectRoot), "# Agent Instructions\n\n", "compact")
}
