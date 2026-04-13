package ide

import (
	"os"
	"path/filepath"
)

func zedRulePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".zed", "gitflow-instructions.md")
}

func zedRuleExists(projectRoot string) bool {
	_, err := os.Stat(zedRulePath(projectRoot))
	return err == nil
}

func generateZedRule(projectRoot string) (string, error) {
	return ensureFileWithGitflow(zedRulePath(projectRoot), "# Gitflow Instructions for Zed\n\n", "full")
}
