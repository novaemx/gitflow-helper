package commands

import (
	"os"

	"github.com/luis-lozano/gitflow-helper/internal/flow"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync current branch with its parent",
		RunE: func(cmd *cobra.Command, args []string) error {
			code, result := flow.Sync(Cfg)
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
