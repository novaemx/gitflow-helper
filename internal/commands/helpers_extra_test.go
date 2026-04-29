package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	"github.com/novaemx/gitflow-helper/internal/output"
)

func TestStartFailureLines_AllFields(t *testing.T) {
	lines := startFailureLines(map[string]any{
		"error":       "boom",
		"hint":        "try again",
		"diagnostics": []string{"d1", "d2"},
	})
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
	}
}

func TestStartFailureLines_Empty(t *testing.T) {
	lines := startFailureLines(map[string]any{})
	if len(lines) != 0 {
		t.Fatalf("expected no lines, got %v", lines)
	}
}

func TestNewModeCmd_ShowCurrentMode(t *testing.T) {
	dir := t.TempDir()
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, IntegrationMode: config.IntegrationModeLocalMerge})

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	defer output.SetJSONMode(prevJSON)

	cmd := newModeCmd()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("mode show failed: %v", err)
	}
}

func TestNewModeCmd_SetAndToggle(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".gitflow"), 0755); err != nil {
		t.Fatalf("mkdir .gitflow: %v", err)
	}
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, IntegrationMode: config.IntegrationModeLocalMerge})

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	defer output.SetJSONMode(prevJSON)

	cmd := newModeCmd()
	if err := cmd.RunE(cmd, []string{"pr"}); err != nil {
		t.Fatalf("set pr mode failed: %v", err)
	}
	if GF.Config.IntegrationMode != config.IntegrationModePullRequest {
		t.Fatalf("expected pull-request mode, got %q", GF.Config.IntegrationMode)
	}
	if err := cmd.RunE(cmd, []string{"toggle"}); err != nil {
		t.Fatalf("toggle mode failed: %v", err)
	}
	if GF.Config.IntegrationMode != config.IntegrationModeLocalMerge {
		t.Fatalf("expected local-merge mode after toggle, got %q", GF.Config.IntegrationMode)
	}
}

func TestNewModeCmd_InvalidMode(t *testing.T) {
	dir := t.TempDir()
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, IntegrationMode: config.IntegrationModeLocalMerge})

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	defer output.SetJSONMode(prevJSON)

	cmd := newModeCmd()
	if err := cmd.RunE(cmd, []string{"bad"}); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}
