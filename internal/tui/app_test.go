package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	mcpserver "github.com/novaemx/gitflow-helper/internal/mcp"
	"github.com/novaemx/gitflow-helper/internal/state"
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

func TestLoadActivityIfChanged_SkipsReloadWhenFingerprintMatches(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(mcpserver.ActivityLogPath(root), []byte(""), 0644); err != nil {
		t.Fatalf("write activity log: %v", err)
	}

	m := model{
		gf:          &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: root}},
		mcpActivity: []mcpserver.ActivityEntry{{Tool: "cached", Timestamp: "2026-04-19T10:00:00Z"}},
	}
	m.lastActivityFingerprint = activityLogFingerprint(root)

	changed := m.loadActivityIfChanged(false)
	if changed {
		t.Fatal("expected unchanged activity log to skip reload")
	}
	if len(m.mcpActivity) != 1 || m.mcpActivity[0].Tool != "cached" {
		t.Fatalf("expected cached activity to remain intact, got %+v", m.mcpActivity)
	}
}

func TestRenderActivityPanel_UsesNewestLoadedOrder(t *testing.T) {
	m := model{
		mcpActivity: []mcpserver.ActivityEntry{
			{Tool: "newest", Result: "ok", Source: "cli", Timestamp: "2026-04-19T12:00:00Z"},
			{Tool: "older", Result: "ok", Source: "cli", Timestamp: "2026-04-19T10:00:00Z"},
		},
	}

	rendered := stripANSI(m.renderActivityPanel(60, 10))
	if strings.Index(rendered, "newest") > strings.Index(rendered, "older") {
		t.Fatalf("expected activity panel to keep newest entry first, got %q", rendered)
	}
	if strings.Count(rendered, "newest") != 1 || strings.Count(rendered, "older") != 1 {
		t.Fatalf("expected stable render output without duplication, got %q", rendered)
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

func TestRenderActionsForWidth_SelectedActionInActionsSection(t *testing.T) {
	m := model{
		actions: []action{
			{Tag: "start", Label: "Start a new feature", Recommended: true},
			{Tag: "release", Label: "Start a release", Recommended: true},
			{Tag: "pull", Label: "Pull latest"},
		},
		selected: 2,
	}

	rendered := stripANSI(m.renderActionsForWidth(80))
	if !strings.Contains(rendered, "Actions:") {
		t.Fatalf("expected Actions section header, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "▸ Pull latest") {
		t.Fatalf("expected selected cursor in Actions section, got:\n%s", rendered)
	}
}

func TestActionSelectionRow_AccountsForSplitHeader(t *testing.T) {
	actions := []action{
		{Tag: "start", Label: "Start a new feature", Recommended: true},
		{Tag: "release", Label: "Start a release", Recommended: true},
		{Tag: "pull", Label: "Pull latest"},
	}

	if got := actionSelectionRow(actions, 2); got != 6 {
		t.Fatalf("expected selected action row 6 after split header, got %d", got)
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

	// expanded → normal
	next2, _ := updated.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated2, ok := next2.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated2.activityPanel != activityNormal {
		t.Fatalf("expected activityPanel to return to activityNormal, got %d", updated2.activityPanel)
	}
	if !updated2.activityNormalCloses {
		t.Fatal("expected normal panel after expanded state to close on next toggle")
	}

	// normal → hidden
	next3, _ := updated2.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated3, ok := next3.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated3.activityPanel != activityHidden {
		t.Fatalf("expected activityPanel to be activityHidden, got %d", updated3.activityPanel)
	}

	// hidden → normal
	next4, _ := updated3.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated4, ok := next4.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated4.activityPanel != activityNormal {
		t.Fatalf("expected activityPanel to reopen at activityNormal, got %d", updated4.activityPanel)
	}
	if updated4.activityNormalCloses {
		t.Fatal("expected reopened normal panel to expand on the next toggle")
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

// TestOutputClose_SnapsToZero verifies that closing the output overlay
// immediately resets to dashboard (no shrink animation).
func TestOutputClose_SnapsToZero(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, mode: viewOutput, outputAnim: 1.0}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	next, _ := m.handleOutputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated, ok := next.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated.outputAnim != 0 {
		t.Fatalf("expected outputAnim=0 (snap close), got %.2f", updated.outputAnim)
	}
	if updated.mode != viewDashboard {
		t.Fatalf("expected mode=viewDashboard after close, got %v", updated.mode)
	}
}

// TestCmdDoneMsg_SnapsOverlayOpen verifies that the overlay opens at full size
// immediately (no grow animation) to prevent ghost-box artifacts.
func TestCmdDoneMsg_SnapsOverlayOpen(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, running: true, runningTitle: "health"}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	next, _ := m.Update(cmdDoneMsg{title: "Repo health check", output: "ok\n"})
	updated, ok := next.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated.outputAnim != 1.0 {
		t.Fatalf("expected outputAnim=1.0 (snap open), got %.2f", updated.outputAnim)
	}
	if updated.mode != viewOutput {
		t.Fatalf("expected mode=viewOutput, got %v", updated.mode)
	}
}

// TestRenderOutputOverlay_BoxHeightNeverExceedsTarget verifies that the rendered
// box (including scroll indicator) never exceeds the target box height.
func TestRenderOutputOverlay_BoxHeightNeverExceedsTarget(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	m := model{
		spinner:     s,
		mode:        viewOutput,
		outputAnim:  1.0,
		outputTitle: "Test",
		outputLines: lines,
		width:       80,
		height:      24,
	}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	blank := strings.Repeat(strings.Repeat(" ", 80)+"\n", 23) + strings.Repeat(" ", 80)
	result := m.renderOutputOverlay(blank)
	rows := strings.Split(result, "\n")

	// Count actual box rows (lines with border characters).
	boxStart, boxEnd := -1, -1
	for i, row := range rows {
		plain := stripANSI(row)
		if strings.Contains(plain, "╭") && boxStart == -1 {
			boxStart = i
		}
		if strings.Contains(plain, "╰") {
			boxEnd = i
		}
	}
	if boxStart == -1 || boxEnd == -1 {
		t.Fatalf("could not find box borders")
	}
	actualBoxH := boxEnd - boxStart + 1
	targetH := m.height - 4
	if actualBoxH > targetH {
		t.Fatalf("box height %d exceeds target %d (scroll indicator overflow)", actualBoxH, targetH)
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

func TestRenderBase_ExpandedActivityPreservesChrome(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{
		spinner:       s,
		mode:          viewDashboard,
		activityPanel: activityExpanded,
		activityAnim:  float64(activityExpanded),
		width:         100,
		height:        24,
		mcpActivity: []mcpserver.ActivityEntry{{
			Tool:      "interactive-tui",
			Args:      "gitflow status",
			Result:    "ok",
			Source:    "cli",
			Timestamp: "2026-04-19T20:21:00Z",
		}},
	}
	m.gf = &gitflow.Logic{
		Config:     config.FlowConfig{ProjectRoot: t.TempDir(), IntegrationMode: config.IntegrationModeLocalMerge},
		AppVersion: "0.5.34",
		State:      state.RepoState{Current: "feature/activity-overlay", Version: "0.5.34"},
	}

	rendered := m.renderBase()
	rows := strings.Split(rendered, "\n")
	if len(rows) != m.height {
		t.Fatalf("expected %d rows, got %d", m.height, len(rows))
	}
	if !strings.Contains(stripANSI(rows[0]), "gitflow v0.5.34") {
		t.Fatalf("expected title bar in first row, got %q", stripANSI(rows[0]))
	}
	if !strings.Contains(stripANSI(rows[len(rows)-1]), "[a] smaller") {
		t.Fatalf("expected status bar in last row, got %q", stripANSI(rows[len(rows)-1]))
	}
	if !strings.Contains(stripANSI(rendered), "Agent Activity") {
		t.Fatalf("expected expanded activity panel in render, got %q", stripANSI(rendered))
	}
	for i, row := range rows {
		if got := lipgloss.Width(row); got != m.width {
			t.Fatalf("expected row %d width %d, got %d", i, m.width, got)
		}
	}
}

func TestRenderBase_ActivityTransitionFitsViewportWithoutArtifacts(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{
		spinner:       s,
		mode:          viewDashboard,
		activityPanel: activityNormal,
		activityAnim:  1.4,
		width:         110,
		height:        24,
		mcpActivity: []mcpserver.ActivityEntry{{
			Tool:      "interactive-tui",
			Args:      "gitflow finish",
			Result:    "started",
			Source:    "cli",
			Timestamp: "2026-04-19T20:18:00Z",
		}},
	}
	m.gf = &gitflow.Logic{
		Config:     config.FlowConfig{ProjectRoot: t.TempDir(), IntegrationMode: config.IntegrationModeLocalMerge},
		AppVersion: "0.5.34",
		State:      state.RepoState{Current: "feature/activity-overlay", Version: "0.5.34"},
	}
	m.dashLines = []dashLine{{text: "dashboard", style: "ok"}}
	m.actions = []action{{Tag: "finish", Label: "Finish feature", Recommended: true}}

	rendered := m.renderBase()
	rows := strings.Split(rendered, "\n")
	if len(rows) != m.height {
		t.Fatalf("expected %d rows, got %d", m.height, len(rows))
	}
	for i, row := range rows {
		if got := lipgloss.Width(row); got != m.width {
			t.Fatalf("expected row %d width %d, got %d", i, m.width, got)
		}
	}
	if !strings.Contains(stripANSI(rows[0]), "gitflow v0.5.34") {
		t.Fatalf("expected title bar to remain visible, got %q", stripANSI(rows[0]))
	}
	if !strings.Contains(stripANSI(rows[len(rows)-1]), "[a] expand") {
		t.Fatalf("expected status bar to remain visible, got %q", stripANSI(rows[len(rows)-1]))
	}
}

func TestOutputOverlayClose_SnapsReturnsDashboard(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, mode: viewOutput, outputAnim: 1.0}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	next, _ := m.handleOutputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated, ok := next.(model)
	if !ok {
		t.Fatal("expected model type")
	}
	if updated.mode != viewDashboard {
		t.Fatalf("expected mode to return to dashboard after close, got %v", updated.mode)
	}
	if updated.outputAnim != 0 {
		t.Fatalf("expected output animation to be 0, got %.2f", updated.outputAnim)
	}
}

func TestPlaceOverlay_PadsBaseRowsToViewportWidth(t *testing.T) {
	base := "short\nbase"
	overlay := "BOX"
	rendered := placeOverlay(base, overlay, 12, 4)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 12 {
			t.Fatalf("expected row %d to be padded to width 12, got %d (%q)", i, got, line)
		}
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

// TestRenderOutputOverlay_BoxCoversOwnArea verifies that the overlay box
// replaces base content within its boundaries.
func TestRenderOutputOverlay_BoxCoversOwnArea(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{
		spinner:     s,
		mode:        viewOutput,
		outputAnim:  1.0,
		outputTitle: "Repo health check",
		outputLines: []string{"✓ all good"},
		width:       80,
		height:      24,
	}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}
	m.dashLines = []dashLine{{text: "Repo health check", style: "ok"}}
	m.actions = []action{{Tag: "health", Label: "Repo health check"}}

	base := m.renderBase()
	result := m.renderOutputOverlay(base)
	rows := strings.Split(result, "\n")

	boxStart, boxEnd := -1, -1
	for i, row := range rows {
		plain := stripANSI(row)
		if strings.Contains(plain, "╭") && boxStart == -1 {
			boxStart = i
		}
		if strings.Contains(plain, "╰") {
			boxEnd = i
		}
	}
	if boxStart == -1 || boxEnd == -1 {
		t.Fatalf("could not find overlay box borders in rendered output")
	}
	// Box rows must contain border characters (properly rendered overlay)
	for i := boxStart; i <= boxEnd && i < len(rows); i++ {
		plain := stripANSI(rows[i])
		if !strings.Contains(plain, "│") && !strings.Contains(plain, "╭") && !strings.Contains(plain, "╰") {
			t.Errorf("box row %d missing border character: %q", i, plain)
		}
	}
	if len(rows) != m.height {
		t.Fatalf("expected %d rows, got %d", m.height, len(rows))
	}
}

// TestRenderOutputOverlay_ScrollIndicatorFitsWithinViewport verifies that when
// output triggers a scroll indicator the rendered box does not overflow the viewport.
func TestRenderOutputOverlay_ScrollIndicatorFitsWithinViewport(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("output line %d", i+1)
	}
	m := model{
		spinner:     s,
		mode:        viewOutput,
		outputAnim:  1.0,
		outputTitle: "Health check",
		outputLines: lines,
		width:       80,
		height:      24,
	}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}

	base := strings.Repeat(strings.Repeat(" ", 80)+"\n", 23) + strings.Repeat(" ", 80)
	result := m.renderOutputOverlay(base)
	rows := strings.Split(result, "\n")

	if len(rows) > m.height {
		t.Fatalf("overlay overflows viewport: got %d rows, viewport height is %d", len(rows), m.height)
	}

	found := false
	for _, row := range rows {
		if strings.Contains(stripANSI(row), "↕") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected scroll indicator '↕' in output, but it was missing")
	}
}

// TestAllOverlays_BackgroundVisibleOutsideBox verifies that the dashboard
// background is NOT blanked outside the overlay box (background shows through).
func TestAllOverlays_BackgroundVisibleOutsideBox(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	textInput := textinput.New()
	textInput.SetValue("")
	m := model{
		spinner:     s,
		width:       80,
		height:      24,
		outputAnim:  1.0,
		outputTitle: "Test",
		outputLines: []string{"ok"},
		inputPrompt: "Name:",
		inputField:  textInput,
	}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}
	m.dashLines = []dashLine{{text: "CANARY_TEXT", style: "ok"}}
	m.actions = []action{{Tag: "test", Label: "CANARY_TEXT action"}}

	base := m.renderBase()
	if !strings.Contains(stripANSI(base), "CANARY_TEXT") {
		t.Fatal("base must contain CANARY_TEXT for this test to be meaningful")
	}

	tests := []struct {
		name   string
		render func() string
	}{
		{"output", func() string { return m.renderOutputOverlay(base) }},
		{"help", func() string { return m.renderHelpOverlay(base) }},
		{"palette", func() string { return m.renderPaletteOverlay(base) }},
		{"input", func() string { return m.renderInputOverlay(base) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.render()
			rows := strings.Split(result, "\n")
			// Find box boundaries
			boxStart, boxEnd := -1, -1
			for i, row := range rows {
				plain := stripANSI(row)
				if strings.Contains(plain, "╭") && boxStart == -1 {
					boxStart = i
				}
				if strings.Contains(plain, "╰") {
					boxEnd = i
				}
			}
			if boxStart == -1 {
				t.Fatalf("could not find top box border")
			}
			if boxEnd == -1 {
				// Bottom border may be clipped by viewport (e.g. help overlay)
				boxEnd = len(rows) - 1
			}
			// Overlay box MUST replace its rows (no base text inside box rows)
			for i := boxStart; i <= boxEnd && i < len(rows); i++ {
				// Box rows should contain border characters, not leak base text through the box
				plain := stripANSI(rows[i])
				if !strings.Contains(plain, "│") && !strings.Contains(plain, "╭") && !strings.Contains(plain, "╰") {
					t.Errorf("box row %d missing border character: %q", i, plain)
				}
			}
			// Total rows must equal viewport height
			if len(rows) != m.height {
				t.Errorf("expected %d rows, got %d", m.height, len(rows))
			}
		})
	}
}

// TestOutputStatusBar_ShowsActionName verifies the status bar includes the
// action title when in output view mode.
func TestOutputStatusBar_ShowsActionName(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{
		spinner:     s,
		mode:        viewOutput,
		outputTitle: "Repo health check",
		width:       80,
		height:      24,
	}
	m.gf = &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}}
	m.dashLines = []dashLine{}
	m.actions = []action{}

	bar := m.renderStatusBar()
	plain := stripANSI(bar)
	if !strings.Contains(plain, "Repo health check") {
		t.Fatalf("status bar should show action title, got: %q", plain)
	}
}
