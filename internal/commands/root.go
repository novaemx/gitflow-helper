package commands

import (
	"fmt"
	"os"

	"github.com/novaemx/gitflow-helper/internal/debug"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

var (
	jsonFlag bool
	GF       *gitflow.Logic
)

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "gitflow",
		Short: "Git Flow helper — interactive TUI + CLI subcommands",
		Long:  "A comprehensive Git Flow workflow helper with an interactive TUI and CLI subcommands for agent and human use.\nOnly requires git — no git-flow extensions needed.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			deferTotal := debug.Start("root.PersistentPreRun.total")
			defer deferTotal()

			output.SetJSONMode(jsonFlag)

			deferNew := debug.Start("root.PersistentPreRun.gitflow.New")
			GF = gitflow.New("")
			deferNew()
			GF.AppVersion = version

			// Skip git checks for help/version/completion subcommands.
			name := cmd.Name()
			if name == "help" || name == "completion" {
				return
			}

			deferGitAvail := debug.Start("root.PersistentPreRun.IsGitAvailable")
			if !GF.IsGitAvailable() {
				deferGitAvail()
				fmt.Fprintln(os.Stderr, "fatal: git is not installed or not in PATH")
				os.Exit(1)
			}
			deferGitAvail()

			deferIsRepo := debug.Start("root.PersistentPreRun.IsGitRepo")
			if !GF.IsGitRepo() {
				deferIsRepo()
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{
						"error":   "not_a_git_repo",
						"cwd":     GF.Config.ProjectRoot,
						"message": "Run 'git init' first, then 'gitflow init'.",
					})
					os.Exit(1)
				}
				output.Infof("  %sNot inside a git repository.%s", output.Red, output.Reset)
				output.Infof("  Run %sgit init%s first, then %sgitflow init%s.", output.Bold, output.Reset, output.Bold, output.Reset)
				os.Exit(1)
			}
			deferIsRepo()

			// Auto-initialize gitflow structure if running a command that needs it
			if name != "doctor" && name != "health" && name != "setup" {
				deferInitCheck := debug.Start("root.PersistentPreRun.IsGitFlowInitialized")
				if !GF.IsGitFlowInitialized() {
					deferInitCheck()
					if !output.IsJSONMode() {
						output.Infof("  %sGitflow structure not detected. Auto-initializing...%s", output.Yellow, output.Reset)
					}
					deferInit := debug.Start("root.PersistentPreRun.GF.Init")
					ok, msg := GF.Init()
					deferInit()
					if !ok {
						if output.IsJSONMode() {
							output.JSONOutput(map[string]any{
								"error":   "init_failed",
								"message": msg,
							})
						}
						os.Exit(1)
					}
				} else {
					deferInitCheck()
				}
			}

			// Auto-provision IDE rules when missing (silent, idempotent)
			if name != "doctor" && name != "health" {
				deferEnsure := debug.Start("root.PersistentPreRun.GF.EnsureRules")
				_, _ = GF.EnsureRules()
				deferEnsure()
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonFlag {
				return statusCmd.RunE(cmd, args)
			}
			return runTUI()
		},
		Version: version,
	}

	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Machine-readable JSON output (for agents)")

	root.AddCommand(
		newStatusCmd(),
		newPullCmd(),
		newInitCmd(),
		newStartCmd(),
		newFinishCmd(),
		newSyncCmd(),
		newSwitchCmd(),
		newBackmergeCmd(),
		newCleanupCmd(),
		newHealthCmd(),
		newDoctorCmd(),
		newLogCmd(),
		newUndoCmd(),
		newReleaseNotesCmd(),
		newSetupCmd(),
		newServeCmd(),
	)

	return root
}
