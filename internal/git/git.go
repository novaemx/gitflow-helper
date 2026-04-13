package git

import (
	"os/exec"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/branch"
	"github.com/luis-lozano/gitflow-helper/internal/output"
)

// ProjectRoot is the working directory for all git commands.
// Deprecated: will be removed once all callers migrate to GitClient.
var ProjectRoot string

// Exec runs git with explicit arguments (no shell interpretation).
// This is the safe replacement for Run() that prevents shell injection.
func Exec(args ...string) error {
	output.Infof("  %s→ git %s%s", output.Dim, strings.Join(args, " "), output.Reset)
	cmd := exec.Command("git", args...)
	cmd.Dir = ProjectRoot
	cmd.Stdout = output.Writer()
	cmd.Stderr = output.Writer()
	return cmd.Run()
}

// ExecResult runs git with explicit arguments and captures output.
func ExecResult(args ...string) (int, string, string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = ProjectRoot
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	return code, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())
}

// ExecQuiet runs git with explicit arguments and returns only stdout.
func ExecQuiet(args ...string) string {
	_, stdout, _ := ExecResult(args...)
	return stdout
}

// ExecLines runs git with explicit arguments and returns stdout split by newline.
func ExecLines(args ...string) []string {
	raw := ExecQuiet(args...)
	if raw == "" {
		return nil
	}
	var result []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// --- Legacy wrappers (delegate to safe Exec functions) ---
// These exist to minimize churn during migration. New code should call Exec* directly.

func Run(cmdStr string) error {
	args := splitCommand(cmdStr)
	if len(args) > 0 && args[0] == "git" {
		return Exec(args[1:]...)
	}
	output.Infof("  %s→ %s%s", output.Dim, cmdStr, output.Reset)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = ProjectRoot
	cmd.Stdout = output.Writer()
	cmd.Stderr = output.Writer()
	return cmd.Run()
}

func RunResult(cmdStr string) (int, string, string) {
	args := splitCommand(cmdStr)
	if len(args) > 0 && args[0] == "git" {
		return ExecResult(args[1:]...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = ProjectRoot
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	return code, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())
}

func RunQuiet(cmdStr string) string {
	_, stdout, _ := RunResult(cmdStr)
	return stdout
}

func RunLines(cmdStr string) []string {
	raw := RunQuiet(cmdStr)
	if raw == "" {
		return nil
	}
	var result []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// splitCommand splits a shell command string into arguments, respecting
// single and double quotes. This is used by the legacy wrappers to avoid
// shell interpretation while maintaining backward compatibility.
func splitCommand(s string) []string {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == ' ' && !inSingle && !inDouble:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}

	// Strip shell redirects (2>/dev/null) that callers used with sh -c
	var clean []string
	for _, a := range args {
		if strings.HasPrefix(a, "2>") || a == "/dev/null" {
			continue
		}
		clean = append(clean, a)
	}
	return clean
}

// --- Convenience functions (use safe Exec internally) ---

func CurrentBranch() string {
	return ExecQuiet("branch", "--show-current")
}

func HasUncommittedChanges() bool {
	return len(ExecLines("status", "--porcelain")) > 0
}

type WorkTreeStatus struct {
	Staged    int
	Unstaged  int
	Untracked int
	Total     int
}

func WorkingTreeStatus() WorkTreeStatus {
	lines := ExecLines("status", "--porcelain")
	var s WorkTreeStatus
	for _, l := range lines {
		if len(l) < 2 {
			continue
		}
		x, y := l[0], l[1]
		if x == '?' {
			s.Untracked++
		} else {
			if x != ' ' {
				s.Staged++
			}
			if y != ' ' {
				s.Unstaged++
			}
		}
	}
	s.Total = len(lines)
	return s
}

func StashSave(msg string) error {
	return Exec("stash", "push", "-m", msg)
}

func StashPop() error {
	return Exec("stash", "pop")
}

func HasStagedChanges() bool {
	code, _, _ := ExecResult("diff", "--cached", "--quiet")
	return code == 1
}

func TagExists(tag string) bool {
	code, _, _ := ExecResult("rev-parse", "-q", "--verify", "refs/tags/"+tag)
	return code == 0
}

func LatestTag() string {
	tag := ExecQuiet("describe", "--tags", "--abbrev=0")
	if tag == "" {
		return "none"
	}
	return tag
}

func IsGitRepo() bool {
	code, _, _ := ExecResult("rev-parse", "--is-inside-work-tree")
	return code == 0
}

func IsGitFlowInitialized() bool {
	all := ExecLines("branch", "--format=%(refname:short)")
	hasMain := false
	hasDevelop := false
	for _, b := range all {
		if b == "main" || b == "master" {
			hasMain = true
		}
		if b == "develop" {
			hasDevelop = true
		}
	}
	return hasMain && hasDevelop
}

func ActiveReleaseBranches() []string {
	all := ExecLines("branch", "--format=%(refname:short)")
	var releases []string
	for _, b := range all {
		if strings.HasPrefix(b, "release/") {
			releases = append(releases, b)
		}
	}
	return releases
}

// BranchTypeOf delegates to branch.TypeOf for backward compatibility.
func BranchTypeOf(name string) string {
	return branch.TypeOf(name)
}

func RemovePrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}

func FlowVersion(version string) string {
	return strings.TrimLeft(version, "v")
}

func BranchExists(name string) bool {
	code, _, _ := ExecResult("rev-parse", "--verify", name)
	return code == 0
}

func AllLocalBranches() []string {
	return ExecLines("branch", "--format=%(refname:short)")
}
