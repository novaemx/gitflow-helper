package ide

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPrimary_ReturnsValidResult(t *testing.T) {
	dir := t.TempDir()
	result := DetectPrimary(dir)
	// Environment may have IDE env vars (e.g. running inside Cursor/VSCode),
	// so we can't assert a specific IDE. Just verify it returns a valid result.
	if result.ID == "" {
		t.Error("expected non-empty IDE ID")
	}
	if result.DisplayName == "" {
		t.Error("expected non-empty display name")
	}
}

func TestDetectAll_NoPanic(t *testing.T) {
	dir := t.TempDir()
	all := DetectAll(dir)
	// In a temp dir with no IDE markers, the only possible detection
	// is via parent process or env vars (which we can't control in CI).
	// Just verify it doesn't panic and returns a slice.
	_ = all
}

func TestDetectCursor_ByDirectory(t *testing.T) {
	dir := t.TempDir()
	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatal(err)
	}

	// detectCursor checks for .cursor dir existence (and parent process)
	// Since we can't mock parent process in unit tests, just verify
	// that the function finds the directory marker.
	result := detectCursor(dir)
	if !result {
		t.Error("expected detectCursor to return true when .cursor/ exists")
	}
}

func TestDetectClaudeCode_ByClaudeMD(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0644)

	result := detectClaudeCode(dir)
	if !result {
		t.Error("expected detectClaudeCode to return true when CLAUDE.md exists")
	}
}

func TestDetectClaudeCode_ByClaudeDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	result := detectClaudeCode(dir)
	if !result {
		t.Error("expected detectClaudeCode to return true when .claude/ exists")
	}
}

func TestDetectWindsurf_ByDirectory(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".windsurf"), 0755)

	result := detectWindsurf(dir)
	if !result {
		t.Error("expected detectWindsurf to return true when .windsurf/ exists")
	}
}

func TestDetectWindsurf_ByRulesFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ".windsurfrules"), []byte("rules"), 0644)

	result := detectWindsurf(dir)
	if !result {
		t.Error("expected detectWindsurf to return true when .windsurfrules exists")
	}
}

func TestDetectCline_ByRulesFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ".clinerules"), []byte("rules"), 0644)

	result := detectCline(dir)
	if !result {
		t.Error("expected detectCline to return true when .clinerules exists")
	}
}

func TestDetectCline_ByDirectory(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".cline"), 0755)

	result := detectCline(dir)
	if !result {
		t.Error("expected detectCline to return true when .cline/ exists")
	}
}

func TestDetectZed_ByDirectory(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".zed"), 0755)

	result := detectZed(dir)
	if !result {
		t.Error("expected detectZed to return true when .zed/ exists")
	}
}

func TestDetectJetBrains_ByIdea(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".idea"), 0755)

	result := detectJetBrains(dir)
	if !result {
		t.Error("expected detectJetBrains to return true when .idea/ exists")
	}
}

func TestDetectVSCode_EnvVar(t *testing.T) {
	t.Setenv("VSCODE_GIT_ASKPASS_NODE", "/some/path")
	dir := t.TempDir()

	result := detectVSCode(dir)
	if !result {
		t.Error("expected detectVSCode to return true with VSCODE_GIT_ASKPASS_NODE set")
	}
}

func TestDetectNeovim_EnvVar(t *testing.T) {
	t.Setenv("NVIM", "/tmp/nvim.sock")
	dir := t.TempDir()

	result := detectNeovim(dir)
	if !result {
		t.Error("expected detectNeovim to return true with NVIM set")
	}
}

func TestDetectPrimary_ReturnsValid(t *testing.T) {
	dir := t.TempDir()
	result := DetectPrimary(dir)
	if result.ID == "" {
		t.Error("expected non-empty IDE ID")
	}
	if result.DisplayName == "" {
		t.Error("expected non-empty display name")
	}
}

func TestIDERegistryCompleteness(t *testing.T) {
	// Verify all IDE constants have entries in ideRuleRegistry
	ides := []string{
		IDECursor, IDEVSCode, IDECopilot, IDEClaudeCode,
		IDEWindsurf, IDECline, IDEZed, IDENeovim, IDEJetBrains,
	}
	for _, id := range ides {
		if _, ok := ideRuleRegistry[id]; !ok {
			t.Errorf("IDE %q missing from ideRuleRegistry", id)
		}
	}
}

func TestIdeRegistryConsistency(t *testing.T) {
	// Verify all entries in ideRegistry have corresponding rule registry entries
	for _, entry := range ideRegistry {
		if entry.id == IDEUnknown || entry.id == IDEBoth {
			continue
		}
		if _, ok := ideRuleRegistry[entry.id]; !ok {
			t.Errorf("IDE %q in ideRegistry but missing from ideRuleRegistry", entry.id)
		}
	}
}
