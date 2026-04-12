package ide

import (
	"path/filepath"
)

func clineRulePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".clinerules")
}

func clineRuleExists(projectRoot string) bool {
	if fileContainsGitflow(clineRulePath(projectRoot)) {
		return true
	}
	return fileContainsGitflow(filepath.Join(projectRoot, ".cline", "instructions.md"))
}

func generateClineRule(projectRoot string) (string, error) {
	return ensureFileWithGitflow(clineRulePath(projectRoot), "# Cline Rules\n\n", "full")
}
