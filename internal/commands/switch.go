package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/flow"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch [branch]",
		Short: "Switch to a gitflow branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cur := git.CurrentBranch()
			flowBranches := GF.ListSwitchable()

			if len(flowBranches) == 0 {
				output.Infof("  %sNo other branches to switch to.%s", output.Yellow, output.Reset)
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "switch", "result": "no_branches"})
				}
				os.Exit(1)
				return nil
			}

			var target string
			if len(args) > 0 {
				target = args[0]
			}

			if target == "" && output.IsJSONMode() {
				output.JSONOutput(map[string]any{
					"action":    "switch",
					"result":    "branch_required",
					"available": flowBranches,
				})
				os.Exit(1)
				return nil
			}

			if target == "" {
				output.Infof("  Available branches:")
				for _, b := range flowBranches {
					output.Infof("    %s", b)
				}
				return nil
			}

			var chosen string
			for _, b := range flowBranches {
				if b == target || len(b) > len(target) && b[len(b)-len(target)-1:] == "/"+target {
					chosen = b
					break
				}
			}
			if chosen == "" {
				output.Infof("  %sBranch '%s' not found.%s", output.Red, target, output.Reset)
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{
						"action":    "switch",
						"result":    "not_found",
						"target":    target,
						"available": flowBranches,
					})
				}
				os.Exit(1)
				return nil
			}

			stashed := false
			if git.HasUncommittedChanges() {
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{
						"action": "switch",
						"result": "dirty",
						"detail": "uncommitted changes — stash or commit first",
					})
					os.Exit(1)
					return nil
				}
				stashed = flow.SmartStashSave(cur)
			}

			code, _, _ := git.ExecResult("checkout", chosen)
			if code != 0 {
				output.Infof("  %sFailed to switch to '%s'.%s", output.Red, chosen, output.Reset)
				if stashed {
					flow.SmartStashPop(cur)
				}
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "switch", "result": "error", "target": chosen})
				}
				os.Exit(1)
				return nil
			}

			flow.SmartStashPop(chosen)
			output.Infof("  %sSwitched to '%s'.%s", output.Green, chosen, output.Reset)
			if output.IsJSONMode() {
				output.JSONOutput(map[string]any{
					"action":   "switch",
					"result":   "ok",
					"branch":   chosen,
					"previous": cur,
				})
			}
			return nil
		},
	}
}
