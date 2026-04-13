package flow

import (
	"fmt"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
)

func Cleanup(cfg config.FlowConfig) (int, map[string]any) {
	output.Infof("\n  %sCleaning up merged branches...%s", output.Bold, output.Reset)

	mergedDevelop := make(map[string]bool)
	mergedMain := make(map[string]bool)

	code, _, _ := git.ExecResult("rev-parse", "--verify", cfg.DevelopBranch)
	if code == 0 {
		for _, b := range git.ExecLines("branch", "--merged", cfg.DevelopBranch, "--format=%(refname:short)") {
			mergedDevelop[b] = true
		}
	}
	code, _, _ = git.ExecResult("rev-parse", "--verify", cfg.MainBranch)
	if code == 0 {
		for _, b := range git.ExecLines("branch", "--merged", cfg.MainBranch, "--format=%(refname:short)") {
			mergedMain[b] = true
		}
	}

	protected := map[string]bool{
		cfg.MainBranch:       true,
		cfg.DevelopBranch:    true,
		"master":             true,
		git.CurrentBranch(): true,
	}

	flowPrefixes := []string{"feature/", "bugfix/", "release/", "hotfix/"}
	var toDelete []string

	for b := range mergedDevelop {
		if !protected[b] && isFlowBranch(b, flowPrefixes) {
			toDelete = append(toDelete, b)
		}
	}
	for b := range mergedMain {
		if !protected[b] && isFlowBranch(b, flowPrefixes) {
			already := false
			for _, d := range toDelete {
				if d == b {
					already = true
					break
				}
			}
			if !already {
				toDelete = append(toDelete, b)
			}
		}
	}

	if len(toDelete) == 0 {
		output.Infof("  %sNo merged branches to clean up.%s", output.Green, output.Reset)
		return 0, map[string]any{"action": "cleanup", "deleted": []string{}, "result": "nothing_to_clean"}
	}

	output.Info("\n  Branches to delete:")
	for _, b := range toDelete {
		output.Infof("    %s●%s %s", output.Dim, output.Reset, b)
	}

	for _, b := range toDelete {
		_ = git.Exec("branch", "-d", b)
	}

	output.Infof("  %sCleanup complete.%s", output.Green, output.Reset)
	return 0, map[string]any{"action": "cleanup", "deleted": toDelete, "result": "ok"}
}

func isFlowBranch(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func SmartStashSave(branch string) bool {
	if !git.HasUncommittedChanges() {
		return false
	}
	msg := fmt.Sprintf("gitflow-auto: %s", branch)
	err := git.Exec("stash", "push", "-m", msg)
	return err == nil
}

func SmartStashPop(branch string) bool {
	stashes := git.ExecLines("stash", "list")
	needle := fmt.Sprintf("gitflow-auto: %s", branch)
	for i, line := range stashes {
		if strings.Contains(line, needle) {
			err := git.Exec("stash", "pop", fmt.Sprintf("stash@{%d}", i))
			if err == nil {
				output.Infof("  %sRestored stashed changes for '%s'.%s", output.Green, branch, output.Reset)
				return true
			}
			output.Infof("  %sStash pop had conflicts. Resolve manually.%s", output.Yellow, output.Reset)
			return false
		}
	}
	return false
}
