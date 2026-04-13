package commands

import (
	"os"

	"github.com/luis-lozano/gitflow-helper/internal/flow"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize gitflow structure (main + develop branches)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ok, result := flow.EnsureGitFlowReady(Cfg)
			if output.IsJSONMode() {
				output.JSONOutput(map[string]any{"action": "init", "result": result})
			}
			if !ok {
				os.Exit(1)
			}
			return nil
		},
	}
}
