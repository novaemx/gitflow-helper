package tui

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/git"
)

// needsShell returns true if the command string contains shell metacharacters
// that require a shell interpreter to execute as written (pipes, redirects,
// command substitution, boolean ops).
func needsShell(s string) bool {
	if strings.Contains(s, "||") || strings.Contains(s, "&&") {
		return true
	}
	if strings.ContainsAny(s, "|&;><`$") {
		return true
	}
	if strings.Contains(s, "$(") {
		return true
	}
	return false
}

// BuildExecCmd constructs an *exec.Cmd for the given command string and
// project root. It prefers to run the command directly without a shell by
// splitting arguments safely via git.SplitCommand. If the string appears to
// require shell interpretation, it falls back to a platform-appropriate
// shell invocation (sh -c on Unix, powershell -Command on Windows).
func BuildExecCmd(cmdStr, projectRoot string) *exec.Cmd {
	args := git.SplitCommand(cmdStr)
	if len(args) == 0 || needsShell(cmdStr) {
		if runtime.GOOS == "windows" {
			cmd := exec.Command("powershell", "-NoProfile", "-Command", cmdStr)
			cmd.Dir = projectRoot
			return cmd
		}
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Dir = projectRoot
		return cmd
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = projectRoot
	return cmd
}
