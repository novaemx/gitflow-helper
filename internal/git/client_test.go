package git

import (
	"os"
	"os/exec"
	"testing"
)

func setupSimpleRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "initial commit")
	return dir
}

func TestLocalGitClient_ExecResult_CurrentBranch(t *testing.T) {
	dir := setupSimpleRepo(t)
	c := NewLocalGitClient(dir)
	code, stdout, _ := c.ExecResult("branch", "--show-current")
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if stdout != "main" && stdout != "master" {
		t.Fatalf("expected main/master, got %q", stdout)
	}
}

func TestDefaultClientCanBeSet(t *testing.T) {
	dir := setupSimpleRepo(t)
	c := NewLocalGitClient(dir)
	old := DefaultClient()
	SetDefaultClient(c)
	defer SetDefaultClient(old)
	code, stdout, _ := DefaultClient().ExecResult("rev-parse", "--is-inside-work-tree")
	if code != 0 || stdout != "true" {
		t.Fatalf("expected inside-work-tree true, got code=%d stdout=%q", code, stdout)
	}
}

func TestLocalGitClient_ExecQuietAndLines(t *testing.T) {
	dir := setupSimpleRepo(t)
	c := NewLocalGitClient(dir)
	out := c.ExecQuiet("rev-parse", "--is-inside-work-tree")
	if out != "true" {
		t.Fatalf("expected true got %q", out)
	}
	lines := c.ExecLines("branch", "--format=%(refname:short)")
	if len(lines) == 0 {
		t.Fatalf("expected branches, got %v", lines)
	}
}
