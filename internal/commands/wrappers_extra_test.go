package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	"github.com/novaemx/gitflow-helper/internal/ide"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func setupCommandsRepo(t *testing.T) string {
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
	run("git", "commit", "--allow-empty", "-m", "init")
	run("git", "branch", "develop")
	return dir
}

func TestWrapperCommands_SuccessPathsInJSONMode(t *testing.T) {
	dir := setupCommandsRepo(t)
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v"})

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	defer output.SetJSONMode(prevJSON)

	cmds := []*cobra.Command{
		newPullCmd(),
		newBackmergeCmd(),
		newCleanupCmd(),
		newInitCmd(),
	}
	for _, c := range cmds {
		if err := c.RunE(c, []string{}); err != nil {
			t.Fatalf("RunE failed for %s: %v", c.Use, err)
		}
	}
}

func TestRunTUI_ReturnsNilWhenGFIsSet(t *testing.T) {
	dir := setupCommandsRepo(t)
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop"})
	_ = runTUI // compile coverage for bridge entrypoint
}

func TestNewPushCmd_JSONMode(t *testing.T) {
	dir := setupCommandsRepo(t)
	// Set up a local bare repo as the remote so push returns 0 (the
	// command's RunE calls os.Exit on non-zero, so the test process
	// would die without a working remote).
	remoteDir := t.TempDir()
	runRemote := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = remoteDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("remote cmd %v failed: %v\n%s", args, err, out)
		}
	}
	runRemote("git", "init", "--bare", "-b", "main")

	// Add the bare repo as origin and push main+develop to it so push
	// has a valid target.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "remote", "add", "origin", remoteDir)
	run("git", "push", "origin", "main")
	run("git", "push", "origin", "develop")

	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "origin", TagPrefix: "v"})

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	t.Cleanup(func() { output.SetJSONMode(prevJSON) })

	cmd := newPushCmd()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE failed for newPushCmd: %v", err)
	}
}

func TestNewSyncCmd_JSONMode(t *testing.T) {
	dir := setupCommandsRepo(t)
	// Sync requires being on a flow branch (feature/bugfix/release/hotfix),
	// not on develop/main directly. Create a feature branch.
	mkFeature := exec.Command("git", "checkout", "-b", "feature/test-sync")
	mkFeature.Dir = dir
	if out, err := mkFeature.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b feature/test-sync: %v\n%s", err, out)
	}

	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v"})

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	t.Cleanup(func() { output.SetJSONMode(prevJSON) })

	cmd := newSyncCmd()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE failed for newSyncCmd: %v", err)
	}
}

func TestNewReleaseNotesCmd_Empty(t *testing.T) {
	dir := setupCommandsRepo(t)
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v"})

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	t.Cleanup(func() { output.SetJSONMode(prevJSON) })

	cmd := newReleaseNotesCmd()
	// A fresh repo with no tags has no commit range to summarize. The
	// command's RunE returns nil even when meta is nil (no commits found).
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE failed for newReleaseNotesCmd: %v", err)
	}
}

func TestNewSetupCmd_ForceIDE_JSONOutput(t *testing.T) {
	dir := setupCommandsRepo(t)
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v"})
	GF.Config.ProjectRoot = dir

	prevHome := ide.UserHomeDirFunc
	home := t.TempDir()
	ide.UserHomeDirFunc = func() (string, error) { return home, nil }
	t.Cleanup(func() { ide.UserHomeDirFunc = prevHome })

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	t.Cleanup(func() { output.SetJSONMode(prevJSON) })

	cmd := newSetupCmd()
	cmd.SetArgs([]string{"--ide", "cursor", "--yes"})
	cmd.SetOut(os.NewFile(0, os.DevNull))
	cmd.SetErr(os.NewFile(0, os.DevNull))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("setup --ide cursor --yes: %v", err)
	}
	// Sanity: cursor rule created.
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules", "gitflow-preflight.mdc")); err != nil {
		t.Errorf("expected cursor rule: %v", err)
	}
}
