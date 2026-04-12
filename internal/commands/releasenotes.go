package commands

import (
	"github.com/luis-lozano/gitflow-helper/internal/flow"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newReleaseNotesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "releasenotes [from-tag]",
		Short: "Generate user-facing release notes from git history",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromTag := ""
			if len(args) > 0 {
				fromTag = args[0]
			}
			meta := GF.ReleaseNotes(fromTag)
			if meta == nil {
				output.Infof("  %sNo commits found for release notes.%s", output.Yellow, output.Reset)
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "releasenotes", "result": "empty"})
				}
				return nil
			}

			if output.IsJSONMode() {
				result := map[string]any{"action": "releasenotes", "result": "ok"}
				for k, v := range meta {
					result[k] = v
				}
				output.JSONOutput(result)
			} else {
				flow.PrintReleaseNotes(meta)
			}
			return nil
		},
	}
}
