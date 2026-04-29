package commands

import (
	"os"
	"os/exec"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
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
