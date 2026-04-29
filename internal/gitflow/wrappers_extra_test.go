package gitflow

import (
	"os"
	"os/exec"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
)

func setupWrapperRepo(t *testing.T) string {
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
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "initial commit")
	run("git", "branch", "develop")
	return dir
}

func TestLogicWrappers_PullSyncBackmergeCleanup(t *testing.T) {
	dir := setupWrapperRepo(t)
	gf := NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v", VersionFile: ""})

	code, _ := gf.Pull()
	if code != 0 {
		t.Fatalf("expected Pull code 0, got %d", code)
	}

	code, _ = gf.Sync()
	if code == 0 {
		t.Fatalf("expected Sync non-zero on non-flow branch")
	}

	code, _ = gf.Backmerge()
	if code != 0 {
		t.Fatalf("expected Backmerge code 0, got %d", code)
	}

	code, _ = gf.Cleanup()
	if code != 0 {
		t.Fatalf("expected Cleanup code 0, got %d", code)
	}
}

func TestLogicWrappers_HealthDoctorLogUndo(t *testing.T) {
	dir := setupWrapperRepo(t)
	gf := NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "origin", TagPrefix: "v"})

	h := gf.Health()
	if h["action"] != "health" {
		t.Fatalf("expected health action, got %v", h["action"])
	}

	d := gf.Doctor()
	if d["action"] != "doctor" {
		t.Fatalf("expected doctor action, got %v", d["action"])
	}

	l := gf.Log(5)
	if l["action"] != "log" {
		t.Fatalf("expected log action, got %v", l["action"])
	}

	u := gf.Undo()
	if u["action"] != "undo" {
		t.Fatalf("expected undo action, got %v", u["action"])
	}
}

func TestLogicWrappers_IntegrationModeAndResetChecks(t *testing.T) {
	dir := setupWrapperRepo(t)
	gf := NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", IntegrationMode: ""})
	if gf.IntegrationMode() != config.IntegrationModeLocalMerge {
		t.Fatalf("expected default local-merge mode")
	}
	gf.ResetChecks()
	if gf.gitAvailCache != nil || gf.isgitRepoCache != nil || gf.gfInitCache != nil {
		t.Fatalf("expected caches to be reset")
	}
}
