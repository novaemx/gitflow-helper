package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newFastReleaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fast-release <feature-name>",
		Short: "Merge a feature branch directly to main (skip release/ phase)",
		Long: `fast-release merges a feature or bugfix branch directly into main and tags
the release, bypassing the release/ staging phase. Use this for small,
self-contained changes that are ready for production without a batched release.

Flow:
  1. Verify invariant: main must not be ahead of develop (backmerge first if so)
  2. Merge feature → main  (--no-ff)
  3. Tag main with the current VERSION
  4. Merge feature → develop (back-merge for consistency)
  5. Delete local and remote branches`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			featureName := args[0]

			code, result := GF.FastRelease(featureName)

			if output.IsJSONMode() {
				output.JSONOutput(result)
			}
			if code != 0 {
				if !output.IsJSONMode() {
					if msg, ok := result["error"].(string); ok && msg != "" {
						output.Infof("  %s✗ %s%s", output.Red, msg, output.Reset)
					}
				}
				os.Exit(code)
			}
			return nil
		},
	}
}
