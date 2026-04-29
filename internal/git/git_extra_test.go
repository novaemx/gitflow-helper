package git

import (
	"os"
	"os/exec"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	dir := setupGitRepo(t)
	if HasUncommittedChanges() {
		t.Fatal("expected clean tree")
	}
	if err := os.WriteFile(dir+"/temp.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if !HasUncommittedChanges() {
		t.Fatal("expected uncommitted changes")
	}
}

func TestWorkingTreeStatus(t *testing.T) {
	dir := setupGitRepo(t)
	if err := os.WriteFile(dir+"/tracked.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("write tracked: %v", err)
	}
	runGit(t, dir, "add", "tracked.txt")
	if err := os.WriteFile(dir+"/tracked.txt", []byte("xy"), 0644); err != nil {
		t.Fatalf("modify tracked: %v", err)
	}
	if err := os.WriteFile(dir+"/untracked.txt", []byte("u"), 0644); err != nil {
		t.Fatalf("write untracked: %v", err)
	}

	s := WorkingTreeStatus()
	if s.Staged == 0 {
		t.Fatal("expected staged changes")
	}
	if s.Unstaged == 0 {
		t.Fatal("expected unstaged changes")
	}
	if s.Untracked == 0 {
		t.Fatal("expected untracked changes")
	}
	if s.Total == 0 {
		t.Fatal("expected total > 0")
	}
}

func TestHasStagedChanges(t *testing.T) {
	dir := setupGitRepo(t)
	if HasStagedChanges() {
		t.Fatal("expected no staged changes")
	}
	if err := os.WriteFile(dir+"/staged.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, dir, "add", "staged.txt")
	if !HasStagedChanges() {
		t.Fatal("expected staged changes")
	}
}

func TestTagExistsAndLatestTag(t *testing.T) {
	dir := setupGitRepo(t)
	if TagExists("v1.0.0") {
		t.Fatal("expected tag not to exist")
	}
	if LatestTag() != "none" {
		t.Fatalf("expected no tags yet, got %q", LatestTag())
	}
	runGit(t, dir, "tag", "v1.0.0")
	if !TagExists("v1.0.0") {
		t.Fatal("expected tag v1.0.0 to exist")
	}
	if LatestTag() != "v1.0.0" {
		t.Fatalf("expected latest tag v1.0.0, got %q", LatestTag())
	}
}

func TestActiveReleaseBranches(t *testing.T) {
	dir := setupGitRepo(t)
	runGit(t, dir, "branch", "release/1.0.0")
	runGit(t, dir, "branch", "release/1.1.0")

	releases := ActiveReleaseBranches()
	if len(releases) < 2 {
		t.Fatalf("expected at least 2 release branches, got %v", releases)
	}
}

func TestBranchTypeAndHelpers(t *testing.T) {
	if BranchTypeOf("feature/x") != "feature" {
		t.Fatal("expected feature branch type")
	}
	if RemovePrefix("feature/x", "feature/") != "x" {
		t.Fatal("expected prefix removal")
	}
	if FlowVersion("v1.2.3") != "1.2.3" {
		t.Fatal("expected flow version without leading v")
	}
}

func TestRemotesAndRemoteExists(t *testing.T) {
	dir := setupGitRepo(t)
	runGit(t, dir, "remote", "add", "origin", "https://example.com/repo.git")

	r := Remotes()
	if len(r) == 0 {
		t.Fatal("expected at least one remote")
	}
	if !RemoteExists("origin") {
		t.Fatal("expected origin remote to exist")
	}
	if RemoteExists("upstream") {
		t.Fatal("did not expect upstream remote")
	}
}

func TestStashSaveAndPop(t *testing.T) {
	dir := setupGitRepo(t)
	if err := os.WriteFile(dir+"/stash.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := StashSave("test stash"); err != nil {
		t.Fatalf("StashSave failed: %v", err)
	}
	if !BranchExists("main") {
		t.Fatal("sanity check failed: main should still exist")
	}
	_ = StashPop()
}
