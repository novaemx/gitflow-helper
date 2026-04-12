package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/output"
)

var ProjectRoot string

func Run(cmdStr string) error {
	output.Infof("  %s→ %s%s", output.Dim, cmdStr, output.Reset)
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = ProjectRoot
	cmd.Stdout = output.Writer()
	cmd.Stderr = output.Writer()
	return cmd.Run()
}

func RunResult(cmdStr string) (int, string, string) {
	cmd := exec.Command("sh", "-c", cmdStr)
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

func CurrentBranch() string {
	return RunQuiet("git branch --show-current")
}

func HasUncommittedChanges() bool {
	return len(RunLines("git status --porcelain")) > 0
}

func LatestTag() string {
	tag := RunQuiet("git describe --tags --abbrev=0 2>/dev/null")
	if tag == "" {
		return "none"
	}
	return tag
}

func IsGitRepo() bool {
	_, err := os.Stat(filepath.Join(ProjectRoot, ".git"))
	return err == nil
}

// IsGitFlowInitialized checks whether the repo has both main and develop
// branches — the defining characteristic of a gitflow-structured repository.
// We no longer rely on `git flow` extensions being installed.
func IsGitFlowInitialized() bool {
	all := RunLines("git branch --format='%(refname:short)'")
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
	all := RunLines("git branch --format='%(refname:short)'")
	var releases []string
	for _, b := range all {
		if strings.HasPrefix(b, "release/") {
			releases = append(releases, b)
		}
	}
	return releases
}

func BranchTypeOf(name string) string {
	prefixes := []string{"feature/", "bugfix/", "release/", "hotfix/"}
	types := []string{"feature", "bugfix", "release", "hotfix"}
	for i, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return types[i]
		}
	}
	if name == "develop" || name == "main" || name == "master" {
		return "base"
	}
	return "other"
}

func RemovePrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}

func FlowVersion(version string) string {
	return strings.TrimLeft(version, "v")
}

func BranchExists(name string) bool {
	code, _, _ := RunResult("git rev-parse --verify " + name + " 2>/dev/null")
	return code == 0
}

func AllLocalBranches() []string {
	return RunLines("git branch --format='%(refname:short)'")
}
