package commands

import (
	"os"

	"github.com/luis-lozano/gitflow-helper/internal/flow"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newFinishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "finish [name]",
		Short: "Finish current or named branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			code, result := flow.FinishCurrent(Cfg, name)
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
