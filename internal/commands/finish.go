package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/flow"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newFinishCmd() *cobra.Command {
	var squash bool
	var rebase bool
	var runTests bool

	cmd := &cobra.Command{
		Use:   "finish [name]",
		Short: "Finish current or named branch (with pre-merge safety check)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}

			branch := git.CurrentBranch()
			btype := git.BranchTypeOf(branch)

			var code int
			var result map[string]any

			if runTests {
				code, result = GF.TestGatedFinish(name)
			} else if squash || rebase {
				// When explicit flags are provided, bypass SmartFinish and call
				// Finish directly with the user-requested options.
				opts := flow.FinishOptions{
					Rebase:       rebase,
					Squash:       squash,
					DeleteRemote: true,
				}
				code, result = GF.Finish(name, opts)
			} else if btype == "hotfix" {
				code, result = GF.SafeHotfixFinish(name)
			} else {
				code, result = GF.SmartFinish(name)
			}

			if output.IsJSONMode() {
				output.JSONOutput(result)
			}
			if code != 0 {
				if !output.IsJSONMode() {
					if msg, ok := result["error"].(string); ok && msg != "" {
						output.Infof("  %s✗ %s%s", output.Red, msg, output.Reset)
					}
				}
				os.Exit(code)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&squash, "squash", false, "Squash all branch commits into a single commit on develop")
	cmd.Flags().BoolVar(&rebase, "rebase", false, "Rebase the branch onto develop before the final merge")
	cmd.Flags().BoolVarP(&runTests, "run-tests", "t", false, "Run the project test suite; finish only if all tests pass")
	return cmd
}
