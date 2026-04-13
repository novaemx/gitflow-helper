package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newBackmergeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backmerge",
		Short: "Merge main into develop (restore gitflow invariant)",
		RunE: func(cmd *cobra.Command, args []string) error {
			code, result := GF.Backmerge()
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
