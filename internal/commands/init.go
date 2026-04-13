package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize gitflow structure (main + develop branches)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ok, result := GF.Init()
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
