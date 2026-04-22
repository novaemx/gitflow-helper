package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit <message>",
		Short: "Stage all changes and commit (enables finish on dirty branches)",
		Long:  "Stages all tracked and untracked files (git add -A) and creates a commit with the given message. Intended to clean up a dirty feature or bugfix branch so that 'gitflow finish' can proceed.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg := strings.Join(args, " ")

			// Stage everything: tracked modifications, deletions, and new files.
			if err := git.ExecSilent("add", "-A"); err != nil {
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{
						"result": "error",
						"error":  fmt.Sprintf("git add failed: %v", err),
					})
				} else {
					output.Infof("  %s✗ stage failed: %v%s", output.Red, err, output.Reset)
				}
				os.Exit(1)
			}

			code, _, stderr := git.ExecResult("commit", "-m", msg)
			if code != 0 {
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{
						"result": "error",
						"error":  fmt.Sprintf("git commit failed: %s", strings.TrimSpace(stderr)),
					})
				} else {
					output.Infof("  %s✗ commit failed: %s%s", output.Red, strings.TrimSpace(stderr), output.Reset)
				}
				os.Exit(code)
			}

			if output.IsJSONMode() {
				output.JSONOutput(map[string]any{"result": "ok", "message": msg})
			}
			return nil
		},
	}
}
