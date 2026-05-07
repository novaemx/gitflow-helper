package ide

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
)

func writeTestConsent(t *testing.T, projectDir string, enabled bool) {
	t.Helper()
	if err := config.SaveAIIntegrationChoice(projectDir, config.AIIntegrationChoice{Enabled: enabled}); err != nil {
		t.Fatalf("save consent: %v", err)
	}
}

func readConsentEnabled(t *testing.T, projectDir string) bool {
	t.Helper()
	choice, exists, err := config.LoadAIIntegrationChoice(projectDir)
	if err != nil {
		t.Fatalf("read consent: %v", err)
	}
	if !exists {
		t.Fatal("expected consent to exist")
	}
	return choice.Enabled
}

func TestEnsureRulesWithAIConsent_FirstRunAccepts(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := AskAIIntegrationFunc
	AskAIIntegrationFunc = func(_ DetectedIDE) (bool, error) { return true, nil }
	defer func() { AskAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.0.0")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) < 3 {
		t.Fatalf("expected created files, got %d", len(created))
	}
	if !readConsentEnabled(t, dir) {
		t.Fatal("expected consent enabled=true stored in project dir")
	}
}

func TestEnsureRulesWithAIConsent_FirstRunDeclines(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := AskAIIntegrationFunc
	AskAIIntegrationFunc = func(_ DetectedIDE) (bool, error) { return false, nil }
	defer func() { AskAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.0.0")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected no files when declined, got %d", len(created))
	}
	if readConsentEnabled(t, dir) {
		t.Fatal("expected consent enabled=false stored in project dir")
	}
	if cursorRuleExists(dir) {
		t.Fatal("expected no cursor rule when declined")
	}
}

func TestEnsureRulesWithAIConsent_UsesExistingEnabledChoice(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()
	writeTestConsent(t, dir, true)

	prevAsk := AskAIIntegrationFunc
	called := false
	AskAIIntegrationFunc = func(_ DetectedIDE) (bool, error) {
		called = true
		return false, nil
	}
	defer func() { AskAIIntegrationFunc = prevAsk }()

	// Pass empty version to bypass version check — simulates already-provisioned state.
	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if called {
		t.Fatal("did not expect prompt when choice already exists in project")
	}
	if len(created) < 3 {
		t.Fatalf("expected files from enabled existing choice, got %d", len(created))
	}
}

func TestEnsureRulesWithAIConsent_UsesExistingDeclinedChoice(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()
	writeTestConsent(t, dir, false)

	prevAsk := AskAIIntegrationFunc
	called := false
	AskAIIntegrationFunc = func(_ DetectedIDE) (bool, error) {
		called = true
		return true, nil
	}
	defer func() { AskAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.0.0")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if called {
		t.Fatal("did not expect prompt when choice already exists in project")
	}
	if len(created) != 0 {
		t.Fatalf("expected no files for declined existing choice, got %d", len(created))
	}
}

func TestEnsureRulesWithAIConsent_NonInteractiveSkipsWithoutPriorConsent(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := AskAIIntegrationFunc
	AskAIIntegrationFunc = func(_ DetectedIDE) (bool, error) {
		t.Fatal("did not expect prompt in non-interactive mode")
		return false, nil
	}
	defer func() { AskAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, false, "1.0.0")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected no files when non-interactive and no prior consent, got %d", len(created))
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

	// Write invalid JSON to the project-scoped unified config path.
	consentPath := config.ProjectConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(consentPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(consentPath, []byte("{invalid"), 0644); err != nil {
		t.Fatalf("write invalid consent: %v", err)
	}

	_, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.0.0")
	if err == nil {
		t.Fatal("expected parse error for invalid consent file")
	}
}

func TestEnsureRulesWithAIConsent_SkipsWhenVersionMatches(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	// Pre-create project-scoped consent with version "1.0.0"
	p := config.ProjectConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"ai_integration":{"enabled":true,"version":"1.0.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.0.0")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected zero files when version matches, got %d: %v", len(created), created)
	}
}

func TestEnsureRulesWithAIConsent_ReprovisionsOnVersionUpgrade(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	// Pre-create project-scoped consent with old version
	p := config.ProjectConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"ai_integration":{"enabled":true,"version":"0.9.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.0.0")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) < 1 {
		t.Fatalf("expected files to be provisioned on version upgrade, got %d", len(created))
	}

	// Verify stored version was updated in the unified project config file.
	choice, exists, err := config.LoadAIIntegrationChoice(dir)
	if err != nil {
		t.Fatalf("LoadAIIntegrationChoice: %v", err)
	}
	if !exists {
		t.Fatal("expected ai integration choice to exist after reprovision")
	}
	if choice.Version != "1.0.0" {
		t.Fatalf("expected version updated to 1.0.0, got %q", choice.Version)
	}
}

func TestEnsureRulesWithAIConsent_SkipsWhenStoredVersionIsNewer(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	// Simulate running an older binary after rules were already stamped by a
	// newer one. This should not force re-provision.
	p := config.ProjectConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"ai_integration":{"enabled":true,"version":"1.2.0"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.1.9")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected zero files when stored version is newer, got %d", len(created))
	}
}

func TestShouldReprovisionRules(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		app    string
		want   bool
	}{
		{name: "empty_app_reprovisions", stored: "1.0.0", app: "", want: true},
		{name: "empty_stored_reprovisions", stored: "", app: "1.0.0", want: true},
		{name: "equal_versions_skip", stored: "1.0.0", app: "1.0.0", want: false},
		{name: "upgrade_reprovisions", stored: "1.0.0", app: "1.0.1", want: true},
		{name: "downgrade_skips", stored: "1.2.0", app: "1.1.9", want: false},
		{name: "accept_v_prefix", stored: "v1.0.0", app: "v1.0.1", want: true},
		{name: "invalid_version_falls_back_to_mismatch", stored: "latest", app: "1.0.0", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldReprovisionRules(tc.stored, tc.app)
			if got != tc.want {
				t.Fatalf("stored=%q app=%q: expected %v, got %v", tc.stored, tc.app, tc.want, got)
			}
		})
	}
}

// TestEnsureRulesWithAIConsent_DifferentProjectsAskedSeparately verifies that
// consent stored in project A does not suppress the prompt in project B.
func TestEnsureRulesWithAIConsent_DifferentProjectsAskedSeparately(t *testing.T) {
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	dirA := t.TempDir()
	dirB := t.TempDir()

	askCount := 0
	prevAsk := AskAIIntegrationFunc
	AskAIIntegrationFunc = func(_ DetectedIDE) (bool, error) {
		askCount++
		return true, nil
	}
	defer func() { AskAIIntegrationFunc = prevAsk }()

	_, err := EnsureRulesWithAIConsent(dirA, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.0.0")
	if err != nil {
		t.Fatalf("project A: %v", err)
	}
	if askCount != 1 {
		t.Fatalf("expected 1 prompt for project A, got %d", askCount)
	}

	// Project B has no consent file — must ask independently.
	_, err = EnsureRulesWithAIConsent(dirB, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, true, "1.0.0")
	if err != nil {
		t.Fatalf("project B: %v", err)
	}
	if askCount != 2 {
		t.Fatalf("expected 2 prompts (one per project), got %d", askCount)
	}
}

func TestEnsureRulesWithAIConsent_NonInteractiveSkipsFirstRun(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	prevHome := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return home, nil }
	defer func() { UserHomeDirFunc = prevHome }()

	prevAsk := AskAIIntegrationFunc
	AskAIIntegrationFunc = func(_ DetectedIDE) (bool, error) {
		t.Fatal("should not prompt in non-interactive mode")
		return false, nil
	}
	defer func() { AskAIIntegrationFunc = prevAsk }()

	created, err := EnsureRulesWithAIConsent(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"}, false, "1.0.0")
	if err != nil {
		t.Fatalf("EnsureRulesWithAIConsent: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected no files in non-interactive first run, got %d", len(created))
	}
}
