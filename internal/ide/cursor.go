package ide

import (
	"os"
	"path/filepath"
)

const cursorRuleFrontmatter = `---
description: >-
  MANDATORY pre-flight check before ANY code modification. Enforces gitflow
  discipline throughout the entire development cycle.
alwaysApply: true
---

`

func cursorRulePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".cursor", "rules", "gitflow-preflight.mdc")
}

func cursorRuleExists(projectRoot string) bool {
	_, err := os.Stat(cursorRulePath(projectRoot))
	return err == nil
}

func generateCursorRule(projectRoot string) (string, error) {
	dir := filepath.Join(projectRoot, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := cursorRulePath(projectRoot)
	content := withVersionHeaderFrontmatter(cursorRuleFrontmatter) + GitflowInstructionsFull
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}
