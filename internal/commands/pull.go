package commands

import (
	"os"

	"github.com/luis-lozano/gitflow-helper/internal/flow"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Safe fetch + fast-forward merge (never pushes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			code, result := flow.Pull(Cfg)
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
