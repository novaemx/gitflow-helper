package ide

import (
	"os"
	"path/filepath"
)

func jetbrainsRulePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".idea", "gitflow-instructions.md")
}

func jetbrainsRuleExists(projectRoot string) bool {
	_, err := os.Stat(jetbrainsRulePath(projectRoot))
	return err == nil
}

func generateJetBrainsRule(projectRoot string) (string, error) {
	return ensureFileWithGitflow(jetbrainsRulePath(projectRoot), "# Gitflow Instructions for JetBrains\n\n", "full")
}
