package ide

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTestConsent(t *testing.T, home string, enabled bool, ideID string) {
	t.Helper()
	p := filepath.Join(home, ".gitflow", "ai-integration.json")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("mkdir consent dir: %v", err)
	}
	payload := map[string]any{
		"enabled": enabled,
		"ide_id":  ideID,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal consent: %v", err)
	}
	if err := os.WriteFile(p, b, 0644); err != nil {
		t.Fatalf("write consent: %v", err)
	}
}

func readConsentEnabled(t *testing.T, home string) bool {
	t.Helper()
	p := filepath.Join(home, ".gitflow", "ai-integration.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read consent: %v", err)
	}
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("unmarshal consent: %v", err)
	}
	return payload.Enabled
}

func TestEnsureRulesWithAIConsent_FirstRunAccepts(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()

	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := askAIIntegrationFunc
	askAIIntegrationFunc = func(_ DetectedIDE) (bool, error) { return true, nil }
	defer func() { askAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true)
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) < 3 {
		t.Fatalf("expected created files, got %d", len(created))
	}
	if !readConsentEnabled(t, home) {
		t.Fatal("expected consent enabled=true")
	}
}

func TestEnsureRulesWithAIConsent_FirstRunDeclines(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()

	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := askAIIntegrationFunc
	askAIIntegrationFunc = func(_ DetectedIDE) (bool, error) { return false, nil }
	defer func() { askAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true)
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected no files when declined, got %d", len(created))
	}
	if readConsentEnabled(t, home) {
		t.Fatal("expected consent enabled=false")
	}
	if cursorRuleExists(dir) {
		t.Fatal("expected no cursor rule when declined")
	}
}

func TestEnsureRulesWithAIConsent_UsesExistingEnabledChoice(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	writeTestConsent(t, home, true, IDECursor)

	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := askAIIntegrationFunc
	called := false
	askAIIntegrationFunc = func(_ DetectedIDE) (bool, error) {
		called = true
		return false, nil
	}
	defer func() { askAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true)
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if called {
		t.Fatal("did not expect prompt when choice already exists")
	}
	if len(created) < 3 {
		t.Fatalf("expected files from enabled existing choice, got %d", len(created))
	}
}

func TestEnsureRulesWithAIConsent_UsesExistingDeclinedChoice(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	writeTestConsent(t, home, false, IDECursor)

	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := askAIIntegrationFunc
	called := false
	askAIIntegrationFunc = func(_ DetectedIDE) (bool, error) {
		called = true
		return true, nil
	}
	defer func() { askAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true)
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if called {
		t.Fatal("did not expect prompt when choice already exists")
	}
	if len(created) != 0 {
		t.Fatalf("expected no files for declined existing choice, got %d", len(created))
	}
}

func TestEnsureRulesWithAIConsent_NonInteractiveDefaultsToInstall(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()

	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := askAIIntegrationFunc
	askAIIntegrationFunc = func(_ DetectedIDE) (bool, error) {
		t.Fatal("did not expect prompt in non-interactive mode")
		return false, nil
	}
	defer func() { askAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, false)
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) < 3 {
		t.Fatalf("expected files in non-interactive mode, got %d", len(created))
	}
}

func TestAskAIIntegration_ParseAnswers(t *testing.T) {
	prevRead := readAIAnswerFunc
	defer func() { readAIAnswerFunc = prevRead }()

	t.Run("default_yes", func(t *testing.T) {
		readAIAnswerFunc = func() (string, error) { return "\n", nil }
		enabled, err := askAIIntegration(DetectedIDE{DisplayName: "Cursor"})
		if err != nil {
			t.Fatalf("askAIIntegration: %v", err)
		}
		if !enabled {
			t.Fatal("expected enabled on empty answer")
		}
	})

	t.Run("explicit_no", func(t *testing.T) {
		readAIAnswerFunc = func() (string, error) { return "n\n", nil }
		enabled, err := askAIIntegration(DetectedIDE{DisplayName: "Cursor"})
		if err != nil {
			t.Fatalf("askAIIntegration: %v", err)
		}
		if enabled {
			t.Fatal("expected disabled on no answer")
		}
	})

	t.Run("io_error", func(t *testing.T) {
		readAIAnswerFunc = func() (string, error) { return "", errors.New("boom") }
		_, err := askAIIntegration(DetectedIDE{DisplayName: "Cursor"})
		if err == nil {
			t.Fatal("expected input error")
		}
	})
}

func TestEnsureRulesWithAIConsent_InvalidStoredChoiceFails(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()

	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	consentPath := filepath.Join(home, ".gitflow", "ai-integration.json")
	if err := os.MkdirAll(filepath.Dir(consentPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(consentPath, []byte("{invalid"), 0644); err != nil {
		t.Fatalf("write invalid consent: %v", err)
	}

	_, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true)
	if err == nil {
		t.Fatal("expected parse error for invalid consent file")
	}
}
