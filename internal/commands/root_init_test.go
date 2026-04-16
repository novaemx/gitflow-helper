package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/ide"
)

func TestAutoGitInit_WhenNotRepo(t *testing.T) {
	// Create an empty directory and run the root PersistentPreRun; after
	// it returns the directory should contain a .git folder (interactive path).
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	defer os.Chdir(oldwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	root := NewRootCmd("test")
	// Call the PersistentPreRun directly to exercise initialization.
	// Use an empty args list which triggers the interactive path.
	root.PersistentPreRun(root, []string{})

	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		t.Fatalf("expected .git to exist after auto-init")
	}
}

// TestNonFreshInit_NeverPromptsAIConsent verifies that on an already-initialized
// repo the PersistentPreRun does NOT call EnsureRulesWithAIConsent in
// interactive mode. The canary is the askAIIntegrationFunc variable: if it is
// invoked the test fails.
func TestNonFreshInit_NeverPromptsAIConsent(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "init", "-b", "main", dir)
	run("git", "-C", dir, "commit", "--allow-empty", "-m", "init")
	run("git", "-C", dir, "branch", "develop")

	prompted := false
	prev := ide.AskAIIntegrationFunc
	ide.AskAIIntegrationFunc = func(detected ide.DetectedIDE) (bool, error) {
		prompted = true
		return false, nil
	}
	defer func() { ide.AskAIIntegrationFunc = prev }()

	oldwd, _ := os.Getwd()
	defer os.Chdir(oldwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	backmergeCmd := NewRootCmd("test")
	// Simulate running `gitflow backmerge` — PersistentPreRun is called on a
	// known subcommand so the non-freshInit path is exercised.
	backmergeCmd.PersistentPreRun(backmergeCmd, []string{})

	if prompted {
		t.Error("AI consent prompt must NOT be shown during non-freshInit command execution")
	}
}
