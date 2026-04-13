package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <type> <name>",
		Short: "Start a feature/bugfix/release/hotfix",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, result := GF.Start(args[0], args[1])
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
