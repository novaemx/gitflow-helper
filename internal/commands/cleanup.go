package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Delete local branches merged into develop/main",
		RunE: func(cmd *cobra.Command, args []string) error {
			code, result := GF.Cleanup()
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
