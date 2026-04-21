package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/debug"
	"github.com/novaemx/gitflow-helper/internal/output"
)

// GitClient abstracts git command execution for easier testing and migration
// away from package-level globals.
type GitClient interface {
	Exec(args ...string) error
	ExecResult(args ...string) (int, string, string)
	ExecQuiet(args ...string) string
	ExecLines(args ...string) []string
}

// LocalGitClient runs git commands in a specific working directory.
type LocalGitClient struct {
	Root string
}

// NewLocalGitClient creates a LocalGitClient bound to the provided root.
func NewLocalGitClient(root string) *LocalGitClient {
	return &LocalGitClient{Root: root}
}

func previewGitOutput(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "<empty>"
	}
	if len(trimmed) > 320 {
		return trimmed[:320] + "..."
	}
	return trimmed
}

func logGitInvocation(root string, args []string) {
	cmdLine := fmt.Sprintf("git %s", strings.Join(args, " "))
	if debug.IsDebugEnabled() {
		debug.Printf("cwd=%s cmd=%s", root, cmdLine)
		return
	}
	if debug.IsLogEnabled() {
		debug.Logf("%s", cmdLine)
	}
}

func logGitResult(code int, stdout, stderr string) {
	if !debug.IsDebugEnabled() {
		return
	}
	debug.Printf("git exit=%d stdout=%s stderr=%s", code, previewGitOutput(stdout), previewGitOutput(stderr))
}

func (c *LocalGitClient) Exec(args ...string) error {
	logGitInvocation(c.Root, args)
	output.Infof("  %s→ git %s%s", output.Dim, strings.Join(args, " "), output.Reset)
	cmd := exec.Command("git", args...)
	cmd.Dir = c.Root
	cmd.Stdout = output.Writer()
	cmd.Stderr = output.Writer()
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	logGitResult(code, "", "")
	return err
}

func (c *LocalGitClient) ExecResult(args ...string) (int, string, string) {
	logGitInvocation(c.Root, args)
	cmd := exec.Command("git", args...)
	cmd.Dir = c.Root
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
	trimmedStdout := strings.TrimSpace(stdout.String())
	trimmedStderr := strings.TrimSpace(stderr.String())
	logGitResult(code, trimmedStdout, trimmedStderr)
	return code, trimmedStdout, trimmedStderr
}

func (c *LocalGitClient) ExecQuiet(args ...string) string {
	_, stdout, _ := c.ExecResult(args...)
	return stdout
}

func (c *LocalGitClient) ExecLines(args ...string) []string {
	raw := c.ExecQuiet(args...)
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

var defaultClient GitClient

// dynamicLocalClient delegates to a fresh LocalGitClient built from the
// current ProjectRoot on every call. This ensures callers that capture the
// returned client still respect test-time changes to `git.ProjectRoot`.
type dynamicLocalClient struct{}

func (d *dynamicLocalClient) Exec(args ...string) error {
	return NewLocalGitClient(ProjectRoot).Exec(args...)
}

func (d *dynamicLocalClient) ExecResult(args ...string) (int, string, string) {
	return NewLocalGitClient(ProjectRoot).ExecResult(args...)
}

func (d *dynamicLocalClient) ExecQuiet(args ...string) string {
	return NewLocalGitClient(ProjectRoot).ExecQuiet(args...)
}

func (d *dynamicLocalClient) ExecLines(args ...string) []string {
	return NewLocalGitClient(ProjectRoot).ExecLines(args...)
}

// DefaultClient returns the current default GitClient. When no override is
// set, a dynamic client is returned so the effective working directory
// follows mutations to `git.ProjectRoot` (important for tests).
func DefaultClient() GitClient {
	if defaultClient == nil {
		return &dynamicLocalClient{}
	}
	return defaultClient
}

// SetDefaultClient replaces the package default client (useful for tests).
func SetDefaultClient(c GitClient) {
	defaultClient = c
}
