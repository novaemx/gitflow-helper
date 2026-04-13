package tui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/luis-lozano/gitflow-helper/internal/branch"
	"github.com/luis-lozano/gitflow-helper/internal/gitflow"
	"github.com/luis-lozano/gitflow-helper/internal/ide"
)

type viewMode int

const (
	viewDashboard viewMode = iota
	viewOutput
	viewHelp
	viewPalette
	viewInput
)

type model struct {
	gf           *gitflow.Logic
	actions      []action
	dashLines    []dashLine
	selected     int
	scroll       int
	width        int
	height       int
	mode         viewMode
	quitting     bool

	// Command output overlay
	outputTitle  string
	outputLines  []string
	outputScroll int

	// Command palette
	paletteQuery string

	// Input overlay
	inputPrompt   string
	inputValue    string
	inputDefault  string
	inputCallback func(string)
}

type refreshMsg struct{}

type cmdDoneMsg struct {
	title  string
	output string
	err    error
}

func Run(gf *gitflow.Logic) error {
	m := model{gf: gf, mode: viewDashboard}
	m.refresh()

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *model) refresh() {
	m.gf.Refresh()
	m.actions = buildActions(m.gf.State, m.gf.Config)
	m.dashLines = buildDashboardLines(m.gf.State, m.gf.Config)
	m.selected = 0
	m.scroll = 0
	for i, a := range m.actions {
		if a.Recommended {
			m.selected = i
			break
		}
	}
}

func (m model) Init() tea.Cmd { return nil }

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
		m.mode = viewOutput
		m.refresh()
		return m, nil

	case refreshMsg:
		m.refresh()
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case viewOutput:
			return m.handleOutputKey(msg)
		case viewHelp:
			m.mode = viewDashboard
			return m, nil
		case viewPalette:
			return m.handlePaletteKey(msg)
		case viewInput:
			return m.handleInputKey(msg)
		default:
			return m.handleKey(msg)
		}
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if m.selected < len(m.actions) {
			a := m.actions[m.selected]
			if a.Tag == "exit" {
				m.quitting = true
				return m, tea.Quit
			}
			// Actions that need user input before executing
			if a.NeedsInput {
				m.inputPrompt = a.InputPrompt
				m.inputDefault = a.InputDefault
				m.inputValue = a.InputDefault
				m.inputCallback = func(val string) {
					// will be handled via the action's Command template
				}
				m.mode = viewInput
				return m, nil
			}
			if a.Command != "" {
				return m, m.runCommandAsync(a)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
		m.mode = viewPalette
		m.paletteQuery = ""
	case key.Matches(msg, key.NewBinding(key.WithKeys("?"))):
		m.mode = viewHelp
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		m.refresh()
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
		m.mode = viewDashboard
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

func (m model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = viewDashboard
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		filtered := m.filteredActions()
		if len(filtered) > 0 && m.selected < len(filtered) {
			a := filtered[m.selected]
			m.mode = viewDashboard
			if a.Tag == "exit" {
				m.quitting = true
				return m, tea.Quit
			}
			if a.Command != "" {
				return m, m.runCommandAsync(a)
			}
		}
		m.mode = viewDashboard
	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if len(m.paletteQuery) > 0 {
			m.paletteQuery = m.paletteQuery[:len(m.paletteQuery)-1]
			m.selected = 0
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
		if m.selected > 0 {
			m.selected--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
		filtered := m.filteredActions()
		if m.selected < len(filtered)-1 {
			m.selected++
		}
	default:
		if len(msg.String()) == 1 {
			m.paletteQuery += msg.String()
			m.selected = 0
		}
	}
	return m, nil
}

func (m model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = viewDashboard
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		val := m.inputValue
		if val == "" {
			val = m.inputDefault
		}
		m.mode = viewDashboard
		if val != "" && m.inputCallback != nil {
			m.inputCallback(val)
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if len(m.inputValue) > 0 {
			m.inputValue = m.inputValue[:len(m.inputValue)-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 {
			m.inputValue += ch
		}
	}
	return m, nil
}

func (m model) filteredActions() []action {
	if m.paletteQuery == "" {
		return m.actions
	}
	q := strings.ToLower(m.paletteQuery)
	var filtered []action
	for _, a := range m.actions {
		if strings.Contains(strings.ToLower(a.Label), q) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// runCommandAsync executes a shell command in the background, captures output,
// and sends it back to the TUI as a message — never leaves the AltScreen.
func (m model) runCommandAsync(a action) tea.Cmd {
	cmdStr := a.Command
	label := a.Label
	projectRoot := m.gf.Config.ProjectRoot
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Dir = projectRoot
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
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
		return m.renderOutputOverlay(base)
	case viewHelp:
		return m.renderHelpOverlay(base)
	case viewPalette:
		return m.renderPaletteOverlay(base)
	case viewInput:
		return m.renderInputOverlay(base)
	}

	return base
}

func (m model) renderBase() string {
	var sections []string
	sections = append(sections, m.renderTitleBar())
	sections = append(sections, "")

	dashContent := m.renderDashboard()
	actionContent := m.renderActions()

	contentHeight := m.height - 3
	content := dashContent + "\n" + actionContent
	lines := strings.Split(content, "\n")

	dashLineCount := len(m.dashLines) + 2
	selectedRow := dashLineCount + m.selected + 2
	if selectedRow-m.scroll >= contentHeight {
		m.scroll = selectedRow - contentHeight + 1
	}
	if selectedRow-m.scroll < 0 {
		m.scroll = selectedRow
	}
	if m.scroll < 0 {
		m.scroll = 0
	}

	end := m.scroll + contentHeight
	if end > len(lines) {
		end = len(lines)
	}
	start := m.scroll
	if start > len(lines) {
		start = len(lines)
	}
	visible := lines[start:end]

	for len(visible) < contentHeight {
		visible = append(visible, "")
	}

	sections = append(sections, strings.Join(visible, "\n"))
	sections = append(sections, m.renderStatusBar())

	return strings.Join(sections, "\n")
}

func (m model) renderTitleBar() string {
	s := m.gf.State
	btype := branch.TypeOf(s.Current)

	pname := ""
	parts := strings.Split(m.gf.Config.ProjectRoot, string(os.PathSeparator))
	if len(parts) > 0 {
		pname = parts[len(parts)-1]
	}

	branchLabel := branchStyle(btype).Render(" " + s.Current + " ")

	tagDisplay := s.LastTag
	if tagDisplay == "none" {
		tagDisplay = ""
	}

	segments := []string{
		" " + pname + " ",
		"│",
		branchLabel,
		"│",
		"v" + s.Version,
	}
	if tagDisplay != "" {
		segments = append(segments, "│", tagDisplay)
	}
	if s.Dirty {
		segments = append(segments, "│", dirtyBadge.Render("● dirty"))
	}

	left := strings.Join(segments, " ")

	// Always show IDE — either detected name or "Terminal"
	var rightParts []string
	ideName := m.gf.IDEDisplay()
	if ideName == "" || m.gf.IDE.ID == ide.IDEUnknown {
		ideName = "Terminal"
	}
	rightParts = append(rightParts, ideName)
	if s.GitFlowInitialized {
		rightParts = append(rightParts, "gitflow")
	}
	right := " " + strings.Join(rightParts, " │ ") + " "

	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 0 {
		padding = 0
	}

	bar := left + strings.Repeat(" ", padding) + right
	return titleStyle.Width(m.width).Render(bar)
}

func (m model) renderDashboard() string {
	var lines []string
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
		if len(text) > m.width-2 {
			text = text[:m.width-2]
		}
		lines = append(lines, " "+s.Render(text))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderActions() string {
	var lines []string
	lines = append(lines, "")
	lines = append(lines, " "+sectionStyle.Render("Actions:"))

	for i, a := range m.actions {
		label := a.Label
		if len(label) > m.width-8 {
			label = label[:m.width-8]
		}

		if i == m.selected {
			line := selectedStyle.Width(m.width - 2).Render(" ▸ " + label)
			lines = append(lines, " "+line)
		} else if a.Recommended {
			lines = append(lines, "   "+recommendedStyle.Render(label)+" "+dimStyle.Render("← recommended"))
		} else {
			lines = append(lines, "   "+label)
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) renderStatusBar() string {
	var hint string
	switch m.mode {
	case viewOutput:
		hint = " [j/k] scroll  [q/Esc/Enter] close"
	default:
		hint = " [j/k] move  [Enter] run  [/] search  [?] help  [r] refresh  [q] quit"
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

// renderOutputOverlay draws the command output inside a box overlay.
func (m model) renderOutputOverlay(base string) string {
	boxW := m.width - 4
	if boxW < 40 {
		boxW = m.width - 2
	}
	boxH := m.height - 4
	if boxH < 8 {
		boxH = m.height - 2
	}

	visibleLines := boxH - 4
	if visibleLines < 1 {
		visibleLines = 1
	}

	end := m.outputScroll + visibleLines
	if end > len(m.outputLines) {
		end = len(m.outputLines)
	}
	start := m.outputScroll
	if start > len(m.outputLines) {
		start = len(m.outputLines)
	}

	var contentLines []string
	contentLines = append(contentLines, boldStyle.Render(" "+m.outputTitle))
	contentLines = append(contentLines, dimStyle.Render(" "+strings.Repeat("─", boxW-6)))

	for _, l := range m.outputLines[start:end] {
		if len(l) > boxW-6 {
			l = l[:boxW-6]
		}
		contentLines = append(contentLines, " "+l)
	}

	// Scroll indicator
	if len(m.outputLines) > visibleLines {
		pos := fmt.Sprintf(" [%d-%d / %d lines]", start+1, end, len(m.outputLines))
		contentLines = append(contentLines, dimStyle.Render(pos))
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
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
		"  /            Search / filter actions",
		"  r            Refresh dashboard",
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

func (m model) renderPaletteOverlay(base string) string {
	filtered := m.filteredActions()
	var lines []string
	query := m.paletteQuery
	if query == "" {
		query = dimStyle.Render("type to filter...")
	}
	lines = append(lines, " > "+query)
	lines = append(lines, "")
	for i, a := range filtered {
		marker := "  "
		if i == m.selected {
			marker = "▸ "
		}
		rec := ""
		if a.Recommended {
			rec = " ←"
		}
		lines = append(lines, " "+marker+a.Label+rec)
	}

	boxWidth := m.width / 2
	if boxWidth < 55 {
		boxWidth = 55
	}
	if boxWidth > m.width-4 {
		boxWidth = m.width - 4
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("5")).
		Padding(0, 1).
		Width(boxWidth)

	content := strings.Join(lines, "\n")
	box := boxStyle.Render(content)

	return placeOverlay(base, box, m.width, m.height)
}

func (m model) renderInputOverlay(base string) string {
	displayVal := m.inputValue
	if displayVal == "" {
		displayVal = dimStyle.Render(m.inputDefault)
	}
	lines := []string{
		"",
		" " + m.inputPrompt,
		"",
		" > " + displayVal,
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

	boxH := len(overlayLines)
	boxW := 0
	for _, l := range overlayLines {
		if lipgloss.Width(l) > boxW {
			boxW = lipgloss.Width(l)
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
		baseLine := baseLines[row]
		for lipgloss.Width(baseLine) < w {
			baseLine += " "
		}
		runes := []rune(baseLine)
		oRunes := []rune(overlayLine)
		if startCol+len(oRunes) <= len(runes) {
			copy(runes[startCol:], oRunes)
		} else {
			for j, r := range oRunes {
				if startCol+j < len(runes) {
					runes[startCol+j] = r
				}
			}
		}
		baseLines[row] = string(runes)
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
