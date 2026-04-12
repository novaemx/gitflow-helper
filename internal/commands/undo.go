package commands

import (
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: "Undo last gitflow operation (soft reset via reflog)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cur := git.CurrentBranch()
			reflog := git.ExecLines("reflog", "--format=%H %gs", "-n", "20")

			if len(reflog) == 0 {
				output.Infof("  %sNo reflog entries found.%s", output.Red, output.Reset)
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "undo", "result": "no_reflog"})
				}
				return nil
			}

			type candidate struct {
				SHA  string
				Desc string
			}
			keywords := []string{"merge", "checkout: moving", "commit", "finish", "start"}
			var candidates []candidate
			for _, entry := range reflog {
				parts := strings.SplitN(entry, " ", 2)
				if len(parts) < 2 {
					continue
				}
				sha, desc := parts[0], parts[1]
				descLower := strings.ToLower(desc)
				for _, kw := range keywords {
					if strings.Contains(descLower, kw) {
						candidates = append(candidates, candidate{sha, desc})
						break
					}
				}
				if len(candidates) >= 10 {
					break
				}
			}

			if len(candidates) == 0 {
				output.Infof("  %sNo undoable operations found in recent history.%s", output.Yellow, output.Reset)
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "undo", "result": "nothing_to_undo"})
				}
				return nil
			}

			if output.IsJSONMode() {
				entries := make([]map[string]string, len(candidates))
				for i, c := range candidates {
					sha := c.SHA
					if len(sha) > 12 {
						sha = sha[:12]
					}
					entries[i] = map[string]string{"sha": sha, "description": c.Desc}
				}
				output.JSONOutput(map[string]any{
					"action":         "undo",
					"result":         "candidates",
					"current_branch": cur,
					"entries":        entries,
				})
				return nil
			}

			output.Infof("\n  %sRecent operations (newest first):%s\n", output.Bold, output.Reset)
			for _, c := range candidates {
				sha := c.SHA
				if len(sha) > 10 {
					sha = sha[:10]
				}
				output.Infof("    %s%s%s  %s", output.Cyan, sha, output.Reset, c.Desc)
			}
			output.Info("\n  Use --json mode to get SHA values for programmatic reset.")
			return nil
		},
	}
}
