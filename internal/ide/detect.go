package ide

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// IDE type constants
const (
	IDECursor    = "cursor"
	IDEVSCode    = "vscode"
	IDECopilot   = "copilot" // vscode + copilot
	IDEClaudeCode = "claude-code"
	IDEWindsurf  = "windsurf"
	IDECline     = "cline"
	IDEZed       = "zed"
	IDENeovim    = "neovim"
	IDEJetBrains = "jetbrains"
	IDEUnknown   = "unknown"
	IDEBoth      = "both" // legacy: cursor + copilot
)

// DetectedIDE holds the result of IDE detection with display-friendly name.
type DetectedIDE struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

var ideRegistry = []struct {
	id      string
	display string
	detect  func(string) bool
}{
	{IDECursor, "Cursor", detectCursor},
	{IDEClaudeCode, "Claude Code", detectClaudeCode},
	{IDEWindsurf, "Windsurf", detectWindsurf},
	{IDECline, "Cline", detectCline},
	{IDECopilot, "VS Code + Copilot", detectCopilot},
	{IDEVSCode, "VS Code", detectVSCode},
	{IDEZed, "Zed", detectZed},
	{IDENeovim, "Neovim", detectNeovim},
	{IDEJetBrains, "JetBrains", detectJetBrains},
}

// DetectAll returns all detected IDEs (there may be multiple signals).
func DetectAll(projectRoot string) []DetectedIDE {
	var found []DetectedIDE
	for _, entry := range ideRegistry {
		if entry.detect(projectRoot) {
			found = append(found, DetectedIDE{ID: entry.id, DisplayName: entry.display})
		}
	}
	return found
}

// DetectPrimary returns the most specific IDE detected, or "unknown".
func DetectPrimary(projectRoot string) DetectedIDE {
	all := DetectAll(projectRoot)
	if len(all) > 0 {
		return all[0]
	}
	return DetectedIDE{ID: IDEUnknown, DisplayName: "Terminal"}
}

// Detect returns legacy string for backward compatibility with Generate().
func Detect(projectRoot string) string {
	primary := DetectPrimary(projectRoot)
	switch primary.ID {
	case IDECursor:
		return IDECursor
	case IDECopilot, IDEVSCode:
		return IDECopilot
	default:
		return IDEBoth
	}
}

// --- Individual IDE detectors ---

func detectCursor(projectRoot string) bool {
	envVars := []string{"CURSOR_TRACE_ID", "CURSOR_SESSION", "CURSOR_CHANNEL"}
	for _, v := range envVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".cursor")); err == nil {
		if matchParentProcess("cursor") {
			return true
		}
		return true
	}
	return matchParentProcess("cursor")
}

func detectVSCode(projectRoot string) bool {
	envVars := []string{"VSCODE_GIT_ASKPASS_NODE", "VSCODE_GIT_ASKPASS_MAIN", "VSCODE_IPC_HOOK", "VSCODE_CWD"}
	for _, v := range envVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return matchParentProcess("code")
}

func detectCopilot(projectRoot string) bool {
	if !detectVSCode(projectRoot) {
		return false
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GITHUB_COPILOT_") {
			return true
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".github", "copilot-instructions.md")); err == nil {
		return true
	}
	return false
}

func detectClaudeCode(projectRoot string) bool {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDE_") || strings.HasPrefix(e, "ANTHROPIC_") {
			return true
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "CLAUDE.md")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".claude")); err == nil {
		return true
	}
	return matchParentProcess("claude")
}

func detectWindsurf(projectRoot string) bool {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "WINDSURF_") || strings.HasPrefix(e, "CODEIUM_") {
			return true
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".windsurf")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".windsurfrules")); err == nil {
		return true
	}
	return matchParentProcess("windsurf")
}

func detectCline(projectRoot string) bool {
	if _, err := os.Stat(filepath.Join(projectRoot, ".clinerules")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".cline")); err == nil {
		return true
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLINE_") {
			return true
		}
	}
	return false
}

func detectZed(projectRoot string) bool {
	if os.Getenv("ZED_TERM") != "" {
		return true
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".zed")); err == nil {
		return true
	}
	return matchParentProcess("zed")
}

func detectNeovim(projectRoot string) bool {
	if os.Getenv("NVIM") != "" || os.Getenv("NVIM_LISTEN_ADDRESS") != "" {
		return true
	}
	return matchParentProcess("nvim")
}

func detectJetBrains(projectRoot string) bool {
	if _, err := os.Stat(filepath.Join(projectRoot, ".idea")); err == nil {
		return true
	}
	jetbrainsProcesses := []string{"idea", "pycharm", "webstorm", "goland", "clion", "rider", "phpstorm", "rubymine", "datagrip"}
	for _, p := range jetbrainsProcesses {
		if matchParentProcess(p) {
			return true
		}
	}
	return false
}

// matchParentProcess checks if a process name appears in the parent chain.
func matchParentProcess(name string) bool {
	ppid := os.Getppid()
	if ppid <= 1 {
		return false
	}

	switch runtime.GOOS {
	case "linux":
		cmdline, err := os.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", ppid), "cmdline"))
		if err == nil {
			return strings.Contains(strings.ToLower(string(cmdline)), name)
		}
	case "darwin":
		out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", ppid), "-o", "comm=").Output()
		if err == nil {
			return strings.Contains(strings.ToLower(string(out)), name)
		}
	}
	return false
}

// Generate dispatches to the appropriate rule/instruction file generators.
func Generate(projectRoot, ideType string) ([]string, error) {
	var files []string

	switch ideType {
	case IDECursor:
		f, err := generateCursorRule(projectRoot)
		if err != nil {
			return nil, err
		}
		files = append(files, f)

	case IDECopilot:
		f, err := generateCopilotInstructions(projectRoot)
		if err != nil {
			return nil, err
		}
		files = append(files, f)

	case IDEBoth, IDEUnknown:
		f1, err := generateCursorRule(projectRoot)
		if err != nil {
			return nil, err
		}
		files = append(files, f1)

		f2, err := generateCopilotInstructions(projectRoot)
		if err != nil {
			return nil, err
		}
		files = append(files, f2)

		f3, err := generateAgentsMD(projectRoot)
		if err != nil {
			return nil, err
		}
		files = append(files, f3)
	}

	return files, nil
}
