package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestDetectCommitHash_FromRealRepo(t *testing.T) {
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
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "init")

	oldWD, _ := os.Getwd()
	defer os.Chdir(oldWD)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	h := detectCommitHash()
	if len(h) != 12 {
		t.Fatalf("expected 12-char hash, got %q", h)
	}
	if normalizeCommitHash(h) != h {
		t.Fatalf("expected normalized hash, got %q", h)
	}
}

func TestDetectCommitHash_NotRepoReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	defer os.Chdir(oldWD)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if got := detectCommitHash(); got != "" {
		t.Fatalf("expected empty hash outside repo, got %q", got)
	}
}

func TestBuildDisplayVersion_InvalidHashIgnored(t *testing.T) {
	got := buildDisplayVersion("1.2.3", "not-a-hash")
	if got != "1.2.3" {
		t.Fatalf("expected plain version when hash invalid, got %q", got)
	}
}
