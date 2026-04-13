package commands

import (
	"os"

	"github.com/luis-lozano/gitflow-helper/internal/flow"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newBackmergeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backmerge",
		Short: "Merge main into develop (restore gitflow invariant)",
		RunE: func(cmd *cobra.Command, args []string) error {
			code, result := flow.Backmerge(Cfg)
			if output.IsJSONMode() {
				output.JSONOutput(result)
			}
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}
