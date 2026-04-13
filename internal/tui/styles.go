package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("6")).
			PaddingLeft(1).
			PaddingRight(1)

	branchFeatureStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	branchBugfixStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	branchReleaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	branchHotfixStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("6"))

	recommendedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	dimStyle     = lipgloss.NewStyle().Faint(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	sectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	boldStyle    = lipgloss.NewStyle().Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("7"))

	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("5"))

	dirtyBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
)

func branchStyle(btype string) lipgloss.Style {
	switch btype {
	case "feature":
		return branchFeatureStyle
	case "bugfix":
		return branchBugfixStyle
	case "release":
		return branchReleaseStyle
	case "hotfix":
		return branchHotfixStyle
	default:
		return dimStyle
	}
}
