package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCursorRule(t *testing.T) {
	SetGeneratorVersion("1.2.3")

	dir := t.TempDir()

	path, err := generateCursorRule(dir)
	if err != nil {
		t.Fatalf("generateCursorRule: %v", err)
	}

	expected := filepath.Join(dir, ".cursor", "rules", "gitflow-preflight.mdc")
	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if firstLine(content) != "---" {
		t.Fatalf("expected frontmatter opening on first line, got %q", firstLine(content))
	}
	if !strings.Contains(content, `gitflow_version: "1.2.3"`) {
		t.Fatalf("expected gitflow_version in frontmatter, got:\n%s", content)
	}
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("expected Cursor frontmatter with alwaysApply")
	}
	if !strings.Contains(content, "gitflow --json status") {
		t.Error("expected gitflow CLI reference")
	}
	if strings.Count(content, "Gitflow Pre-flight Check") > 1 {
		t.Error("expected no duplicate Gitflow Pre-flight Check heading")
	}
}

func TestCursorRuleExists(t *testing.T) {
	dir := t.TempDir()

	if cursorRuleExists(dir) {
		t.Error("expected false before generation")
	}

	_, _ = generateCursorRule(dir)
	if !cursorRuleExists(dir) {
		t.Error("expected true after generation")
	}
}

func TestGenerateCopilotInstructions(t *testing.T) {
	SetGeneratorVersion("1.2.3")

	dir := t.TempDir()

	path, err := generateCopilotInstructions(dir)
	if err != nil {
		t.Fatalf("generateCopilotInstructions: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if firstLine(content) != "<!-- gitflow-version: 1.2.3 -->" {
		t.Fatalf("expected version header first line, got %q", firstLine(content))
	}
	if !strings.Contains(content, "Copilot Instructions") {
		t.Error("expected Copilot Instructions header")
	}
	if !strings.Contains(content, "Gitflow Enforcement") {
		t.Error("expected Gitflow Enforcement section")
	}
	if !strings.Contains(content, "When to use the gitflow skill") {
		t.Error("expected skill usage guidance section")
	}
}

func TestCopilotIdempotent(t *testing.T) {
	dir := t.TempDir()

	_, _ = generateCopilotInstructions(dir)
	path, _ := generateCopilotInstructions(dir)

	data, _ := os.ReadFile(path)
	count := strings.Count(string(data), "Gitflow Enforcement")
	if count != 1 {
		t.Errorf("expected 1 occurrence of Gitflow Enforcement, got %d", count)
	}
	guidanceCount := strings.Count(string(data), "When to use the gitflow skill")
	if guidanceCount != 1 {
		t.Errorf("expected 1 occurrence of skill guidance, got %d", guidanceCount)
	}
}

func TestGenerateAgentsMD(t *testing.T) {
	SetGeneratorVersion("1.2.3")

	dir := t.TempDir()

	path, err := generateAgentsMD(dir)
	if err != nil {
		t.Fatalf("generateAgentsMD: %v", err)
	}

	expected := filepath.Join(dir, "AGENTS.md")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}

	data, _ := os.ReadFile(path)
	if firstLine(string(data)) != "<!-- gitflow-version: 1.2.3 -->" {
		t.Fatalf("expected version header first line, got %q", firstLine(string(data)))
	}
	if !strings.Contains(string(data), "Agent Instructions") {
		t.Error("expected Agent Instructions header")
	}
}

func TestAgentsIdempotent(t *testing.T) {
	dir := t.TempDir()

	_, _ = generateAgentsMD(dir)
	if !agentsRuleExists(dir) {
		t.Error("expected agents rule to exist after generation")
	}

	_, _ = generateAgentsMD(dir)
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	count := strings.Count(string(data), "Gitflow Enforcement")
	if count != 1 {
		t.Errorf("expected 1 occurrence, got %d", count)
	}
}

func TestGenerateClaudeCodeRule(t *testing.T) {
	dir := t.TempDir()

	path, err := generateClaudeCodeRule(dir)
	if err != nil {
		t.Fatalf("generateClaudeCodeRule: %v", err)
	}

	expected := filepath.Join(dir, "CLAUDE.md")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "CLAUDE.md") {
		t.Error("expected CLAUDE.md header")
	}
	if !strings.Contains(string(data), "Gitflow Pre-flight Check") {
		t.Error("expected gitflow instructions")
	}
}

func TestClaudeCodeIdempotent(t *testing.T) {
	dir := t.TempDir()
	_, _ = generateClaudeCodeRule(dir)
	if !claudeCodeRuleExists(dir) {
		t.Error("expected rule to exist")
	}
	_, _ = generateClaudeCodeRule(dir)
	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	count := strings.Count(string(data), "Gitflow Pre-flight Check")
	if count != 1 {
		t.Errorf("expected 1 occurrence, got %d", count)
	}
}

func TestGenerateWindsurfRule(t *testing.T) {
	dir := t.TempDir()

	path, err := generateWindsurfRule(dir)
	if err != nil {
		t.Fatalf("generateWindsurfRule: %v", err)
	}

	expected := filepath.Join(dir, ".windsurf", "rules", "gitflow-preflight.md")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Gitflow Pre-flight Check") {
		t.Error("expected gitflow instructions")
	}
}

func TestWindsurfRuleExists(t *testing.T) {
	dir := t.TempDir()
	if windsurfRuleExists(dir) {
		t.Error("expected false before generation")
	}
	_, _ = generateWindsurfRule(dir)
	if !windsurfRuleExists(dir) {
		t.Error("expected true after generation")
	}
}

func TestGenerateClineRule(t *testing.T) {
	dir := t.TempDir()

	path, err := generateClineRule(dir)
	if err != nil {
		t.Fatalf("generateClineRule: %v", err)
	}

	expected := filepath.Join(dir, ".clinerules")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Cline Rules") {
		t.Error("expected Cline Rules header")
	}
}

func TestClineIdempotent(t *testing.T) {
	dir := t.TempDir()
	_, _ = generateClineRule(dir)
	if !clineRuleExists(dir) {
		t.Error("expected rule to exist")
	}
	_, _ = generateClineRule(dir)
	data, _ := os.ReadFile(filepath.Join(dir, ".clinerules"))
	count := strings.Count(string(data), "Gitflow Pre-flight Check")
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestGenerateZedRule(t *testing.T) {
	dir := t.TempDir()

	path, err := generateZedRule(dir)
	if err != nil {
		t.Fatalf("generateZedRule: %v", err)
	}

	expected := filepath.Join(dir, ".zed", "gitflow-instructions.md")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}

	if !zedRuleExists(dir) {
		t.Error("expected rule to exist after generation")
	}
}

func TestGenerateNeovimRule(t *testing.T) {
	dir := t.TempDir()

	path, err := generateNeovimRule(dir)
	if err != nil {
		t.Fatalf("generateNeovimRule: %v", err)
	}

	expected := filepath.Join(dir, ".nvim", "gitflow-instructions.md")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}

	if !neovimRuleExists(dir) {
		t.Error("expected rule to exist after generation")
	}
}

func TestGenerateJetBrainsRule(t *testing.T) {
	dir := t.TempDir()

	path, err := generateJetBrainsRule(dir)
	if err != nil {
		t.Fatalf("generateJetBrainsRule: %v", err)
	}

	expected := filepath.Join(dir, ".idea", "gitflow-instructions.md")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}

	if !jetbrainsRuleExists(dir) {
		t.Error("expected rule to exist after generation")
	}
}

func TestEnsureRulesForIDE_Cursor(t *testing.T) {
	dir := t.TempDir()

	created, err := EnsureRulesForIDE(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"})
	if err != nil {
		t.Fatalf("EnsureRulesForIDE: %v", err)
	}

	// Cursor supports .agents/ so AGENTS.md must NOT be created.
	// Expected: cursor rule + .agents/skills/gitflow/SKILL.md (2 files)
	if len(created) < 2 {
		t.Errorf("expected at least 2 files (cursor rule + skill), got %d", len(created))
	}

	if !cursorRuleExists(dir) {
		t.Error("expected cursor rule to exist")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "gitflow", "SKILL.md")); err != nil {
		t.Error("expected project skill to exist")
	}
	if agentsRuleExists(dir) {
		t.Error("AGENTS.md must NOT be created for IDEs that support .agents/")
	}

	// Idempotent
	created2, _ := EnsureRulesForIDE(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"})
	if len(created2) != 0 {
		t.Errorf("expected 0 files on second call, got %d", len(created2))
	}
}

func TestEnsureRulesForIDE_Unknown(t *testing.T) {
	dir := t.TempDir()
	tmpHome := t.TempDir()
	prev := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return tmpHome, nil }
	defer func() { UserHomeDirFunc = prev }()

	created, err := EnsureRulesForIDE(dir, DetectedIDE{ID: IDEUnknown, DisplayName: "Terminal"})
	if err != nil {
		t.Fatalf("EnsureRulesForIDE: %v", err)
	}

	// Unknown IDE does not support .agents/ so AGENTS.md must be created.
	if len(created) != 2 {
		t.Errorf("expected 2 files (skill + AGENTS.md), got %d: %v", len(created), created)
	}
	if _, err := os.Stat(filepath.Join(tmpHome, ".agents", "skills", "gitflow", "SKILL.md")); err != nil {
		t.Error("expected fallback user skill to exist")
	}
	if !agentsRuleExists(dir) {
		t.Error("expected AGENTS.md for unknown IDE (no .agents/ support)")
	}
}

func TestEnsureRulesForIDE_AllIDEs(t *testing.T) {
	ides := []struct {
		id      string
		display string
	}{
		{IDECursor, "Cursor"},
		{IDEVSCode, "VS Code"},
		{IDECopilot, "VS Code + Copilot"},
		{IDEClaudeCode, "Claude Code"},
		{IDEWindsurf, "Windsurf"},
		{IDECline, "Cline"},
		{IDEZed, "Zed"},
		{IDENeovim, "Neovim"},
		{IDEJetBrains, "JetBrains"},
	}
	for _, tc := range ides {
		t.Run(tc.id, func(t *testing.T) {
			dir := t.TempDir()
			created, err := EnsureRulesForIDE(dir, DetectedIDE{ID: tc.id, DisplayName: tc.display})
			if err != nil {
				t.Fatalf("EnsureRulesForIDE(%s): %v", tc.id, err)
			}
			if len(created) < 1 {
				t.Errorf("expected at least 1 file created for %s", tc.id)
			}
		})
	}
}

func TestGenerate_Cursor(t *testing.T) {
	dir := t.TempDir()
	files, err := Generate(dir, IDECursor)
	if err != nil {
		t.Fatalf("Generate(cursor): %v", err)
	}
	if len(files) < 3 {
		t.Errorf("expected at least 3 files, got %d", len(files))
	}
}

func TestGenerate_Both(t *testing.T) {
	dir := t.TempDir()
	files, err := Generate(dir, IDEBoth)
	if err != nil {
		t.Fatalf("Generate(both): %v", err)
	}
	if len(files) < 4 {
		t.Errorf("expected at least 4 files, got %d: %v", len(files), files)
	}
}

func TestCopilotAppendToExisting(t *testing.T) {
	dir := t.TempDir()
	githubDir := filepath.Join(dir, ".github")
	_ = os.MkdirAll(githubDir, 0755)
	existing := "# My Project\n\nSome existing instructions.\n"
	_ = os.WriteFile(filepath.Join(githubDir, "copilot-instructions.md"), []byte(existing), 0644)

	_, err := generateCopilotInstructions(dir)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(githubDir, "copilot-instructions.md"))
	content := string(data)
	if !strings.Contains(content, "My Project") {
		t.Error("expected existing content preserved")
	}
	if !strings.Contains(content, "Gitflow Enforcement") {
		t.Error("expected gitflow section appended")
	}
	if !strings.Contains(content, "When to use the gitflow skill") {
		t.Error("expected skill usage guidance appended")
	}
}

func TestClaudeCodeAppendToExisting(t *testing.T) {
	dir := t.TempDir()
	existing := "# My Claude Setup\n\nExisting instructions.\n"
	_ = os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(existing), 0644)

	_, err := generateClaudeCodeRule(dir)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	content := string(data)
	if !strings.Contains(content, "My Claude Setup") {
		t.Error("expected existing content preserved")
	}
	if !strings.Contains(content, "Gitflow Enforcement") {
		t.Error("expected gitflow section appended")
	}
}

func TestGenerateSemverCursorRule(t *testing.T) {
	SetGeneratorVersion("1.2.3")

	dir := t.TempDir()

	path, err := generateSemverCursorRule(dir)
	if err != nil {
		t.Fatalf("generateSemverCursorRule: %v", err)
	}
	expected := filepath.Join(dir, ".cursor", "rules", "semver.mdc")
	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if firstLine(content) != "---" {
		t.Fatalf("expected frontmatter opening on first line, got %q", firstLine(content))
	}
	if !strings.Contains(content, `gitflow_version: "1.2.3"`) {
		t.Fatalf("expected gitflow_version in frontmatter, got:\n%s", content)
	}
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("expected frontmatter with alwaysApply")
	}
	if !strings.Contains(content, "feat") || !strings.Contains(content, "BREAKING CHANGE") {
		t.Error("expected conventional commits content")
	}
}

func TestSemverCursorRuleIdempotent(t *testing.T) {
	dir := t.TempDir()
	if semverCursorRuleExists(dir) {
		t.Error("expected false before generation")
	}
	_, _ = generateSemverCursorRule(dir)
	if !semverCursorRuleExists(dir) {
		t.Error("expected true after generation")
	}
}

func TestGenerateSemverCopilotSection(t *testing.T) {
	dir := t.TempDir()

	path, err := generateSemverCopilotSection(dir)
	if err != nil {
		t.Fatalf("generateSemverCopilotSection: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, semverCopilotMarker) {
		t.Error("expected semver section marker in copilot instructions")
	}
	if !strings.Contains(content, "feat") {
		t.Error("expected conventional commits content")
	}
}

func TestSemverCopilotSectionIdempotent(t *testing.T) {
	dir := t.TempDir()
	if semverCopilotSectionExists(dir) {
		t.Error("expected false before generation")
	}
	_, _ = generateSemverCopilotSection(dir)
	if !semverCopilotSectionExists(dir) {
		t.Error("expected true after generation")
	}
	// Second call must not duplicate content
	_, _ = generateSemverCopilotSection(dir)
	data, _ := os.ReadFile(copilotPath(dir))
	count := strings.Count(string(data), semverCopilotMarker)
	if count != 1 {
		t.Errorf("expected semver marker exactly once, found %d times", count)
	}
}

func TestEnsureRulesForIDE_Cursor_HasSemver(t *testing.T) {
	dir := t.TempDir()
	_, err := EnsureRulesForIDE(dir, DetectedIDE{ID: IDECursor, DisplayName: "Cursor"})
	if err != nil {
		t.Fatalf("EnsureRulesForIDE: %v", err)
	}
	if !semverCursorRuleExists(dir) {
		t.Error("expected .cursor/rules/semver.mdc to be created for Cursor")
	}
}

func TestEnsureRulesForIDE_VSCode_HasSemver(t *testing.T) {
	dir := t.TempDir()
	_, err := EnsureRulesForIDE(dir, DetectedIDE{ID: IDEVSCode, DisplayName: "VS Code"})
	if err != nil {
		t.Fatalf("EnsureRulesForIDE: %v", err)
	}
	if !semverCopilotSectionExists(dir) {
		t.Error("expected semver section in copilot-instructions.md for VSCode")
	}
}

func TestEnsureRulesForIDE_Copilot_HasSemver(t *testing.T) {
	dir := t.TempDir()
	_, err := EnsureRulesForIDE(dir, DetectedIDE{ID: IDECopilot, DisplayName: "VS Code + Copilot"})
	if err != nil {
		t.Fatalf("EnsureRulesForIDE: %v", err)
	}
	if !semverCopilotSectionExists(dir) {
		t.Error("expected semver section in copilot-instructions.md for Copilot")
	}
}

// TestGenerateCursorRule_Idempotent verifies that a second call returns "" when
// the content is unchanged (no write is performed).
func TestGenerateCursorRule_Idempotent(t *testing.T) {
	SetGeneratorVersion("1.2.3")
	dir := t.TempDir()

	path1, err := generateCursorRule(dir)
	if err != nil || path1 == "" {
		t.Fatalf("first generateCursorRule: path=%q err=%v", path1, err)
	}
	path2, err := generateCursorRule(dir)
	if err != nil {
		t.Fatalf("second generateCursorRule: %v", err)
	}
	if path2 != "" {
		t.Errorf("expected empty path on idempotent second call, got %q", path2)
	}
}

// TestGenerateCursorRule_RefreshesOnContentChange verifies that a rule is
// regenerated when its on-disk content differs from what the current binary
// would produce — even if the stored version field matches.
func TestGenerateCursorRule_RefreshesOnContentChange(t *testing.T) {
	SetGeneratorVersion("1.2.3")
	dir := t.TempDir()

	path, err := generateCursorRule(dir)
	if err != nil || path == "" {
		t.Fatalf("first generate: %v", err)
	}

	// Corrupt the body (simulates a template update without a version bump).
	data, _ := os.ReadFile(path)
	corrupted := strings.Replace(string(data), "Gitflow Pre-flight Check", "OLD CONTENT", 1)
	if err := os.WriteFile(path, []byte(corrupted), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// fileContentDiffers should detect the change.
	if !fileContentDiffers(path, cursorRuleContent()) {
		t.Fatal("expected fileContentDiffers to return true after body corruption")
	}

	// generate should overwrite and return path.
	path2, err := generateCursorRule(dir)
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if path2 == "" {
		t.Error("expected non-empty path when content was stale")
	}

	data2, _ := os.ReadFile(path)
	if !strings.Contains(string(data2), "Gitflow Pre-flight Check") {
		t.Error("expected restored content after regeneration")
	}
}

// TestGenerateSemverCursorRule_Idempotent mirrors the cursor rule idempotency test.
func TestGenerateSemverCursorRule_Idempotent(t *testing.T) {
	SetGeneratorVersion("1.2.3")
	dir := t.TempDir()

	path1, err := generateSemverCursorRule(dir)
	if err != nil || path1 == "" {
		t.Fatalf("first generateSemverCursorRule: path=%q err=%v", path1, err)
	}
	path2, err := generateSemverCursorRule(dir)
	if err != nil {
		t.Fatalf("second generateSemverCursorRule: %v", err)
	}
	if path2 != "" {
		t.Errorf("expected empty path on idempotent second call, got %q", path2)
	}
}
