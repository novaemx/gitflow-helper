package flow

import (
	"os"
	osexec "os/exec"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
)

// ── helpers ────────────────────────────────────────────────────────────────

func setupIntegrationRepo(t *testing.T) (string, config.FlowConfig) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := osexec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "initial commit")
	run("git", "branch", "develop")

	cfg := config.FlowConfig{
		ProjectRoot:   dir,
		MainBranch:    "main",
		DevelopBranch: "develop",
		Remote:        "",
		TagPrefix:     "v",
	}
	return dir, cfg
}

func setFlowProjectRoot(t *testing.T, dir string) {
	t.Helper()
	old := git.ProjectRoot
	git.ProjectRoot = dir
	t.Cleanup(func() { git.ProjectRoot = old })
}

// ── Backmerge ──────────────────────────────────────────────────────────────

func TestBackmerge_UpToDate(t *testing.T) {
	dir, cfg := setupIntegrationRepo(t)
	setFlowProjectRoot(t, dir)

	code, result := Backmerge(cfg)
	if code != 0 {
		t.Fatalf("expected code 0 (up_to_date), got %d: %v", code, result)
	}
	if result["result"] != "up_to_date" {
		t.Fatalf("expected result=up_to_date, got %v", result["result"])
	}
}

func TestBackmerge_MergesMainIntoDevelop(t *testing.T) {
	dir, cfg := setupIntegrationRepo(t)
	setFlowProjectRoot(t, dir)

	run := func(args ...string) {
		t.Helper()
		cmd := osexec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v\n%s", args, err, out)
		}
	}

	// Add a commit to main that develop doesn't have.
	run("git", "checkout", "main")
	run("git", "commit", "--allow-empty", "-m", "hotfix commit on main")
	run("git", "checkout", "develop")

	code, result := Backmerge(cfg)
	if code != 0 {
		t.Fatalf("expected code 0 after backmerge, got %d: %v", code, result)
	}
	if result["result"] != "ok" {
		t.Fatalf("expected result=ok, got %v", result["result"])
	}
	if result["commits_merged"] != 1 {
		t.Fatalf("expected commits_merged=1, got %v", result["commits_merged"])
	}
}

// ── Sync ───────────────────────────────────────────────────────────────────

func TestSync_NotOnFlowBranch(t *testing.T) {
	dir, cfg := setupIntegrationRepo(t)
	setFlowProjectRoot(t, dir)

	// On 'develop' which is not a flow branch (feature/bugfix/release/hotfix)
	code, result := Sync(cfg)
	if code != 1 {
		t.Fatalf("expected code 1 when not on flow branch, got %d", code)
	}
	if result["error"] != "not on flow branch" {
		t.Fatalf("expected not on flow branch error, got %v", result["error"])
	}
}

func TestSync_FeatureBranchRebase_AlreadyUpToDate(t *testing.T) {
	dir, cfg := setupIntegrationRepo(t)
	setFlowProjectRoot(t, dir)

	run := func(args ...string) {
		t.Helper()
		cmd := osexec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v\n%s", args, err, out)
		}
	}

	// Create a feature branch from develop.
	run("git", "checkout", "develop")
	run("git", "checkout", "-b", "feature/sync-test")

	code, result := Sync(cfg)
	if code != 0 {
		t.Fatalf("expected code 0 for sync (already up to date), got %d: %v", code, result)
	}
	if result["strategy"] != "rebase" {
		t.Fatalf("expected strategy=rebase for feature branch, got %v", result["strategy"])
	}
}

// ── Pull ───────────────────────────────────────────────────────────────────

func TestPull_NoRemote(t *testing.T) {
	dir, cfg := setupIntegrationRepo(t)
	setFlowProjectRoot(t, dir)

	// On develop with no remote configured.
	code, result := Pull(cfg)
	if code != 0 {
		t.Fatalf("expected code 0 when no remote configured, got %d: %v", code, result)
	}
	if result["result"] != "no_remote" {
		t.Fatalf("expected result=no_remote, got %v", result["result"])
	}
}

// ── Cleanup ────────────────────────────────────────────────────────────────

func TestCleanup_NothingToClean(t *testing.T) {
	dir, cfg := setupIntegrationRepo(t)
	setFlowProjectRoot(t, dir)

	code, result := Cleanup(cfg)
	if code != 0 {
		t.Fatalf("expected code 0 when nothing to clean, got %d", code)
	}
	if result["result"] != "nothing_to_clean" {
		t.Fatalf("expected nothing_to_clean, got %v", result["result"])
	}
}

func TestCleanup_DeletesMergedFeatureBranch(t *testing.T) {
	dir, cfg := setupIntegrationRepo(t)
	setFlowProjectRoot(t, dir)

	run := func(args ...string) {
		t.Helper()
		cmd := osexec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v\n%s", args, err, out)
		}
	}

	// Create and merge a feature branch into develop.
	run("git", "checkout", "develop")
	run("git", "checkout", "-b", "feature/merged-feature")
	run("git", "commit", "--allow-empty", "-m", "feature commit")
	run("git", "checkout", "develop")
	run("git", "merge", "--no-ff", "feature/merged-feature", "-m", "Merge feature/merged-feature")

	// Cleanup should delete the merged branch.
	code, result := Cleanup(cfg)
	if code != 0 {
		t.Fatalf("expected code 0 after cleanup, got %d: %v", code, result)
	}
	if result["result"] != "ok" {
		t.Fatalf("expected result=ok, got %v", result["result"])
	}
	deleted, ok := result["deleted"].([]string)
	if !ok || len(deleted) == 0 {
		t.Fatalf("expected at least one deleted branch, got %v", result["deleted"])
	}
	found := false
	for _, b := range deleted {
		if b == "feature/merged-feature" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected feature/merged-feature in deleted list, got %v", deleted)
	}
}
