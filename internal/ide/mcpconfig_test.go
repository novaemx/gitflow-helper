package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureMCPConfigForCopilotCreatesVSCodeConfig(t *testing.T) {
	dir := t.TempDir()

	path, err := EnsureMCPConfig(dir, IDECopilot)
	if err != nil {
		t.Fatalf("EnsureMCPConfig: %v", err)
	}

	expected := filepath.Join(dir, ".vscode", "mcp.json")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mcp config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"gitflow"`) {
		t.Fatalf("expected gitflow server entry, got: %s", content)
	}
	if !strings.Contains(content, `"serve"`) {
		t.Fatalf("expected serve arg in mcp config, got: %s", content)
	}
}

func TestGenerateCopilotCreatesInstructionAndMCP(t *testing.T) {
	dir := t.TempDir()

	files, err := Generate(dir, IDECopilot)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	wantInstructions := filepath.Join(dir, ".github", "copilot-instructions.md")
	wantMCP := filepath.Join(dir, ".vscode", "mcp.json")

	if _, err := os.Stat(wantInstructions); err != nil {
		t.Fatalf("expected copilot instructions: %v", err)
	}
	if _, err := os.Stat(wantMCP); err != nil {
		t.Fatalf("expected copilot MCP config: %v", err)
	}
	if len(files) < 3 {
		t.Fatalf("expected at least copilot instructions + AGENTS.md + mcp config, got %d", len(files))
	}
}
