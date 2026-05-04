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

// cursorRuleContent returns the full content that generateCursorRule would
// write for the running version. Used for content-equality checks.
func cursorRuleContent() string {
	return withVersionHeaderFrontmatter(cursorRuleFrontmatter) + GitflowInstructionsFull
}

func generateCursorRule(projectRoot string) (string, error) {
	dir := filepath.Join(projectRoot, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := cursorRulePath(projectRoot)
	content := cursorRuleContent()
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		return "", nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}
