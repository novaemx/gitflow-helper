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
	IDEID   string `json:"ide_id,omitempty"`
}

var askAIIntegrationFunc = askAIIntegration
var readAIAnswerFunc = func() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	return reader.ReadString('\n')
}

func aiIntegrationChoicePath() (string, error) {
	home, err := UserHomeDirFunc()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gitflow", "ai-integration.json"), nil
}

func loadAIIntegrationChoice() (choice aiIntegrationChoice, exists bool, err error) {
	path, err := aiIntegrationChoicePath()
	if err != nil {
		return aiIntegrationChoice{}, false, err
	}
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

func saveAIIntegrationChoice(choice aiIntegrationChoice) error {
	path, err := aiIntegrationChoicePath()
	if err != nil {
		return err
	}
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
// Consent is persisted at $HOME/.gitflow/ai-integration.json.
// In non-interactive mode this function defaults to enabled to preserve
// automation behavior for agents and scripts.
func EnsureRulesWithAIConsent(projectRoot string, detected DetectedIDE, interactive bool) ([]string, error) {
	choice, exists, err := loadAIIntegrationChoice()
	if err != nil {
		return nil, err
	}

	if !exists {
		enabled := true
		if interactive {
			enabled, err = askAIIntegrationFunc(detected)
			if err != nil {
				return nil, err
			}
		}
		choice = aiIntegrationChoice{Enabled: enabled, IDEID: detected.ID}
		if err := saveAIIntegrationChoice(choice); err != nil {
			return nil, err
		}
	}

	if !choice.Enabled {
		return []string{}, nil
	}

	return EnsureRulesForIDE(projectRoot, detected)
}
