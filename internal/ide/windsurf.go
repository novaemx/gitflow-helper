package ide

import (
	"os"
	"path/filepath"
)

func windsurfRulePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".windsurf", "rules", "gitflow-preflight.md")
}

func windsurfRuleExists(projectRoot string) bool {
	if _, err := os.Stat(windsurfRulePath(projectRoot)); err == nil {
		return true
	}
	return fileContainsGitflow(filepath.Join(projectRoot, ".windsurfrules"))
}

func generateWindsurfRule(projectRoot string) (string, error) {
	return ensureFileWithGitflow(windsurfRulePath(projectRoot), "# Gitflow Pre-flight Check\n\n", "full")
}
