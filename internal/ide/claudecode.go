package ide

import (
	"path/filepath"
)

func claudeCodePath(projectRoot string) string {
	return filepath.Join(projectRoot, "CLAUDE.md")
}

func claudeCodeRuleExists(projectRoot string) bool {
	return fileContainsGitflow(claudeCodePath(projectRoot))
}

func generateClaudeCodeRule(projectRoot string) (string, error) {
	return ensureFileWithGitflow(claudeCodePath(projectRoot), "# CLAUDE.md\n\n", "full")
}
