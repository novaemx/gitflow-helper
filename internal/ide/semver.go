package ide

import (
	"os"
	"path/filepath"
	"strings"
)

// semverConventionalCommitsContent is the conventional commits / semantic
// versioning rule injected into IDEs that support agent rule files.
const semverConventionalCommitsContent = `# Conventional Commits — Semantic Versioning

## Commit Format

` + "```" + `
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
` + "```" + `

## Types

| Type       | SemVer bump | When to use                                   |
|------------|-------------|-----------------------------------------------|
| ` + "`feat`" + `     | MINOR       | New feature                                   |
| ` + "`fix`" + `      | PATCH       | Bug fix                                       |
| ` + "`perf`" + `     | PATCH       | Performance improvement                       |
| ` + "`chore`" + `    | –           | Build tooling, maintenance, dependency bumps  |
| ` + "`docs`" + `     | –           | Documentation only                            |
| ` + "`refactor`" + ` | –           | Code restructure without feature/fix          |
| ` + "`test`" + `     | –           | Adding or updating tests                      |
| ` + "`ci`" + `       | –           | CI/CD configuration                           |
| ` + "`style`" + `    | –           | Formatting, whitespace, semicolons            |
| ` + "`revert`" + `   | depends     | Reverts a previous commit                     |

## Breaking Changes → MAJOR bump

Two equivalent ways:
1. Append ` + "`!`" + ` after type/scope: ` + "`feat!: remove legacy endpoint`" + `
2. Add ` + "`BREAKING CHANGE:`" + ` footer in the commit body

## Rules

- Subject: imperative mood, no trailing period, ≤72 chars
- Body: explain *why*, not *what* — wrap at 72 chars
- Scope: optional noun describing the section (e.g. ` + "`auth`" + `, ` + "`tui`" + `, ` + "`api`" + `)
- Footers: ` + "`Fixes #123`" + `, ` + "`Co-authored-by: Name <email>`" + `, ` + "`BREAKING CHANGE: …`" + `

## Examples

` + "```" + `
feat(auth): add OAuth2 login support

fix(tui): snap overlay close to prevent ghost artifacts

chore: bump version to 1.2.3

feat!: remove deprecated --legacy flag

BREAKING CHANGE: --legacy is removed; use --mode instead
` + "```" + `
`

const semverCopilotMarker = "Conventional Commits — Semantic Versioning"

const semverCopilotSection = `
## Conventional Commits — Semantic Versioning

Always write commit messages in Conventional Commits format:
` + "```" + `
<type>(<scope>): <subject>
` + "```" + `

Types: ` + "`feat`" + ` (MINOR), ` + "`fix`/`perf`" + ` (PATCH), ` + "`feat!`/`fix!`" + ` (MAJOR), ` + "`chore`/`docs`/`refactor`/`test`/`ci`/`style`" + ` (no bump).
Breaking change: append ` + "`!`" + ` or add ` + "`BREAKING CHANGE:`" + ` footer.
Subject: imperative mood, no period, ≤72 chars.
`

// semverCursorRulePath returns the path to the semver Cursor rule file.
func semverCursorRulePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".cursor", "rules", "semver.mdc")
}

// semverCursorRuleExists reports whether the semver Cursor rule already exists.
func semverCursorRuleExists(projectRoot string) bool {
	_, err := os.Stat(semverCursorRulePath(projectRoot))
	return err == nil
}

const semverCursorRuleFrontmatter = `---
description: >-
  Conventional Commits / Semantic Versioning rule. Enforces structured commit
  messages so AI-generated commits trigger the correct SemVer bump.
alwaysApply: true
---

`

// generateSemverCursorRule writes .cursor/rules/semver.mdc with conventional
// commits guidance formatted as a Cursor agent rule.
func generateSemverCursorRule(projectRoot string) (string, error) {
	dir := filepath.Join(projectRoot, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := semverCursorRulePath(projectRoot)
	content := withVersionHeaderFrontmatter(semverCursorRuleFrontmatter) + semverConventionalCommitsContent
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// semverCopilotSectionExists reports whether the copilot instructions file
// already contains the semver section.
func semverCopilotSectionExists(projectRoot string) bool {
	path := copilotPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), semverCopilotMarker)
}

// generateSemverCopilotSection appends the semver/conventional-commits section
// to .github/copilot-instructions.md (creating the file if needed).
func generateSemverCopilotSection(projectRoot string) (string, error) {
	path := copilotPath(projectRoot)
	// Ensure the file exists — generateCopilotInstructions already handles
	// directory creation and base content; call it only when the file is absent.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if _, err := generateCopilotInstructions(projectRoot); err != nil {
			return "", err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if strings.Contains(string(data), semverCopilotMarker) {
		return path, nil
	}

	content := string(data) + semverCopilotSection
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}
