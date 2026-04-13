package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync current branch with its parent",
		RunE: func(cmd *cobra.Command, args []string) error {
			code, result := GF.Sync()
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
