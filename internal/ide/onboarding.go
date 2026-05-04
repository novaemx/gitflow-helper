package ide

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/config"
)

type aiIntegrationChoice = config.AIIntegrationChoice

var AskAIIntegrationFunc = askAIIntegration
var readAIAnswerFunc = func() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	return reader.ReadString('\n')
}

func aiIntegrationChoicePath(projectRoot string) string {
	return config.ProjectConfigPath(projectRoot)
}

func loadAIIntegrationChoice(projectRoot string) (choice aiIntegrationChoice, exists bool, err error) {
	return config.LoadAIIntegrationChoice(projectRoot)
}

func saveAIIntegrationChoice(projectRoot string, choice aiIntegrationChoice) error {
	return config.SaveAIIntegrationChoice(projectRoot, choice)
}

func askAIIntegration(detected DetectedIDE) (bool, error) {
	fmt.Printf("\nEnable AI integration for gitflow in %s? [Y/n]: ", detected.DisplayName)
	text, err := readAIAnswerFunc()
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(text))
	if answer == "" || answer == "y" || answer == "yes" || answer == "s" || answer == "si" {
		return true, nil
	}
	if answer == "n" || answer == "no" {
		return false, nil
	}
	return false, nil
}

func parseSemverParts(version string) ([3]int, bool) {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return [3]int{}, false
	}

	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return [3]int{}, false
	}

	var out [3]int
	for i := 0; i < len(parts); i++ {
		if parts[i] == "" {
			return [3]int{}, false
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}

	return out, true
}

// shouldReprovisionRules reports whether IDE rules/skills should be refreshed
// based on stored and running versions.
func shouldReprovisionRules(storedVersion, appVersion string) bool {
	if strings.TrimSpace(appVersion) == "" {
		return true
	}
	if strings.TrimSpace(storedVersion) == "" {
		return true
	}

	running, runningOK := parseSemverParts(appVersion)
	stored, storedOK := parseSemverParts(storedVersion)
	if !runningOK || !storedOK {
		return appVersion != storedVersion
	}

	for i := 0; i < 3; i++ {
		if running[i] > stored[i] {
			return true
		}
		if running[i] < stored[i] {
			return false
		}
	}

	return false
}

func needsReprovisionFromFileVersions(projectRoot string, detected DetectedIDE) bool {
	// Cursor rule is fully-generated: use content equality so any body change
	// (fix, new section, updated template) triggers refresh, not just version bumps.
	if detected.ID == IDECursor || detected.ID == IDEBoth {
		if fileContentDiffers(cursorRulePath(projectRoot), cursorRuleContent()) {
			return true
		}
		if fileContentDiffers(semverCursorRulePath(projectRoot), semverCursorRuleContent()) {
			return true
		}
	} else if spec, ok := ideRuleRegistry[detected.ID]; ok {
		// Append-style files: version stamp is sufficient.
		if fileNeedsVersionRefresh(spec.path(projectRoot)) {
			return true
		}
	}

	// SKILL.md is fully-generated; content equality is checked inside
	// ensureEmbeddedSkill. Here we also version-check to trigger the
	// EnsureRulesForIDE call that invokes ensureEmbeddedSkill.
	skillPath, err := skillPathForIDE(projectRoot, detected.ID)
	if err != nil || fileNeedsVersionRefresh(skillPath) {
		return true
	}

	if !projectScopedSkillIDEs[detected.ID] && fileNeedsVersionRefresh(agentsPath(projectRoot)) {
		return true
	}

	return false
}

// EnsureRulesWithAIConsent installs IDE-specific instructions and embedded
// skill only when user consent for AI integration is enabled.
//
// Consent is persisted at {projectRoot}/.gitflow/config.json (per project).
// In non-interactive mode (agents / --json) this function does NOT auto-enable;
// it skips provisioning when no prior consent exists, preserving explicit
// user opt-in.
//
// When consent exists, provisioning runs only when appVersion is unknown, no
// stored version exists yet, or the running appVersion is newer than the
// stored version. Otherwise, file I/O is skipped.
func EnsureRulesWithAIConsent(projectRoot string, detected DetectedIDE, interactive bool, appVersion string) ([]string, error) {
	SetGeneratorVersion(appVersion)

	choice, exists, err := loadAIIntegrationChoice(projectRoot)
	if err != nil {
		return nil, err
	}

	if !exists {
		if !interactive {
			// Non-interactive (agent/JSON): do not auto-enable — require
			// explicit user consent from a prior interactive session.
			return []string{}, nil
		}
		enabled, err := AskAIIntegrationFunc(detected)
		if err != nil {
			return nil, err
		}
		// Save consent WITHOUT version so the provisioning path below runs
		// on this first invocation. The version is stamped after provisioning.
		choice = aiIntegrationChoice{Enabled: enabled}
		if err := saveAIIntegrationChoice(projectRoot, choice); err != nil {
			return nil, err
		}
	}

	if !choice.Enabled {
		return []string{}, nil
	}

	// Never force file reprovision when this binary is older than the stored
	// consent version.
	if isOlderVersion(appVersion, choice.Version) {
		return []string{}, nil
	}

	// Refresh on first stamp, explicit unknown version, newer binary version,
	// or when managed files have missing/stale version headers.
	if !shouldReprovisionRules(choice.Version, appVersion) && !needsReprovisionFromFileVersions(projectRoot, detected) {
		return []string{}, nil
	}

	created, err := EnsureRulesForIDE(projectRoot, detected)
	if err != nil {
		return created, err
	}

	// Update stored version so subsequent runs skip provisioning.
	if appVersion != "" && choice.Version != appVersion {
		choice.Version = appVersion
		_ = saveAIIntegrationChoice(projectRoot, choice)
	}

	return created, nil
}
