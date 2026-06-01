package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// unsetEnvPrefix unsets all env vars matching prefix and restores them after the test.
func unsetEnvPrefix(t *testing.T, prefix string) {
	t.Helper()
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, prefix) {
			name := strings.SplitN(e, "=", 2)[0]
			oldVal := os.Getenv(name)
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("unsetenv %s: %v", name, err)
			}
			t.Cleanup(func() { _ = os.Setenv(name, oldVal) })
		}
	}
}

// withFakeProcessAncestry replaces both the Windows and the generic
// parent-process lookups so that no parent name contains any of the
// provided needles (case-insensitive). Used to make IDE-detection tests
// hermetic when the test runner itself is launched by Claude Code /
// Cursor / VSCode (so matchParentProcess would otherwise return true).
func withFakeProcessAncestry(t *testing.T, names ...string) {
	t.Helper()
	origWin := windowsProcessAncestryFunc
	origGen := parentProcessInfoFunc
	windowsProcessAncestryFunc = func(startPID, maxDepth int) ([]string, error) {
		return names, nil
	}
	parentProcessInfoFunc = func(pid int, goos string) (string, int, error) {
		if len(names) == 0 {
			return "fake-parent", 1, nil
		}
		return names[0], 1, nil
	}
	t.Cleanup(func() {
		windowsProcessAncestryFunc = origWin
		parentProcessInfoFunc = origGen
	})
}

func TestCompanionIDEs_Matrix(t *testing.T) {
	// Make the test hermetic regardless of the test runner's environment.
	unsetEnvPrefix(t, "CLAUDE_")
	unsetEnvPrefix(t, "ANTHROPIC_")
	withFakeProcessAncestry(t, "bash", "go", "test-runner")

	tests := []struct {
		primary  string
		wantHave []string // expected non-empty companion IDs
	}{
		{IDECursor, nil},
		{IDEVSCode, nil},
		{IDECopilot, nil},
		{IDEClaudeCode, nil},
		{IDEWindsurf, nil},
		{IDECline, nil},
		{IDEZed, nil},
		{IDENeovim, nil},
		{IDEJetBrains, nil},
		{IDEBoth, nil},
		{IDEUnknown, nil},
	}
	for _, tc := range tests {
		t.Run(tc.primary, func(t *testing.T) {
			got := companionIDEs(tc.primary, t.TempDir())
			if len(got) != len(tc.wantHave) {
				t.Errorf("companionIDEs(%s) = %v, want empty/size %d", tc.primary, got, len(tc.wantHave))
			}
		})
	}

	// When Claude Code IS detected (env var) and primary is Cursor-family,
	// companion should be [IDEClaudeCode].
	t.Run("cursor+claude-detected", func(t *testing.T) {
		t.Setenv("CLAUDE_SESSION", "1")
		dir := t.TempDir()
		for _, primary := range []string{IDECursor, IDEBoth, IDEVSCode, IDECopilot} {
			got := companionIDEs(primary, dir)
			if len(got) != 1 || got[0] != IDEClaudeCode {
				t.Errorf("companionIDEs(%s) with Claude detected = %v, want [%s]", primary, got, IDEClaudeCode)
			}
		}
	})

	// Claude Code as primary returns no companion (no recursion).
	t.Run("claude-code-primary-no-companion", func(t *testing.T) {
		t.Setenv("CLAUDE_SESSION", "1")
		dir := t.TempDir()
		got := companionIDEs(IDEClaudeCode, dir)
		if got != nil {
			t.Errorf("companionIDEs(claude-code) = %v, want nil", got)
		}
	})
}

func TestEnsureRulesForIDE_Cursor_AlsoInstallsClaudeCodeWhenDetected(t *testing.T) {
	t.Setenv("CLAUDE_SESSION", "1")
	dir := t.TempDir()

	created, err := EnsureRulesForIDE(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"})
	if err != nil {
		t.Fatalf("EnsureRulesForIDE: %v", err)
	}

	// Cursor artifacts must be present.
	if !cursorRuleExists(dir) {
		t.Error("expected Cursor rule to exist")
	}
	cursorMCPPath := filepath.Join(dir, ".cursor", "mcp.json")
	if _, err := os.Stat(cursorMCPPath); err != nil {
		t.Errorf("expected Cursor MCP config at %s: %v", cursorMCPPath, err)
	}

	// Claude Code artifacts must also be present (companion install).
	if !claudeCodeRuleExists(dir) {
		t.Error("expected CLAUDE.md to exist (companion install)")
	}
	claudeMCPPath := filepath.Join(dir, ".claude", "mcp.json")
	if _, err := os.Stat(claudeMCPPath); err != nil {
		t.Errorf("expected Claude MCP config at %s: %v", claudeMCPPath, err)
	}

	// Skill must be installed once.
	skillPath := filepath.Join(dir, ".agents", "skills", "gitflow", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Errorf("expected skill at %s: %v", skillPath, err)
	}

	// AGENTS.md must NOT be created (both Cursor and Claude Code are
	// projectScopedSkillIDEs).
	if agentsRuleExists(dir) {
		t.Error("AGENTS.md must NOT be created when skill is project-scoped")
	}

	// Sanity: at least 4 files were created (cursor rule + cursor mcp + claude.md + claude mcp).
	if len(created) < 4 {
		t.Errorf("expected at least 4 created files, got %d: %v", len(created), created)
	}
}

func TestEnsureRulesForIDE_Cursor_NoClaudeCodeLeakageWhenNotDetected(t *testing.T) {
	unsetEnvPrefix(t, "CLAUDE_")
	unsetEnvPrefix(t, "ANTHROPIC_")
	withFakeProcessAncestry(t, "bash", "go", "test-runner")
	dir := t.TempDir()

	created, err := EnsureRulesForIDE(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"})
	if err != nil {
		t.Fatalf("EnsureRulesForIDE: %v", err)
	}

	// Cursor artifacts present.
	if !cursorRuleExists(dir) {
		t.Error("expected Cursor rule to exist")
	}

	// Claude Code artifacts must NOT be present (no detection → no companion).
	if claudeCodeRuleExists(dir) {
		t.Error("CLAUDE.md must NOT be created when Claude Code is not detected")
	}
	claudeMCPPath := filepath.Join(dir, ".claude", "mcp.json")
	if _, err := os.Stat(claudeMCPPath); !os.IsNotExist(err) {
		t.Errorf("Claude MCP config must not exist; stat err = %v", err)
	}
	// Sanity: only Cursor artifacts (rule + mcp) + 1 skill; no Claude companion files.
	for _, f := range created {
		if strings.Contains(f, ".claude") || strings.HasSuffix(f, "CLAUDE.md") {
			t.Errorf("created list contains Claude artifact but Claude not detected: %s", f)
		}
	}
}

func TestEnsureRulesForIDE_ClaudeCode_NoRecursiveLoop(t *testing.T) {
	t.Setenv("CLAUDE_SESSION", "1")
	dir := t.TempDir()

	// Direct Claude Code primary; with Claude detected, no companion should
	// be added (the companion matrix returns nil for claude-code primary),
	// so this should not infinite-loop.
	type result struct {
		created []string
		err     error
	}
	resCh := make(chan result, 1)
	go func() {
		created, err := EnsureRulesForIDE(dir, DetectedIDE{ID: IDEClaudeCode, DisplayName: "Claude Code"})
		resCh <- result{created, err}
	}()
	_ = result{}
	select {
	case r := <-resCh:
		if r.err != nil {
			t.Fatalf("EnsureRulesForIDE: %v", r.err)
		}
		if !claudeCodeRuleExists(dir) {
			t.Error("expected CLAUDE.md to exist")
		}
		// No cursor rule (Claude Code is not a companion of itself).
		if cursorRuleExists(dir) {
			t.Error("Cursor rule must NOT be created when primary is Claude Code")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("EnsureRulesForIDE did not return within 5s; possible infinite recursion")
	}
}

func TestGenerate_Cursor_AlsoInstallsClaudeCodeWhenDetected(t *testing.T) {
	t.Setenv("CLAUDE_SESSION", "1")
	dir := t.TempDir()

	files, err := Generate(dir, IDECursor)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !cursorRuleExists(dir) {
		t.Error("expected Cursor rule to exist")
	}
	if !claudeCodeRuleExists(dir) {
		t.Error("expected CLAUDE.md to exist (companion install via Generate)")
	}
	claudeMCPPath := filepath.Join(dir, ".claude", "mcp.json")
	if _, err := os.Stat(claudeMCPPath); err != nil {
		t.Errorf("expected Claude MCP config at %s: %v", claudeMCPPath, err)
	}
	// At least 4 distinct files: cursor rule + cursor mcp + CLAUDE.md + .claude/mcp.json.
	if len(files) < 4 {
		t.Errorf("expected at least 4 files from Generate(cursor), got %d: %v", len(files), files)
	}
}

func TestGenerate_VSCode_AlsoInstallsClaudeCompanion(t *testing.T) {
	t.Setenv("CLAUDE_SESSION", "1")
	dir := t.TempDir()

	files, err := Generate(dir, IDEVSCode)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !claudeCodeRuleExists(dir) {
		t.Error("expected CLAUDE.md (companion install via Generate(vscode))")
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "mcp.json")); err != nil {
		t.Errorf("expected .claude/mcp.json: %v", err)
	}
	if len(files) < 3 {
		t.Errorf("expected at least 3 files, got %d", len(files))
	}
}

func TestGenerate_Copilot_AlsoInstallsClaudeCompanion(t *testing.T) {
	t.Setenv("CLAUDE_SESSION", "1")
	dir := t.TempDir()

	files, err := Generate(dir, IDECopilot)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !claudeCodeRuleExists(dir) {
		t.Error("expected CLAUDE.md (companion install via Generate(copilot))")
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "mcp.json")); err != nil {
		t.Errorf("expected .claude/mcp.json: %v", err)
	}
	if len(files) < 3 {
		t.Errorf("expected at least 3 files, got %d", len(files))
	}
}

func TestGenerate_Cursor_NotDetected_NoCompanion(t *testing.T) {
	unsetEnvPrefix(t, "CLAUDE_")
	unsetEnvPrefix(t, "ANTHROPIC_")
	withFakeProcessAncestry(t, "bash", "go", "test-runner")
	dir := t.TempDir()

	files, err := Generate(dir, IDECursor)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if claudeCodeRuleExists(dir) {
		t.Error("CLAUDE.md must NOT be created when Claude Code is not detected")
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "mcp.json")); !os.IsNotExist(err) {
		t.Errorf(".claude/mcp.json must not exist; stat err = %v", err)
	}
	// No companion artifacts should appear in the file list.
	for _, f := range files {
		if strings.Contains(f, ".claude") || strings.HasSuffix(f, "CLAUDE.md") {
			t.Errorf("file list contains Claude artifact but Claude not detected: %s", f)
		}
	}
}
