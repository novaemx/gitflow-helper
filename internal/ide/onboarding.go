package ide

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type aiIntegrationChoice struct {
	Enabled bool   `json:"enabled"`
	Version string `json:"version,omitempty"`
}

var askAIIntegrationFunc = askAIIntegration
var readAIAnswerFunc = func() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	return reader.ReadString('\n')
}

func aiIntegrationChoicePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".gitflow", "ai-integration.json")
}

func loadAIIntegrationChoice(projectRoot string) (choice aiIntegrationChoice, exists bool, err error) {
	path := aiIntegrationChoicePath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return aiIntegrationChoice{}, false, nil
		}
		return aiIntegrationChoice{}, false, err
	}
	if err := json.Unmarshal(data, &choice); err != nil {
		return aiIntegrationChoice{}, false, err
	}
	return choice, true, nil
}

func saveAIIntegrationChoice(projectRoot string, choice aiIntegrationChoice) error {
	path := aiIntegrationChoicePath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(choice, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
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

// EnsureRulesWithAIConsent installs IDE-specific instructions and embedded
// skill only when user consent for AI integration is enabled.
//
// Consent is persisted at {projectRoot}/.gitflow/ai-integration.json (per project).
// In non-interactive mode (agents / --json) this function does NOT auto-enable;
// it skips provisioning when no prior consent exists, preserving explicit
// user opt-in.
//
// When consent exists and the stored version matches appVersion, provisioning
// is skipped entirely (zero file I/O). On version mismatch, rules are
// re-provisioned idempotently and the stored version is updated.
func EnsureRulesWithAIConsent(projectRoot string, detected DetectedIDE, interactive bool, appVersion string) ([]string, error) {
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
		enabled, err := askAIIntegrationFunc(detected)
		if err != nil {
			return nil, err
		}
		// Save consent WITHOUT version so the provisioning path below runs
		// on this first invocation.  The version is stamped after provisioning.
		choice = aiIntegrationChoice{Enabled: enabled}
		if err := saveAIIntegrationChoice(projectRoot, choice); err != nil {
			return nil, err
		}
	}

	if !choice.Enabled {
		return []string{}, nil
	}

	// If the stored version matches the running binary, all rules are
	// already up-to-date — skip all file I/O.
	if appVersion != "" && choice.Version == appVersion {
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
