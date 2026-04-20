package tui

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	"github.com/novaemx/gitflow-helper/internal/ide"
	mcpserver "github.com/novaemx/gitflow-helper/internal/mcp"
)

type viewMode int

const (
	viewDashboard viewMode = iota
	viewOutput
	viewHelp
	viewInput
)

type activityPanelState int

const (
	activityHidden   activityPanelState = 0
	activityNormal   activityPanelState = 1
	activityExpanded activityPanelState = 2
)

const (
	activityPollInterval = 2 * time.Second
	repoWatchInterval    = 2 * time.Second
)

type model struct {
	gf        *gitflow.Logic
	actions   []action
	dashLines []dashLine
	selected  int
	scroll    int
	width     int
	height    int
	mode      viewMode
	quitting  bool

	// Command output overlay
	outputTitle  string
	outputLines  []string
	outputScroll int
	running      bool
	runningTitle string
	spinner      spinner.Model

	// Input overlay
	inputPrompt   string
	inputField    textinput.Model
	pendingAction *action

	// IDE activity from MCP server
	mcpActivity []mcpserver.ActivityEntry

	// Activity panel state (0=hidden, 1=normal right panel, 2=expanded full-width)
	activityPanel        activityPanelState
	activityAnim         float64
	activityNormalCloses bool

	// Output overlay animation state.
	outputAnim    float64
	outputClosing bool

	// Git state watch
	lastGitFingerprint      string
	lastActivityFingerprint string
}

type refreshMsg struct{}
type activityTickMsg struct{}
type watchTickMsg struct{}
type uiAnimTickMsg struct{}

type cmdDoneMsg struct {
	title  string
	output string
	err    error
}

func Run(gf *gitflow.Logic) error {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{gf: gf, mode: viewDashboard, spinner: s, activityPanel: activityNormal, activityAnim: float64(activityNormal)}
	m.refresh(false)

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func actionIdentity(a action) string {
	return a.Tag + "\x00" + a.Label
}

func defaultSelection(actions []action) int {
	if len(actions) == 0 {
		return 0
	}
	for i, a := range actions {
		if a.Recommended {
			return i
		}
	}
	return 0
}

func selectionIndexForRefresh(actions []action, prev *action) int {
	if len(actions) == 0 {
		return 0
	}
	if prev == nil {
		return defaultSelection(actions)
	}

	prevID := actionIdentity(*prev)
	for i, a := range actions {
		if actionIdentity(a) == prevID {
			return i
		}
	}
	for i, a := range actions {
		if a.Tag == prev.Tag {
			return i
		}
	}
	return defaultSelection(actions)
}

func lastRecommendedActionIndex(actions []action) int {
	lastRec := -1
	for i, a := range actions {
		if a.Recommended {
			lastRec = i
		}
	}
	return lastRec
}

func actionSelectionRow(actions []action, selected int) int {
	if selected < 0 {
		selected = 0
	}

	row := 2 + selected
	lastRec := lastRecommendedActionIndex(actions)
	if lastRec >= 0 && selected > lastRec {
		row += 2
	}
	return row
}

func (m *model) refresh(preserveSelection bool) {
	var prev *action
	if preserveSelection && m.selected >= 0 && m.selected < len(m.actions) {
		p := m.actions[m.selected]
		prev = &p
	}

	m.gf.Refresh()
	m.actions = buildActions(m.gf.State, m.gf.Config)
	m.dashLines = buildDashboardLines(m.gf.State, m.gf.Config)
	m.selected = selectionIndexForRefresh(m.actions, prev)
	m.scroll = 0
}

func (m *model) shouldPollProtectedBranchState() bool {
	if m.gf == nil {
		return false
	}
	return m.gf.State.Current == m.gf.Config.DevelopBranch || m.gf.State.Current == m.gf.Config.MainBranch
}

func resolveGitDir(root string) string {
	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return gitPath
	}

	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(strings.ToLower(line), "gitdir:") {
		return ""
	}
	rel := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	resolved := filepath.Clean(filepath.Join(root, rel))
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return ""
	}
	prefix := filepath.Clean(absRoot) + string(os.PathSeparator)
	if absResolved != filepath.Clean(absRoot) && !strings.HasPrefix(absResolved, prefix) {
		return ""
	}
	return resolved
}

func statPart(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "-"
	}
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}

func truncateRunes(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-3]) + "..."
}

func repoFingerprint(root string) string {
	gitDir := resolveGitDir(root)
	if gitDir == "" {
		return ""
	}

	// Keep the watcher cheap: rely on filesystem metadata that Git updates for
	// refs, index, merge state, and reflogs instead of shelling out each tick.
	parts := []string{fmt.Sprintf("gitdir:%s", gitDir)}
	headPath := filepath.Join(gitDir, "HEAD")
	headData, _ := os.ReadFile(headPath)
	head := strings.TrimSpace(string(headData))
	parts = append(parts, "HEAD="+head)
	parts = append(parts, "head.stat="+statPart(headPath))

	if strings.HasPrefix(head, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(head, "ref:"))
		if ref != "" {
			parts = append(parts, "ref="+ref)
			parts = append(parts, "ref.stat="+statPart(filepath.Join(gitDir, filepath.FromSlash(ref))))
		}
	}

	metaFiles := []string{
		"index",
		"ORIG_HEAD",
		"MERGE_HEAD",
		"CHERRY_PICK_HEAD",
		"REBASE_HEAD",
		"packed-refs",
		filepath.Join("logs", "HEAD"),
		filepath.Join("refs", "heads"),
		filepath.Join("refs", "tags"),
		filepath.Join("logs", "refs", "heads"),
	}
	for _, rel := range metaFiles {
		parts = append(parts, rel+"="+statPart(filepath.Join(gitDir, filepath.FromSlash(rel))))
	}

	return strings.Join(parts, "|")
}

func activityLogFingerprint(root string) string {
	return statPart(mcpserver.ActivityLogPath(root))
}

func (m *model) loadActivityIfChanged(force bool) bool {
	if m.gf == nil {
		m.mcpActivity = nil
		m.lastActivityFingerprint = ""
		return true
	}

	fingerprint := activityLogFingerprint(m.gf.Config.ProjectRoot)
	if !force && fingerprint == m.lastActivityFingerprint {
		return false
	}

	m.mcpActivity = mcpserver.ReadActivityLog(m.gf.Config.ProjectRoot, 20)
	m.lastActivityFingerprint = fingerprint
	return true
}

func (m model) Init() tea.Cmd {
	m.lastGitFingerprint = repoFingerprint(m.gf.Config.ProjectRoot)
	m.loadActivityIfChanged(true)
	return tea.Batch(
		tea.Tick(activityPollInterval, func(t time.Time) tea.Msg { return activityTickMsg{} }),
		tea.Tick(repoWatchInterval, func(t time.Time) tea.Msg { return watchTickMsg{} }),
		m.spinner.Tick,
	)
}

func animateToward(current, target, step float64) float64 {
	if current == target {
		return current
	}

	delta := target - current
	dist := math.Abs(delta)

	// Ease movement by distance so panel transitions feel smoother near the end
	// while still starting with enough speed on larger moves.
	dynStep := dist * 0.22
	if dynStep < step {
		dynStep = step
	}
	if dynStep > 0.20 {
		dynStep = 0.20
	}

	next := current + math.Copysign(dynStep, delta)
	if math.Abs(target-next) < 0.01 {
		return target
	}

	if current < target {
		return math.Min(next, target)
	}
	if current > target {
		return math.Max(next, target)
	}
	return current
}

func (m model) hasPendingAnimations() bool {
	const eps = 0.001
	if math.Abs(m.activityAnim-float64(m.activityPanel)) > eps {
		return true
	}
	return false
}

func (m model) animationTickCmd() tea.Cmd {
	if !m.hasPendingAnimations() {
		return nil
	}
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg { return uiAnimTickMsg{} })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case cmdDoneMsg:
		m.outputTitle = msg.title
		lines := strings.Split(stripANSI(msg.output), "\n")
		if msg.err != nil {
			lines = append(lines, "", errorStyle.Render("Error: "+msg.err.Error()))
		}
		m.outputLines = lines
		m.outputScroll = 0
		m.running = false
		m.runningTitle = ""
		m.mode = viewOutput
		m.outputClosing = false
		m.outputAnim = 1 // snap open — no grow animation to prevent ghost-box artifacts
		m.loadActivityIfChanged(true)
		m.refresh(false)
		return m, nil

	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case refreshMsg:
		m.refresh(false)
		return m, nil

	case activityTickMsg:
		m.loadActivityIfChanged(false)
		return m, tea.Tick(activityPollInterval, func(t time.Time) tea.Msg {
			return activityTickMsg{}
		})

	case watchTickMsg:
		fp := repoFingerprint(m.gf.Config.ProjectRoot)
		shouldRefresh := fp != m.lastGitFingerprint
		if !shouldRefresh && m.mode == viewDashboard && m.shouldPollProtectedBranchState() {
			shouldRefresh = true
		}
		if shouldRefresh && m.mode == viewDashboard {
			m.refresh(true)
		}
		m.lastGitFingerprint = fp
		return m, tea.Tick(repoWatchInterval, func(t time.Time) tea.Msg {
			return watchTickMsg{}
		})

	case uiAnimTickMsg:
		m.activityAnim = animateToward(m.activityAnim, float64(m.activityPanel), 0.06)
		return m, m.animationTickCmd()

	case tea.KeyMsg:
		switch m.mode {
		case viewOutput:
			return m.handleOutputKey(msg)
		case viewHelp:
			m.mode = viewDashboard
			return m, nil
		case viewInput:
			return m.handleInputKey(msg)
		default:
			return m.handleKey(msg)
		}

	default:
		if m.mode == viewInput {
			var cmd tea.Cmd
			m.inputField, cmd = m.inputField.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.running {
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("?"))):
			m.mode = viewHelp
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
		m.quitting = true
		return m, tea.Quit
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		if m.selected < len(m.actions)-1 {
			m.selected++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.selected > 0 {
			m.selected--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("g", "home"))):
		m.selected = 0
		m.scroll = 0
	case key.Matches(msg, key.NewBinding(key.WithKeys("G", "end"))):
		m.selected = len(m.actions) - 1
	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+m", "m"))):
		return m.startCommand(action{Label: "Toggle integration mode", Command: "gitflow mode toggle"})
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if m.selected < len(m.actions) {
			a := m.actions[m.selected]
			if a.Tag == "exit" {
				m.quitting = true
				return m, tea.Quit
			}
			if a.NeedsInput {
				pending := a
				m.pendingAction = &pending
				m.inputPrompt = a.InputPrompt
				ti := textinput.New()
				ti.Placeholder = a.InputDefault
				ti.SetValue(a.InputDefault)
				ti.Focus()
				ti.CharLimit = 64
				ti.Width = 40
				m.inputField = ti
				m.mode = viewInput
				return m, ti.Cursor.BlinkCmd()
			}
			if a.Command != "" {
				return m.startCommand(a)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("?"))):
		m.mode = viewHelp
	case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
		switch m.activityPanel {
		case activityHidden:
			m.activityPanel = activityNormal
			m.activityNormalCloses = false
		case activityNormal:
			if m.activityNormalCloses {
				m.activityPanel = activityHidden
				m.activityNormalCloses = false
			} else {
				m.activityPanel = activityExpanded
			}
		case activityExpanded:
			m.activityPanel = activityNormal
			m.activityNormalCloses = true
		}
		return m, m.animationTickCmd()
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		m.refresh(false)
	}
	return m, nil
}

func (m model) handleOutputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxScroll := len(m.outputLines) - (m.height - 6)
	if maxScroll < 0 {
		maxScroll = 0
	}
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc", "enter"))):
		// Snap-close: no shrink animation to prevent ghost-box artifacts.
		m.outputAnim = 0
		m.outputClosing = false
		m.mode = viewDashboard
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		if m.outputScroll < maxScroll {
			m.outputScroll++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.outputScroll > 0 {
			m.outputScroll--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("g", "home"))):
		m.outputScroll = 0
	case key.Matches(msg, key.NewBinding(key.WithKeys("G", "end"))):
		m.outputScroll = maxScroll
	}
	return m, nil
}

func (m model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = viewDashboard
		m.pendingAction = nil
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		val := m.inputField.Value()
		m.mode = viewDashboard
		if val != "" && m.pendingAction != nil {
			finalCmd := fmt.Sprintf(m.pendingAction.Command, val)
			a := action{
				Label:   m.pendingAction.Label,
				Tag:     m.pendingAction.Tag,
				Command: finalCmd,
			}
			m.pendingAction = nil
			return m.startCommand(a)
		}
		m.pendingAction = nil
		return m, nil
	}
	var cmd tea.Cmd
	m.inputField, cmd = m.inputField.Update(msg)
	return m, cmd
}

func (m model) startCommand(a action) (tea.Model, tea.Cmd) {
	m.running = true
	m.runningTitle = a.Label
	return m, tea.Batch(m.spinner.Tick, m.runCommandAsync(a))
}

// runCommandAsync executes a shell command in the background, captures output,
// and sends it back to the TUI as a message — never leaves the AltScreen.
func (m model) runCommandAsync(a action) tea.Cmd {
	cmdStr := a.Command
	label := a.Label
	projectRoot := m.gf.Config.ProjectRoot
	return func() tea.Msg {
		_ = mcpserver.AppendActivityLog(projectRoot, mcpserver.ActivityEntry{
			Tool:   label,
			Args:   cmdStr,
			Result: "started",
			Source: "cli",
		})

		cmd := BuildExecCmd(cmdStr, projectRoot)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
		result := "ok"
		errMsg := ""
		if err != nil {
			result = "error"
			errMsg = err.Error()
		}
		_ = mcpserver.AppendActivityLog(projectRoot, mcpserver.ActivityEntry{
			Tool:   label,
			Args:   cmdStr,
			Result: result,
			Error:  errMsg,
			Source: "cli",
		})

		return cmdDoneMsg{
			title:  label,
			output: buf.String(),
			err:    err,
		}
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	base := m.renderBase()

	switch m.mode {
	case viewOutput:
		return m.renderOutputOverlay(m.blankViewport())
	case viewHelp:
		return m.renderHelpOverlay(base)
	case viewInput:
		return m.renderInputOverlay(base)
	}

	return base
}

func (m model) blankViewport() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	line := strings.Repeat(" ", m.width)
	rows := make([]string, m.height)
	for i := range rows {
		rows[i] = line
	}
	return strings.Join(rows, "\n")
}

func (m model) renderBase() string {
	var sections []string
	sections = append(sections, m.renderTitleBar())

	contentHeight := m.height - 3

	normalW := 0
	if m.width >= 100 {
		normalW = 44
		if normalW > m.width/2 {
			normalW = m.width / 2
		}
	} else if m.width >= 60 {
		normalW = 33
	} else {
		normalW = 27
	}
	fullW := m.width
	if fullW < 24 {
		fullW = 24
	}

	anim := m.activityAnim
	rightW := 0
	switch {
	case anim <= 0.01:
		rightW = 0
	case anim <= 1.0:
		rightW = int(math.Round(float64(normalW) * anim))
	case normalW > 0:
		t := anim - 1.0
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		rightW = int(math.Round(float64(normalW) + (float64(fullW-normalW) * t)))
	default:
		rightW = fullW
	}

	if rightW >= m.width-3 {
		panel := m.renderActivityPanel(m.width, contentHeight)
		sections = append(sections, panel)
		sections = append(sections, m.renderStatusBar())
		return strings.Join(sections, "\n")
	}

	leftW := m.width
	if rightW > 0 {
		leftW = m.width - rightW - 1
	}
	if leftW < 40 {
		leftW = 40
	}

	dashContent := m.renderDashboardForWidth(leftW)
	actionContent := m.renderActionsForWidth(leftW)
	leftLines := strings.Split(dashContent+"\n"+actionContent, "\n")

	dashLineCount := len(strings.Split(dashContent, "\n"))
	selectedRow := dashLineCount + actionSelectionRow(m.actions, m.selected)
	if selectedRow-m.scroll >= contentHeight {
		m.scroll = selectedRow - contentHeight + 1
	}
	if selectedRow-m.scroll < 0 {
		m.scroll = selectedRow
	}
	if m.scroll < 0 {
		m.scroll = 0
	}

	leftEnd := m.scroll + contentHeight
	if leftEnd > len(leftLines) {
		leftEnd = len(leftLines)
	}
	leftStart := m.scroll
	if leftStart > len(leftLines) {
		leftStart = len(leftLines)
	}
	visibleLeft := leftLines[leftStart:leftEnd]
	for len(visibleLeft) < contentHeight {
		visibleLeft = append(visibleLeft, "")
	}

	if rightW == 0 {
		sections = append(sections, strings.Join(visibleLeft, "\n"))
		sections = append(sections, m.renderStatusBar())
		return strings.Join(sections, "\n")
	}

	rightPanelWidth := rightW
	if rightPanelWidth > 0 {
		rightPanelWidth--
	}
	rightPanel := m.renderActivityPanel(rightPanelWidth, contentHeight)
	rightLines := strings.Split(rightPanel, "\n")
	if len(rightLines) > contentHeight {
		rightLines = rightLines[:contentHeight]
	}
	for len(rightLines) < contentHeight {
		rightLines = append(rightLines, "")
	}

	rows := make([]string, 0, contentHeight)
	for i := 0; i < contentHeight; i++ {
		leftLine := visibleLeft[i]
		lw := lipgloss.Width(leftLine)
		if lw < leftW {
			leftLine += strings.Repeat(" ", leftW-lw)
		}
		rows = append(rows, leftLine+" "+rightLines[i])
	}

	sections = append(sections, strings.Join(rows, "\n"))
	sections = append(sections, m.renderStatusBar())

	return strings.Join(sections, "\n")
}

func (m model) renderTitleBar() string {
	s := m.gf.State

	pname := ""
	parts := strings.Split(m.gf.Config.ProjectRoot, string(os.PathSeparator))
	if len(parts) > 0 {
		pname = parts[len(parts)-1]
	}

	appVer := m.gf.AppVersion
	if appVer == "" {
		appVer = "dev"
	}
	left1 := " gitflow v" + appVer

	ideName := m.gf.IDEDisplay()
	if ideName == "" || m.gf.IDE.ID == ide.IDEUnknown {
		ideName = "Terminal"
	}
	right1 := "IDE: " + ideName + " "

	pad1 := m.width - lipgloss.Width(left1) - lipgloss.Width(right1)
	if pad1 < 0 {
		pad1 = 0
	}
	line1 := titleStyle.Width(m.width).Render(left1 + strings.Repeat(" ", pad1) + right1)

	branchBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("2"))
	if s.Dirty {
		branchBadge = branchBadge.Background(lipgloss.Color("3"))
	}
	branchLabel := branchBadge.Render(" ⎇ " + s.Current + " ")
	tagDisplay := s.LastTag
	if tagDisplay == "none" {
		tagDisplay = ""
	}
	if s.Version != "0.0.0" && tagDisplay == "v"+s.Version {
		tagDisplay = ""
	}

	segments := []string{" " + pname, "│", branchLabel}
	modeLabel := config.IntegrationModeDisplay(m.gf.Config.IntegrationMode)
	segments = append(segments, "│", "mode: "+modeLabel)
	if s.Version != "0.0.0" {
		segments = append(segments, "│", "v"+s.Version)
	}
	if tagDisplay != "" {
		segments = append(segments, "│", tagDisplay)
	}
	if s.Dirty {
		segments = append(segments, "│", dirtyBadge.Render("● dirty"))
	}
	left2 := strings.Join(segments, " ")

	pad2 := m.width - lipgloss.Width(left2)
	if pad2 < 0 {
		pad2 = 0
	}
	line2 := subtitleStyle.Width(m.width).Render(left2 + strings.Repeat(" ", pad2))

	return line1 + "\n" + line2
}

func (m model) renderDashboardForWidth(width int) string {
	var lines []string
	dividerWidth := width - 2
	if dividerWidth < 8 {
		dividerWidth = 8
	}
	divider := strings.Repeat("-", dividerWidth)

	for _, dl := range m.dashLines {
		var s lipgloss.Style
		switch dl.style {
		case "error":
			s = errorStyle
		case "warn":
			s = warnStyle
		case "dim":
			s = dimStyle
		case "ok":
			s = okStyle
		case "section":
			s = sectionStyle
		case "feature":
			s = branchFeatureStyle
		case "bugfix":
			s = branchBugfixStyle
		case "release":
			s = branchReleaseStyle
		case "hotfix":
			s = branchHotfixStyle
		default:
			s = lipgloss.NewStyle()
		}
		text := dl.text
		if text == dashboardDividerToken {
			text = divider
		}
		if len(text) > width-2 {
			text = text[:width-2]
		}
		lines = append(lines, " "+s.Render(text))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderDashboard() string {
	return m.renderDashboardForWidth(m.width)
}

func (m model) renderActivityPanel(width, height int) string {
	if width < 24 {
		width = 24
	}
	if height < 5 {
		height = 5
	}
	boxWidth := width - 2
	if boxWidth < 22 {
		boxWidth = 22
	}
	innerHeight := height - 2
	if innerHeight < 3 {
		innerHeight = 3
	}

	var lines []string
	lines = append(lines, boldStyle.Render("Agent Activity"))
	lines = append(lines, "")

	entries := m.mcpActivity
	maxEntries := innerHeight - 2
	if maxEntries < 1 {
		maxEntries = 1
	}
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}

	if len(entries) == 0 {
		lines = append(lines, dimStyle.Render("No activity yet."))
	}

	today := time.Now().UTC().Format("2006-01-02")
	for _, entry := range entries {
		ts := entry.Timestamp
		// Parse RFC3339 timestamp and format as HH:MM (today) or MM-DD HH:MM (other day).
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			t = t.UTC()
			if t.Format("2006-01-02") == today {
				ts = t.Format("15:04")
			} else {
				ts = t.Format("01-02 15:04")
			}
		} else if len(ts) >= 19 {
			// Fallback: extract from ISO-like string.
			ts = ts[11:16]
		}
		icon := okStyle.Render("✓")
		if entry.Error != "" || entry.Result == "error" {
			icon = errorStyle.Render("✗")
		}
		source := entry.Source
		if source == "" {
			source = "mcp"
		}

		detail := entry.Tool
		if entry.Args != "" {
			detail += " " + entry.Args
		}
		detail = strings.TrimSpace(detail)
		if detail == "" {
			detail = "(no details)"
		}

		plainPrefix := ts + " [" + source + "] "
		lineWidth := boxWidth - 4
		if lineWidth < 8 {
			lineWidth = 8
		}
		detailWidth := lineWidth - lipgloss.Width(plainPrefix)
		if detailWidth < 4 {
			detailWidth = 4
		}
		detail = truncateRunes(detail, detailWidth)
		plainLine := plainPrefix + detail

		parts := strings.SplitN(plainLine, " ", 3)
		rendered := plainLine
		if len(parts) >= 3 {
			rendered = lipgloss.JoinHorizontal(
				lipgloss.Top,
				icon,
				" ",
				dimStyle.Render(parts[0]),
				" ",
				parts[1],
				" ",
				parts[2],
			)
		} else {
			rendered = lipgloss.JoinHorizontal(lipgloss.Top, icon, " ", plainLine)
		}
		lines = append(lines, rendered)
	}

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(boxWidth).
		Height(innerHeight)

	return panel.Render(strings.Join(lines, "\n"))
}

func (m model) renderActionsForWidth(width int) string {
	var lines []string
	lines = append(lines, "")

	lastRec := lastRecommendedActionIndex(m.actions)

	if lastRec >= 0 {
		lines = append(lines, " "+sectionStyle.Render("Recommended:"))
	} else {
		lines = append(lines, " "+sectionStyle.Render("Actions:"))
	}

	headerInserted := false
	for i, a := range m.actions {
		if !headerInserted && lastRec >= 0 && !a.Recommended && i > lastRec {
			lines = append(lines, "")
			lines = append(lines, " "+sectionStyle.Render("Actions:"))
			headerInserted = true
		}

		label := a.Label
		if len(label) > width-8 {
			label = label[:width-8]
		}
		if i == m.selected {
			line := selectedStyle.Width(width - 2).Render(" ▸ " + label)
			lines = append(lines, " "+line)
		} else if a.Recommended {
			lines = append(lines, "   "+recommendedStyle.Render("▹ "+label))
		} else {
			lines = append(lines, "   "+dimStyle.Render("▹ ")+label)
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) renderActions() string {
	return m.renderActionsForWidth(m.width)
}

func (m model) renderStatusBar() string {
	var hint string
	switch {
	case m.running:
		hint = " " + m.spinner.View() + " Running: " + m.runningTitle + "  [q] quit"
	case m.mode == viewOutput:
		hint = " " + m.outputTitle + "  [j/k] scroll  [q/Esc/Enter] close"
	default:
		activityHint := "[a] activity"
		switch m.activityPanel {
		case activityNormal:
			if m.activityNormalCloses {
				activityHint = "[a] close"
			} else {
				activityHint = "[a] expand"
			}
		case activityExpanded:
			activityHint = "[a] smaller"
		}
		hint = " [j/k] move  [Enter] run  [?] help  [r] refresh  " + activityHint + "  [Ctrl+M/m] mode  [q] quit"
	}
	if len(hint) > m.width {
		hint = hint[:m.width]
	}
	padding := m.width - len(hint)
	if padding < 0 {
		padding = 0
	}
	return statusBarStyle.Width(m.width).Render(hint + strings.Repeat(" ", padding))
}

func classifyOutputLine(line string) (string, string) {
	lower := strings.ToLower(line)
	trimmed := strings.TrimSpace(line)
	switch {
	case trimmed == "":
		return "", "blank"
	case strings.HasPrefix(trimmed, "✓"):
		return "", "ok"
	case strings.HasPrefix(trimmed, "✗"):
		return "", "error"
	case strings.HasPrefix(trimmed, "⚠"):
		return "", "warn"
	case strings.HasPrefix(trimmed, "↪") || strings.HasPrefix(trimmed, "›"):
		return "", "dim"
	case strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "fatal"):
		return "✗", "error"
	case strings.Contains(lower, "conflict"):
		return "⚠", "warn"
	case strings.Contains(lower, "warning") || strings.Contains(lower, "warn"):
		return "⚠", "warn"
	case strings.Contains(lower, "created") || strings.Contains(lower, "merged") ||
		strings.Contains(lower, "success") || strings.Contains(lower, "deleted branch") ||
		strings.Contains(lower, "tagged") || strings.HasPrefix(trimmed, "✓"):
		return "✓", "ok"
	case strings.HasPrefix(trimmed, "Switched to"):
		return "↪", "dim"
	case strings.HasPrefix(trimmed, "→") || strings.HasPrefix(trimmed, "->"):
		return "›", "dim"
	case strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "}") || strings.HasPrefix(trimmed, "\""):
		return "", "json"
	default:
		return " ", "normal"
	}
}

func (m model) renderOutputOverlay(base string) string {
	factor := m.outputAnim
	if factor < 0.01 {
		return base
	}
	if factor > 1 {
		factor = 1
	}

	targetW := m.width - 4
	if targetW < 40 {
		targetW = m.width - 2
	}
	targetH := m.height - 4
	if targetH < 8 {
		targetH = m.height - 2
	}

	minW := 28
	if minW > targetW {
		minW = targetW
	}
	minH := 6
	if minH > targetH {
		minH = targetH
	}

	boxW := int(math.Round(float64(minW) + float64(targetW-minW)*factor))
	boxH := int(math.Round(float64(minH) + float64(targetH-minH)*factor))
	if boxW < minW {
		boxW = minW
	}
	if boxH < minH {
		boxH = minH
	}

	visibleLines := boxH - 5
	// Reserve 2 lines for scroll indicator (blank + position text)
	// so the rendered box never exceeds targetH.
	visibleLines -= 2
	if visibleLines < 1 {
		visibleLines = 1
	}

	hasError := false
	for _, l := range m.outputLines {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "fatal") {
			hasError = true
			break
		}
	}

	var processed []string
	inJSON := false
	for _, l := range m.outputLines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "{" {
			inJSON = true
			continue
		}
		if trimmed == "}" {
			inJSON = false
			continue
		}
		if inJSON {
			continue
		}
		if trimmed == "" {
			continue
		}
		icon, cat := classifyOutputLine(l)
		maxW := boxW - 10
		if maxW < 20 {
			maxW = 20
		}
		text := strings.TrimSpace(l)
		if len(text) > maxW {
			text = text[:maxW]
		}
		content := text
		if icon != "" {
			content = icon + " " + text
		}

		var styled string
		switch cat {
		case "error":
			styled = errorStyle.Render(content)
		case "warn":
			styled = warnStyle.Render(content)
		case "ok":
			styled = okStyle.Render(content)
		case "dim":
			styled = dimStyle.Render(content)
		default:
			styled = content
		}
		processed = append(processed, "  "+styled)
	}

	if len(processed) == 0 {
		processed = append(processed, "  "+dimStyle.Render("No output."))
	}

	end := m.outputScroll + visibleLines
	if end > len(processed) {
		end = len(processed)
	}
	start := m.outputScroll
	if start > len(processed) {
		start = len(processed)
	}

	var contentLines []string

	titleIcon := okStyle.Render("✓")
	if hasError {
		titleIcon = errorStyle.Render("✗")
	}
	contentLines = append(contentLines, fmt.Sprintf(" %s %s", titleIcon, boldStyle.Render(m.outputTitle)))
	contentLines = append(contentLines, dimStyle.Render(" "+strings.Repeat("─", boxW-6)))
	contentLines = append(contentLines, "")

	contentLines = append(contentLines, processed[start:end]...)

	if len(processed) > visibleLines {
		pos := fmt.Sprintf(" ↕ %d-%d / %d", start+1, end, len(processed))
		contentLines = append(contentLines, "")
		contentLines = append(contentLines, dimStyle.Render(pos))
	}

	borderColor := "2"
	if hasError {
		borderColor = "1"
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1).
		Width(boxW)

	box := boxStyle.Render(strings.Join(contentLines, "\n"))
	return placeOverlay(base, box, m.width, m.height)
}

func (m model) renderHelpOverlay(base string) string {
	help := []string{
		"",
		"  Navigation",
		"  ────────────────",
		"  j / ↓        Move down",
		"  k / ↑        Move up",
		"  g / Home     First item",
		"  G / End      Last item",
		"  Enter        Execute selected action",
		"",
		"  Commands",
		"  ────────────────",
		"  r            Refresh dashboard",
		"  a            Toggle activity panel",
		"  Ctrl+M / m   Toggle integration mode",
		"  ?            Toggle this help",
		"  q / Ctrl+C   Quit",
		"",
		"  Output Panel",
		"  ────────────────",
		"  j/k          Scroll output",
		"  q/Esc/Enter  Close panel",
		"",
	}
	boxWidth := 48
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(0, 1).
		Width(boxWidth)

	content := strings.Join(help, "\n")
	box := boxStyle.Render(content)

	return placeOverlay(base, box, m.width, m.height)
}

func (m model) renderInputOverlay(base string) string {
	lines := []string{
		"",
		" " + boldStyle.Render(m.inputPrompt),
		"",
		" " + m.inputField.View(),
		"",
		dimStyle.Render(" Enter: confirm  Esc: cancel"),
		"",
	}
	boxWidth := 55
	if len(m.inputPrompt)+8 > boxWidth {
		boxWidth = len(m.inputPrompt) + 8
	}
	if boxWidth > m.width-4 {
		boxWidth = m.width - 4
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(0, 1).
		Width(boxWidth)

	box := boxStyle.Render(strings.Join(lines, "\n"))
	return placeOverlay(base, box, m.width, m.height)
}

func placeOverlay(base, overlay string, w, h int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for len(baseLines) < h {
		baseLines = append(baseLines, "")
	}
	for i, line := range baseLines {
		lw := lipgloss.Width(line)
		if lw < w {
			baseLines[i] = line + strings.Repeat(" ", w-lw)
		}
	}

	boxH := len(overlayLines)
	boxW := 0
	for _, l := range overlayLines {
		if lw := lipgloss.Width(l); lw > boxW {
			boxW = lw
		}
	}

	startRow := (h - boxH) / 2
	startCol := (w - boxW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	for i, overlayLine := range overlayLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		olw := lipgloss.Width(overlayLine)
		pad := strings.Repeat(" ", startCol)
		rightPad := w - startCol - olw
		if rightPad < 0 {
			rightPad = 0
		}
		baseLines[row] = pad + overlayLine + strings.Repeat(" ", rightPad)
	}

	return strings.Join(baseLines[:h], "\n")
}

// stripANSI removes ANSI escape codes from a string for clean display.
func stripANSI(s string) string {
	var result strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
