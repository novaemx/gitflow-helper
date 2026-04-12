package commands

import (
	"fmt"
	"os"

	"github.com/luis-lozano/gitflow-helper/internal/gitflow"
	"github.com/luis-lozano/gitflow-helper/internal/output"
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
			output.SetJSONMode(jsonFlag)

			GF = gitflow.New("")

			// Skip git checks for help/version/completion subcommands.
			name := cmd.Name()
			if name == "help" || name == "completion" {
				return
			}

			if !GF.IsGitAvailable() {
				fmt.Fprintln(os.Stderr, "fatal: git is not installed or not in PATH")
				os.Exit(1)
			}

			if !GF.IsGitRepo() {
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

			// Auto-initialize gitflow structure if running a command that needs it
			if name != "doctor" && name != "health" && name != "setup" {
				if !GF.IsGitFlowInitialized() {
					if !output.IsJSONMode() {
						output.Infof("  %sGitflow structure not detected. Auto-initializing...%s", output.Yellow, output.Reset)
					}
					ok, msg := GF.Init()
					if !ok {
						if output.IsJSONMode() {
							output.JSONOutput(map[string]any{
								"error":   "init_failed",
								"message": msg,
							})
						}
						os.Exit(1)
					}
				}
			}

			// Auto-provision IDE rules when missing (silent, idempotent)
			if name != "doctor" && name != "health" {
				_, _ = GF.EnsureRules()
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
