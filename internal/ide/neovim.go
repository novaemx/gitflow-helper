package ide

import (
	"os"
	"path/filepath"
)

func neovimRulePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".nvim", "gitflow-instructions.md")
}

func neovimRuleExists(projectRoot string) bool {
	_, err := os.Stat(neovimRulePath(projectRoot))
	return err == nil
}

func generateNeovimRule(projectRoot string) (string, error) {
	return ensureFileWithGitflow(neovimRulePath(projectRoot), "# Gitflow Instructions for Neovim\n\n", "full")
}
