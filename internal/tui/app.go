package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/ide"
	"github.com/luis-lozano/gitflow-helper/internal/state"
)

type model struct {
	cfg         config.FlowConfig
	state       state.RepoState
	actions     []action
	dashLines   []dashLine
	detectedIDE ide.DetectedIDE
	selected    int
	scroll      int
	width       int
	height      int
	showHelp    bool
	showPalette bool
	paletteQuery string
	quitting    bool
}

type refreshMsg struct{}

func Run(cfg config.FlowConfig) error {
	git.ProjectRoot = cfg.ProjectRoot
	m := model{cfg: cfg}
	m.detectedIDE = ide.DetectPrimary(cfg.ProjectRoot)
	m.refresh()

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *model) refresh() {
	m.state = state.DetectState(m.cfg)
	m.actions = buildActions(m.state, m.cfg)
	m.dashLines = buildDashboardLines(m.state, m.cfg)
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

	case refreshMsg:
		m.refresh()
		return m, nil

	case tea.KeyMsg:
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if m.showPalette {
			return m.handlePaletteKey(msg)
		}
		return m.handleKey(msg)
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
			if a.Command != "" {
				return m, m.executeAction(a)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
		m.showPalette = true
		m.paletteQuery = ""
	case key.Matches(msg, key.NewBinding(key.WithKeys("?"))):
		m.showHelp = true
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		m.refresh()
	}
	return m, nil
}

func (m model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.showPalette = false
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		filtered := m.filteredActions()
		if len(filtered) > 0 && m.selected < len(filtered) {
			a := filtered[m.selected]
			m.showPalette = false
			if a.Tag == "exit" {
				m.quitting = true
				return m, tea.Quit
			}
			if a.Command != "" {
				return m, m.executeAction(a)
			}
		}
		m.showPalette = false
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

func (m model) executeAction(a action) tea.Cmd {
	return tea.ExecProcess(exec.Command("sh", "-c", a.Command+"; echo; echo 'Press Enter to return...'; read _"), func(err error) tea.Msg {
		return refreshMsg{}
	})
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var sections []string
	sections = append(sections, m.renderTitleBar())
	sections = append(sections, "") // spacer

	dashContent := m.renderDashboard()
	actionContent := m.renderActions()

	contentHeight := m.height - 3 // title + spacer + status bar
	content := dashContent + "\n" + actionContent
	lines := strings.Split(content, "\n")

	// Auto-scroll
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

	// Pad to fill
	for len(visible) < contentHeight {
		visible = append(visible, "")
	}

	sections = append(sections, strings.Join(visible, "\n"))
	sections = append(sections, m.renderStatusBar())

	result := strings.Join(sections, "\n")

	if m.showHelp {
		result = m.renderHelpOverlay(result)
	}
	if m.showPalette {
		result = m.renderPaletteOverlay(result)
	}

	return result
}

func (m model) renderTitleBar() string {
	btype := git.BranchTypeOf(m.state.Current)

	pname := ""
	parts := strings.Split(m.cfg.ProjectRoot, string(os.PathSeparator))
	if len(parts) > 0 {
		pname = parts[len(parts)-1]
	}

	branchLabel := branchStyle(btype).Render(" " + m.state.Current + " ")

	tagDisplay := m.state.LastTag
	if tagDisplay == "none" {
		tagDisplay = ""
	}

	segments := []string{
		" " + pname + " ",
		"│",
		branchLabel,
		"│",
		"v" + m.state.Version,
	}
	if tagDisplay != "" {
		segments = append(segments, "│", tagDisplay)
	}
	if m.state.Dirty {
		segments = append(segments, "│", dirtyBadge.Render("● dirty"))
	}

	left := strings.Join(segments, " ")

	var rightParts []string
	if m.detectedIDE.ID != ide.IDEUnknown {
		rightParts = append(rightParts, m.detectedIDE.DisplayName)
	}
	if m.state.GitFlowInitialized {
		rightParts = append(rightParts, "gitflow")
	}
	right := ""
	if len(rightParts) > 0 {
		right = " " + strings.Join(rightParts, " │ ") + " "
	}

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
	hint := " [j/k] move  [Enter] select  [/] search  [?] help  [r] refresh  [q] quit"
	if len(hint) > m.width {
		hint = hint[:m.width]
	}
	padding := m.width - len(hint)
	if padding < 0 {
		padding = 0
	}
	return statusBarStyle.Width(m.width).Render(hint + strings.Repeat(" ", padding))
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
		// Pad base line
		for lipgloss.Width(baseLine) < w {
			baseLine += " "
		}
		runes := []rune(baseLine)
		oRunes := []rune(overlayLine)
		if startCol+len(oRunes) <= len(runes) {
			copy(runes[startCol:], oRunes)
		} else {
			// Just replace what we can
			for j, r := range oRunes {
				if startCol+j < len(runes) {
					runes[startCol+j] = r
				}
			}
		}
		baseLines[row] = string(runes)
	}

	_ = fmt.Sprintf // keep import
	return strings.Join(baseLines[:h], "\n")
}
