package tui

import (
	"os"
	"os/exec"
	"path/filepath"
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
		shell := "sh"
		shellArgs := []string{"-c", cmdStr}
		if runtime.GOOS == "windows" {
			shell = "powershell"
			shellArgs = []string{"-NoProfile", "-Command", cmdStr}
		}
		cmd := exec.Command(shell, shellArgs...)
		cmd.Dir = projectRoot
		return cmd
	}
	// Try to resolve the executable to an absolute path when possible so
	// that the child process does not rely solely on the parent's PATH.
	args[0] = resolveExecutable(args[0], projectRoot)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = projectRoot
	return cmd
}

// resolveExecutable attempts to find an absolute path for the given
// executable name. It first uses exec.LookPath and then falls back to
// common locations such as the project root, $HOME/bin, and the
// directory containing the running binary. If no file is found the
// original name is returned unchanged.
func resolveExecutable(name, projectRoot string) string {
	// If the name contains a path separator, treat it as a path and try
	// to resolve an executable at that location. On Windows also try the
	// .exe variant if the plain name is not found.
	if strings.ContainsAny(name, string(os.PathSeparator)) || strings.Contains(name, "/") {
		if fi, err := os.Stat(name); err == nil && !fi.IsDir() {
			return name
		}
		if runtime.GOOS == "windows" {
			if fi, err := os.Stat(name + ".exe"); err == nil && !fi.IsDir() {
				return name + ".exe"
			}
			// Try converting slashes to the platform form as a last resort.
			alt := filepath.FromSlash(name)
			if fi, err := os.Stat(alt); err == nil && !fi.IsDir() {
				return alt
			}
			if fi, err := os.Stat(alt + ".exe"); err == nil && !fi.IsDir() {
				return alt + ".exe"
			}
		}
		return name
	}

	// Prefer the PATH-resolved executable when available.
	if p, err := exec.LookPath(name); err == nil {
		return p
	}

	var candidates []string
	// Prefer .exe candidates first on Windows when scanning locations.
	if projectRoot != "" {
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(projectRoot, name+".exe"))
			candidates = append(candidates, filepath.Join(projectRoot, name))
		} else {
			candidates = append(candidates, filepath.Join(projectRoot, name))
		}
	}

	home := os.Getenv("HOME")
	if home != "" {
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(home, "bin", name+".exe"))
			candidates = append(candidates, filepath.Join(home, "bin", name))
		} else {
			candidates = append(candidates, filepath.Join(home, "bin", name))
		}
	}
	userprofile := os.Getenv("USERPROFILE")
	if userprofile != "" {
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(userprofile, "bin", name+".exe"))
			candidates = append(candidates, filepath.Join(userprofile, "bin", name))
		} else {
			candidates = append(candidates, filepath.Join(userprofile, "bin", name))
		}
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(dir, name+".exe"))
			candidates = append(candidates, filepath.Join(dir, name))
		} else {
			candidates = append(candidates, filepath.Join(dir, name))
		}
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}

	return name
}
