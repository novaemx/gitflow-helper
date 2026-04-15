package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push [target-branch]",
		Short: "Push current local branch to remote with target validation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = args[0]
			}

			code, result := GF.Push(target)
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
