package commands

import (
	"os"

	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newFinishCmd() *cobra.Command {
	return &cobra.Command{
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

			if btype == "hotfix" {
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
}
