package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	mcpserver "github.com/novaemx/gitflow-helper/internal/mcp"
)

func TestResolveGitDir_DirectoryDotGit(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	got := resolveGitDir(root)
	if got != gitDir {
		t.Fatalf("expected %q, got %q", gitDir, got)
	}
}

func TestResolveGitDir_GitdirFile(t *testing.T) {
	root := t.TempDir()
	realGitDir := filepath.Join(root, ".worktrees", "wt1")
	if err := os.MkdirAll(realGitDir, 0755); err != nil {
		t.Fatalf("mkdir gitdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: .worktrees/wt1\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := resolveGitDir(root)
	if got != realGitDir {
		t.Fatalf("expected %q, got %q", realGitDir, got)
	}
}

func TestResolveGitDir_RejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	traversal := filepath.Join("..", filepath.Base(outside))
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+traversal+"\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := resolveGitDir(root)
	if got != "" {
		t.Fatalf("expected traversal path rejected, got %q", got)
	}
}

func TestResolveGitDir_AbsoluteGitdirPath(t *testing.T) {
	root := t.TempDir()
	absGitDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+absGitDir+"\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := resolveGitDir(root)
	if got != absGitDir {
		t.Fatalf("expected absolute gitdir %q, got %q", absGitDir, got)
	}
}

func TestResolveGitDir_InvalidGitFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not-a-gitdir-line\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := resolveGitDir(root)
	if got != "" {
		t.Fatalf("expected empty result for invalid git file, got %q", got)
	}
}

func TestRepoFingerprint_ChangesWhenHeadChanges(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0755); err != nil {
		t.Fatalf("mkdir refs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/develop\n"), 0644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "develop"), []byte("abc123\n"), 0644); err != nil {
		t.Fatalf("write develop ref: %v", err)
	}

	before := repoFingerprint(root)
	if before == "" {
		t.Fatal("expected non-empty fingerprint")
	}

	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatalf("rewrite HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte("def456\n"), 0644); err != nil {
		t.Fatalf("write main ref: %v", err)
	}

	after := repoFingerprint(root)
	if after == before {
		t.Fatal("expected fingerprint to change after branch head change")
	}
}

func TestSelectionIndexForRefresh_PreservesExactAction(t *testing.T) {
	actions := []action{
		{Tag: "pull", Label: "Pull latest"},
		{Tag: "finish", Label: "Finish bugfix"},
	}
	prev := action{Tag: "finish", Label: "Finish bugfix"}

	got := selectionIndexForRefresh(actions, &prev)
	if got != 1 {
		t.Fatalf("expected index 1, got %d", got)
	}
}

func TestSelectionIndexForRefresh_FallbackByTag(t *testing.T) {
	actions := []action{
		{Tag: "pull", Label: "Pull latest"},
		{Tag: "finish", Label: "Finish feature alpha"},
	}
	prev := action{Tag: "finish", Label: "Finish feature beta"}

	got := selectionIndexForRefresh(actions, &prev)
	if got != 1 {
		t.Fatalf("expected tag fallback index 1, got %d", got)
	}
}

func TestSelectionIndexForRefresh_DefaultRecommended(t *testing.T) {
	actions := []action{
		{Tag: "pull", Label: "Pull latest"},
		{Tag: "start", Label: "Start feature", Recommended: true},
		{Tag: "finish", Label: "Finish bugfix"},
	}
	prev := action{Tag: "unknown", Label: "Unknown"}

	got := selectionIndexForRefresh(actions, &prev)
	if got != 1 {
		t.Fatalf("expected recommended index 1, got %d", got)
	}
}

func TestRenderDashboard_DividerUsesViewportWidth(t *testing.T) {
	m := model{
		width: 20,
		dashLines: []dashLine{
			{text: dashboardDividerToken, style: "dim"},
		},
	}

	rendered := m.renderDashboard()
	if !strings.Contains(rendered, strings.Repeat("-", 18)) {
		t.Fatalf("expected dynamic divider sized to viewport, got %q", rendered)
	}
}

func TestRenderStatusBar_ShowsRunningAction(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse

	m := model{
		width:        120,
		running:      true,
		runningTitle: "Finish release v1.2.3",
		spinner:      s,
	}

	rendered := m.renderStatusBar()
	if !strings.Contains(rendered, "Running: Finish release v1.2.3") {
		t.Fatalf("expected running status in status bar, got %q", rendered)
	}
}

func TestStartCommand_SetsRunningState(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse

	m := model{spinner: s}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}
	next, _ := m.startCommand(action{Label: "Sync branches", Command: "echo ok"})

	updated, ok := next.(model)
	if !ok {
		t.Fatalf("expected model type after startCommand")
	}
	if !updated.running {
		t.Fatalf("expected running to be true")
	}
	if updated.runningTitle != "Sync branches" {
		t.Fatalf("expected runningTitle to be set, got %q", updated.runningTitle)
	}
}

func TestRenderActivityPanel_ShowsDetailsWithoutSubtitle(t *testing.T) {
	m := model{
		mcpActivity: []mcpserver.ActivityEntry{{
			Tool:      "interactive-tui",
			Args:      "gitflow push",
			Result:    "started",
			Source:    "cli",
			Timestamp: "2026-04-14T03:35:32Z",
		}},
	}

	rendered := stripANSI(m.renderActivityPanel(60, 10))
	if strings.Contains(rendered, "MCP + CLI") {
		t.Fatalf("did not expect legacy subtitle in activity panel: %q", rendered)
	}
	if !strings.Contains(rendered, "interactive-tui") {
		t.Fatalf("expected CLI activity details in panel, got %q", rendered)
	}
	if got := len(strings.Split(rendered, "\n")); got < 10 {
		t.Fatalf("expected panel to use full available height, got %d lines", got)
	}
}

func TestRenderActionsForWidth_AllActionsHavePrefix(t *testing.T) {
	m := model{
		actions: []action{
			{Tag: "start", Label: "Start a new feature", Recommended: true},
			{Tag: "pull", Label: "Pull latest"},
			{Tag: "push", Label: "Push current branch"},
		},
		selected: 0,
	}

	rendered := stripANSI(m.renderActionsForWidth(80))
	// Non-recommended items should have the dim ▹ prefix so the cursor is trackable
	if !strings.Contains(rendered, "▹ Pull latest") {
		t.Fatalf("expected dim ▹ prefix on non-recommended action, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "▹ Push current branch") {
		t.Fatalf("expected dim ▹ prefix on second non-recommended action, got:\n%s", rendered)
	}
}

func TestActivityPanel_DefaultsNormal(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{gf: nil, mode: viewDashboard, spinner: s, activityPanel: activityNormal}
	if m.activityPanel != activityNormal {
		t.Fatal("expected activityPanel to default to activityNormal")
	}
}

func TestActivityPanel_CyclesOnKeyA(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, activityPanel: activityNormal, mode: viewDashboard}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	// normal → expanded
	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated, ok := next.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated.activityPanel != activityExpanded {
		t.Fatalf("expected activityPanel to be activityExpanded, got %d", updated.activityPanel)
	}

	// expanded → hidden
	next2, _ := updated.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated2, ok := next2.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated2.activityPanel != activityHidden {
		t.Fatalf("expected activityPanel to be activityHidden, got %d", updated2.activityPanel)
	}

	// hidden → normal
	next3, _ := updated2.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated3, ok := next3.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated3.activityPanel != activityNormal {
		t.Fatalf("expected activityPanel to cycle back to activityNormal, got %d", updated3.activityPanel)
	}
}

func TestActivityAnimationTick_MovesTowardTargetState(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, mode: viewDashboard, activityPanel: activityExpanded, activityAnim: float64(activityNormal)}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	next, _ := m.Update(uiAnimTickMsg{})
	updated, ok := next.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated.activityAnim <= float64(activityNormal) {
		t.Fatalf("expected activity animation to advance, got %.2f", updated.activityAnim)
	}
}

// TestActivityAnimation_RequiresMinFrames verifies that the activity panel
// animation is slow enough to be visually perceptible (≥8 frames ≈ 128ms).
func TestActivityAnimation_RequiresMinFrames(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, mode: viewDashboard, activityPanel: activityNormal, activityAnim: 0}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	frames := 0
	for m.activityAnim < float64(activityNormal)-0.001 {
		next, _ := m.Update(uiAnimTickMsg{})
		cast, ok := next.(model)
		if !ok {
			t.Fatal("expected model type")
		}
		m = cast
		frames++
		if frames > 100 {
			t.Fatal("animation did not converge")
		}
	}
	if frames < 8 {
		t.Fatalf("animation too fast: %d frames (need ≥8 for ~128ms perceptible transition)", frames)
	}
}

// TestOutputAnimation_RequiresMinFrames verifies output overlay animation
// takes ≥8 frames to open fully.
func TestOutputAnimation_RequiresMinFrames(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, mode: viewOutput, outputAnim: 0, outputClosing: false}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	frames := 0
	for m.outputAnim < 1.0-0.001 {
		next, _ := m.Update(uiAnimTickMsg{})
		cast, ok := next.(model)
		if !ok {
			t.Fatal("expected model type")
		}
		m = cast
		frames++
		if frames > 100 {
			t.Fatal("animation did not converge")
		}
	}
	if frames < 8 {
		t.Fatalf("output animation too fast: %d frames (need ≥8 for ~128ms perceptible transition)", frames)
	}
}

// TestActivityPanel_AnimatesVisiblyOnNarrowTerminal verifies that the activity
// panel animates with a non-zero width even when the terminal is narrower than
// 100 columns.  Previous bug: normalW was 0 when width < 100.
func TestActivityPanel_AnimatesVisiblyOnNarrowTerminal(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{
		spinner:       s,
		mode:          viewDashboard,
		activityPanel: activityNormal,
		activityAnim:  0.5, // mid-animation
		width:         80,
		height:        24,
	}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}
	m.dashLines = []dashLine{}
	m.actions = []action{}

	base := m.renderBase()
	// The rendered output must contain "Agent Activity" from the panel.
	// If normalW was 0, rightW would be 0 and the panel would be invisible.
	if !strings.Contains(base, "Agent Activity") {
		t.Fatal("activity panel not visible at width=80 with anim=0.5; normalW is likely 0")
	}
}

func TestOutputOverlayClose_AnimatesThenReturnsDashboard(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, mode: viewOutput, outputAnim: 1.0}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	next, _ := m.handleOutputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated, ok := next.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if !updated.outputClosing {
		t.Fatal("expected outputClosing to be enabled after close key")
	}

	cur := updated
	for i := 0; i < 50; i++ {
		nextModel, _ := cur.Update(uiAnimTickMsg{})
		cast, ok := nextModel.(model)
		if !ok {
			t.Fatal("expected model type during animation")
		}
		cur = cast
		if cur.mode == viewDashboard && !cur.outputClosing {
			break
		}
	}

	if cur.mode != viewDashboard {
		t.Fatalf("expected mode to return to dashboard after close animation, got %v", cur.mode)
	}
	if cur.outputAnim != 0 {
		t.Fatalf("expected output animation to end at 0, got %.2f", cur.outputAnim)
	}
}

func TestIntegrationModeToggle_TogglesOnModeShortcut(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, mode: viewDashboard}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	updated, ok := next.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if !updated.running {
		t.Fatal("expected mode shortcut to trigger mode toggle command")
	}
	if updated.runningTitle != "Toggle integration mode" {
		t.Fatalf("expected mode toggle title, got %q", updated.runningTitle)
	}
}
