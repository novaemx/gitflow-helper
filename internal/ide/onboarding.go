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

// EnsureOptions controls behavior of EnsureRulesWithAIConsent. Zero value
// is the default (interactive consent, no force, no dry-run). Pass as the
// optional last argument; omitting it preserves backward compatibility.
type EnsureOptions struct {
	// AutoAccept skips the consent dialog on first run. Used by `gitflow
	// setup --yes` for non-interactive agents and CI.
	AutoAccept bool

	// Force re-writes all artifacts even when their content matches the
	// expected byte stream (bypasses the idempotency check in
	// ensureEmbeddedSkill and the cursor rule's content-equality guard).
	Force bool

	// CheckOnly computes what would be created or removed without writing
	// to disk. Returned slice is the list of paths that WOULD be created
	// on a real run.
	CheckOnly bool
}

// EnsureRulesWithAIConsent installs IDE-specific instructions and embedded
// skill only when user consent for AI integration is enabled.
//
// Consent is persisted at {projectRoot}/.gitflow/config.json (per project).
// In non-interactive mode (agents / --json) this function does NOT auto-enable
// unless opts.AutoAccept is set; it skips provisioning when no prior consent
// exists, preserving explicit user opt-in.
//
// When consent exists, provisioning runs only when appVersion is unknown, no
// stored version exists yet, or the running appVersion is newer than the
// stored version. Otherwise, file I/O is skipped (unless opts.Force).
func EnsureRulesWithAIConsent(projectRoot string, detected DetectedIDE, interactive bool, appVersion string, opts ...EnsureOptions) ([]string, error) {
	var o EnsureOptions
	if len(opts) > 0 {
		o = opts[0]
	}

	choice, exists, err := loadAIIntegrationChoice(projectRoot)
	if err != nil {
		return nil, err
	}

	if !exists {
		if !o.AutoAccept && !interactive {
			// Non-interactive (agent/JSON) without --yes: do not auto-enable
			// — require explicit user consent from a prior interactive
			// session.
			return []string{}, nil
		}
		var enabled bool
		if o.AutoAccept {
			enabled = true
		} else {
			enabled, err = AskAIIntegrationFunc(detected)
			if err != nil {
				return nil, err
			}
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

	// Refresh only on first stamp, explicit unknown version, or when the
	// running version is newer than the stored one. opts.Force bypasses
	// this check (used by `gitflow setup --force`).
	if !o.Force && !shouldReprovisionRules(choice.Version, appVersion) {
		return []string{}, nil
	}

	if o.CheckOnly {
		// Dry run: report what would be created without writing to disk.
		// Use a throwaway temp dir to compute the would-be-created list.
		return planEnsureRulesForIDE(projectRoot, detected)
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

// RemoveConsent deletes the AI integration choice entry from the project's
// .gitflow/config.json (or no-ops if absent).
func RemoveConsent(projectRoot string) error {
	// Saving an empty choice and then re-loading should leave no enabled
	// entry. The simplest path is to overwrite with a zero-value struct
	// (Enabled=false) which LoadAIIntegrationChoice treats as "exists but
	// declined". For full removal we delegate to the config package.
	return config.RemoveAIIntegrationChoice(projectRoot)
}
