package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureMCPConfig_AllDetectedIDEs(t *testing.T) {
	cases := []struct {
		id      string
		relPath string
	}{
		{IDECursor, filepath.Join(".cursor", "mcp.json")},
		{IDECopilot, filepath.Join(".vscode", "mcp.json")},
		{IDEVSCode, filepath.Join(".vscode", "mcp.json")},
		{IDEClaudeCode, filepath.Join(".claude", "mcp.json")},
		{IDEWindsurf, filepath.Join(".windsurf", "mcp.json")},
		{IDECline, filepath.Join(".cline", "mcp.json")},
		{IDEZed, filepath.Join(".zed", "mcp.json")},
		{IDENeovim, filepath.Join(".nvim", "mcp.json")},
		{IDEJetBrains, filepath.Join(".idea", "mcp.json")},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			dir := t.TempDir()
			expected := filepath.Join(dir, tc.relPath)

			got, err := EnsureMCPConfig(dir, tc.id)
			if err != nil {
				t.Fatalf("EnsureMCPConfig(%s): %v", tc.id, err)
			}
			if got != expected {
				t.Fatalf("EnsureMCPConfig(%s): expected %s, got %s", tc.id, expected, got)
			}

			data, err := os.ReadFile(expected)
			if err != nil {
				t.Fatalf("read %s: %v", expected, err)
			}
			content := string(data)
			if !strings.Contains(content, `"gitflow"`) || !strings.Contains(content, `"serve"`) {
				t.Fatalf("invalid MCP content for %s: %s", tc.id, content)
			}
		})
	}
}

func TestEnsureMCPConfig_UnsupportedIDE(t *testing.T) {
	dir := t.TempDir()
	path, err := EnsureMCPConfig(dir, IDEUnknown)
	if err != nil {
		t.Fatalf("EnsureMCPConfig(unknown): %v", err)
	}
	if path != "" {
		t.Fatalf("expected empty path for unsupported IDE, got %q", path)
	}
}

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
