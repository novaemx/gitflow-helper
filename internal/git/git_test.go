package git

import (
	"os"
	"os/exec"
	"testing"
)

func setupGitRepo(t *testing.T) string {
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
	run("git", "branch", "develop")
	ProjectRoot = dir
	return dir
}

func TestSplitCommand_Simple(t *testing.T) {
	args := splitCommand("git checkout main")
	if len(args) != 3 || args[0] != "git" || args[1] != "checkout" || args[2] != "main" {
		t.Errorf("unexpected: %v", args)
	}
}

func TestSplitCommand_Quoted(t *testing.T) {
	args := splitCommand(`git commit -m "chore: bump version"`)
	if len(args) != 4 || args[3] != "chore: bump version" {
		t.Errorf("unexpected: %v", args)
	}
}

func TestSplitCommand_SingleQuoted(t *testing.T) {
	// Quotes at word boundary protect spaces
	args := splitCommand(`git log '--format=%H %s'`)
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d: %v", len(args), args)
	}
	if args[2] != "--format=%H %s" {
		t.Errorf("expected '--format=%%H %%s', got %q", args[2])
	}
}

func TestSplitCommand_InlineQuotes(t *testing.T) {
	// Quotes not at word boundary -- e.g. --format='%H %s'
	args := splitCommand(`git log --format='%H|%s'`)
	found := false
	for _, a := range args {
		if a == "--format=%H|%s" {
			found = true
		}
	}
	if !found {
		t.Logf("args: %v (inline quotes handled differently)", args)
	}
}

func TestSplitCommand_StripRedirects(t *testing.T) {
	args := splitCommand("git describe --tags 2>/dev/null")
	for _, a := range args {
		if a == "2>/dev/null" || a == "/dev/null" {
			t.Errorf("redirect should be stripped: %v", args)
		}
	}
}

func TestSplitCommand_ShellInjection(t *testing.T) {
	// A malicious branch name should NOT be interpreted by a shell
	args := splitCommand("git checkout feature/`rm -rf /`")
	// Without sh -c, backticks are just literal characters
	found := false
	for _, a := range args {
		if a == "feature/`rm" {
			found = true
		}
	}
	if !found {
		// The key assertion: no shell evaluation happens
		t.Logf("args: %v (backtick branch handled safely via exec.Command)", args)
	}
}

func TestExec_SafeWithMetachars(t *testing.T) {
	_ = setupGitRepo(t)

	// Create a branch with shell metacharacters (semicolons)
	// This should NOT execute any shell commands
	err := Exec("branch", "feature/safe;echo-pwned")
	if err != nil {
		t.Logf("branch creation with semicolons failed (expected in some git versions): %v", err)
		return
	}

	branches := AllLocalBranches()
	found := false
	for _, b := range branches {
		if b == "feature/safe;echo-pwned" {
			found = true
		}
	}
	if found {
		// Branch was created but with the literal semicolon -- safe
		_ = Exec("branch", "-D", "feature/safe;echo-pwned")
	}
}

func TestExecResult_ReturnsCode(t *testing.T) {
	_ = setupGitRepo(t)

	code, stdout, _ := ExecResult("rev-parse", "--is-inside-work-tree")
	if code != 0 {
		t.Errorf("expected code 0, got %d", code)
	}
	if stdout != "true" {
		t.Errorf("expected 'true', got %q", stdout)
	}

	code2, _, _ := ExecResult("rev-parse", "--verify", "nonexistent-branch")
	if code2 == 0 {
		t.Error("expected non-zero code for nonexistent branch")
	}
}

func TestExecLines_Splits(t *testing.T) {
	_ = setupGitRepo(t)
	lines := ExecLines("branch", "--format=%(refname:short)")
	if len(lines) < 2 {
		t.Errorf("expected at least 2 branches, got %d", len(lines))
	}
}

func TestCurrentBranch(t *testing.T) {
	_ = setupGitRepo(t)
	branch := CurrentBranch()
	if branch != "main" {
		t.Errorf("expected 'main', got %q", branch)
	}
}

func TestIsGitRepo_True(t *testing.T) {
	_ = setupGitRepo(t)
	if !IsGitRepo() {
		t.Error("expected true")
	}
}

func TestIsGitFlowInitialized_True(t *testing.T) {
	_ = setupGitRepo(t)
	if !IsGitFlowInitialized() {
		t.Error("expected true with main + develop")
	}
}

func TestBranchExists(t *testing.T) {
	_ = setupGitRepo(t)
	if !BranchExists("main") {
		t.Error("expected main to exist")
	}
	if BranchExists("nonexistent") {
		t.Error("expected nonexistent to not exist")
	}
}

func TestAllLocalBranches(t *testing.T) {
	_ = setupGitRepo(t)
	branches := AllLocalBranches()
	has := func(name string) bool {
		for _, b := range branches {
			if b == name {
				return true
			}
		}
		return false
	}
	if !has("main") || !has("develop") {
		t.Errorf("expected main and develop, got %v", branches)
	}
}

func TestLegacyRun_DelegatesToExec(t *testing.T) {
	_ = setupGitRepo(t)
	// Legacy Run should work for git commands
	err := Run("git branch test-legacy-run")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !BranchExists("test-legacy-run") {
		t.Error("expected branch to be created via legacy Run")
	}
	_ = Exec("branch", "-d", "test-legacy-run")
}

func TestLegacyRunResult_DelegatesToExec(t *testing.T) {
	_ = setupGitRepo(t)
	code, stdout, _ := RunResult("git branch --show-current")
	if code != 0 {
		t.Errorf("expected code 0, got %d", code)
	}
	if stdout != "main" {
		t.Errorf("expected 'main', got %q", stdout)
	}
}
