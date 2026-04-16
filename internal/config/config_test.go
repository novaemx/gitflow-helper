package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_UsesProjectConfigPath(t *testing.T) {
	dir := t.TempDir()
	path := ProjectConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"integration_mode":"pull-request","remote":"upstream"}`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := LoadConfig(dir)
	if cfg.IntegrationMode != IntegrationModePullRequest {
		t.Fatalf("expected pull-request mode, got %q", cfg.IntegrationMode)
	}
	if cfg.Remote != "upstream" {
		t.Fatalf("expected remote upstream, got %q", cfg.Remote)
	}
	if !cfg.ModeConfigured {
		t.Fatal("expected mode to be marked configured")
	}
}

func TestSetIntegrationMode_WritesProjectConfigAndRemovesLegacy(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, legacyConfigFileName)
	if err := os.WriteFile(legacyPath, []byte(`{"remote":"origin"}`), 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	if err := SetIntegrationMode(dir, IntegrationModePullRequest); err != nil {
		t.Fatalf("SetIntegrationMode: %v", err)
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy config removed, got err=%v", err)
	}

	data, err := os.ReadFile(ProjectConfigPath(dir))
	if err != nil {
		t.Fatalf("read new config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"integration_mode": "pull-request"`) {
		t.Fatalf("expected integration_mode in config, got %s", text)
	}
	if !strings.Contains(text, `"remote": "origin"`) {
		t.Fatalf("expected preserved remote in config, got %s", text)
	}
}

func TestAIIntegrationChoice_UsesUnifiedProjectConfig(t *testing.T) {
	dir := t.TempDir()
	choice := AIIntegrationChoice{Enabled: true, Version: "1.2.3"}

	if err := SaveAIIntegrationChoice(dir, choice); err != nil {
		t.Fatalf("SaveAIIntegrationChoice: %v", err)
	}

	loaded, exists, err := LoadAIIntegrationChoice(dir)
	if err != nil {
		t.Fatalf("LoadAIIntegrationChoice: %v", err)
	}
	if !exists {
		t.Fatal("expected ai integration choice to exist")
	}
	if loaded != choice {
		t.Fatalf("expected %+v, got %+v", choice, loaded)
	}

	data, err := os.ReadFile(ProjectConfigPath(dir))
	if err != nil {
		t.Fatalf("read unified config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"ai_integration"`) {
		t.Fatalf("expected ai_integration block, got %s", text)
	}
	if !strings.Contains(text, `"version": "1.2.3"`) {
		t.Fatalf("expected stored version, got %s", text)
	}
	if _, err := os.Stat(legacyAIIntegrationPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("expected no legacy ai config file, got err=%v", err)
	}
}
