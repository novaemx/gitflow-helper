package flow

import (
	"fmt"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
)

func Pull(cfg config.FlowConfig) (int, map[string]any) {
	branch := git.CurrentBranch()
	if branch == "" {
		return 1, map[string]any{"action": "pull", "error": "detached HEAD"}
	}

	stashed := false
	if git.HasUncommittedChanges() {
		output.Infof("  %sStashing uncommitted changes before pull...%s", output.Yellow, output.Reset)
		_ = git.Exec("stash", "push", "-m", "gitflow-auto-stash")
		stashed = true
	}

	output.Infof("\n  %sFetching from all remotes...%s", output.Bold, output.Reset)
	_ = git.Exec("fetch", "--all", "--prune")

	remoteBranch := git.ExecQuiet("config", "--get", "branch."+branch+".remote")
	if remoteBranch == "" {
		remoteBranch = cfg.Remote
	}

	mergeBranch := git.ExecQuiet("config", "--get", "branch."+branch+".merge")
	if mergeBranch == "" {
		trackingRef := remoteBranch + "/" + branch
		code, _, _ := git.ExecResult("rev-parse", "--verify", trackingRef)
		if code != 0 {
			output.Infof("  %sNo upstream tracking for '%s'. Nothing to pull.%s", output.Dim, branch, output.Reset)
			if stashed {
				_ = git.Exec("stash", "pop")
			}
			return 0, map[string]any{"action": "pull", "branch": branch, "result": "no_upstream"}
		}
	}

	mergeRef := fmt.Sprintf("%s/%s", remoteBranch, branch)
	code, _, _ := git.ExecResult("merge", "--ff-only", mergeRef)
	if code == 0 {
		output.Infof("  %sFast-forward merge successful for '%s'.%s", output.Green, branch, output.Reset)
		if stashed {
			popCode, _, _ := git.ExecResult("stash", "pop")
			if popCode != 0 {
				return 2, map[string]any{"action": "pull", "branch": branch, "result": "ok_stash_conflict"}
			}
		}
		return 0, map[string]any{"action": "pull", "branch": branch, "result": "fast_forward"}
	}

	output.Infof("  %sFast-forward not possible — branches have diverged.%s", output.Yellow, output.Reset)
	output.Infof("  %sAttempting rebase...%s", output.Dim, output.Reset)

	rebaseCode, _, _ := git.ExecResult("rebase", mergeRef)
	if rebaseCode == 0 {
		output.Infof("  %sRebase successful for '%s'.%s", output.Green, branch, output.Reset)
		if stashed {
			popCode, _, _ := git.ExecResult("stash", "pop")
			if popCode != 0 {
				return 2, map[string]any{"action": "pull", "branch": branch, "result": "rebase_ok_stash_conflict"}
			}
		}
		return 0, map[string]any{"action": "pull", "branch": branch, "result": "rebased"}
	}

	output.Infof("  %sRebase has conflicts. Aborting to preserve your code.%s", output.Red, output.Reset)
	_ = git.Exec("rebase", "--abort")
	if stashed {
		_ = git.Exec("stash", "pop")
	}
	return 2, map[string]any{"action": "pull", "branch": branch, "result": "conflict", "needs_human": true}
}

func Sync(cfg config.FlowConfig) (int, map[string]any) {
	branch := git.CurrentBranch()
	btype := git.BranchTypeOf(branch)

	var parent string
	switch btype {
	case "feature", "bugfix", "release":
		parent = cfg.DevelopBranch
	case "hotfix":
		parent = cfg.MainBranch
	default:
		return 1, map[string]any{"action": "sync", "error": "not on flow branch"}
	}

	output.Infof("\n  %sSyncing '%s' with '%s'...%s", output.Bold, branch, parent, output.Reset)
	// Fetch from remote if available, but don't fail if remote doesn't exist
	_ = git.Exec("fetch", cfg.Remote, parent)

	// Merge from local parent if remote ref doesn't exist
	mergeRef := fmt.Sprintf("%s/%s", cfg.Remote, parent)
	code, _, _ := git.ExecResult("rev-parse", "--verify", mergeRef)
	if code != 0 {
		mergeRef = parent
	}
	code, _, _ = git.ExecResult("merge", "--no-ff", mergeRef)
	if code == 0 {
		output.Infof("  %sSync successful.%s", output.Green, output.Reset)
		return 0, map[string]any{"action": "sync", "branch": branch, "parent": parent, "result": "ok"}
	}

	conflicts := git.ExecLines("diff", "--name-only", "--diff-filter=U")
	if len(conflicts) > 0 {
		output.Infof("  %sMerge conflicts during sync:%s", output.Red, output.Reset)
		if output.IsJSONMode() {
			_ = git.Exec("merge", "--abort")
			return 2, map[string]any{
				"action": "sync", "branch": branch, "parent": parent,
				"result": "conflict", "files": conflicts, "needs_human": true,
			}
		}
	}

	return 1, map[string]any{"action": "sync", "branch": branch, "result": "error"}
}

func Backmerge(cfg config.FlowConfig) (int, map[string]any) {
	behind := git.ExecQuiet("rev-list", "--count", cfg.DevelopBranch+".."+cfg.MainBranch)
	behindCount := 0
	fmt.Sscanf(behind, "%d", &behindCount)

	if behindCount == 0 {
		output.Infof("  %s%s already contains all commits from %s.%s",
			output.Green, cfg.DevelopBranch, cfg.MainBranch, output.Reset)
		return 0, map[string]any{"action": "backmerge", "result": "up_to_date"}
	}

	output.Infof("\n  %sBack-merging %s into %s...%s", output.Bold, cfg.MainBranch, cfg.DevelopBranch, output.Reset)
	output.Infof("  %s has %s%d%s commit(s) not in %s.",
		cfg.MainBranch, output.Yellow, behindCount, output.Reset, cfg.DevelopBranch)

	changedFiles := git.ExecLines("diff", "--name-only", cfg.DevelopBranch+"..."+cfg.MainBranch)

	originalBranch := git.CurrentBranch()
	if originalBranch != cfg.DevelopBranch {
		output.Infof("  Switching to %s...", cfg.DevelopBranch)
		err := git.Exec("checkout", cfg.DevelopBranch)
		if err != nil {
			return 1, map[string]any{"action": "backmerge", "result": "error", "detail": "checkout_failed"}
		}
	}

	// Fetch from remote if available
	_ = git.Exec("fetch", cfg.Remote, cfg.MainBranch)
	mergeMsg := fmt.Sprintf("Merge %s into %s (backmerge)", cfg.MainBranch, cfg.DevelopBranch)
	code, _, _ := git.ExecResult("merge", "--no-ff", cfg.MainBranch, "-m", mergeMsg)

	if code == 0 {
		output.Infof("  %sBack-merge successful. %s now contains all of %s.%s",
			output.Green, cfg.DevelopBranch, cfg.MainBranch, output.Reset)
		return 0, map[string]any{
			"action": "backmerge", "result": "ok",
			"commits_merged": behindCount, "files_merged": changedFiles,
		}
	}

	conflicts := git.ExecLines("diff", "--name-only", "--diff-filter=U")
	if len(conflicts) > 0 {
		if output.IsJSONMode() {
			_ = git.Exec("merge", "--abort")
			return 2, map[string]any{
				"action": "backmerge", "result": "conflict",
				"files": conflicts, "needs_human": true,
			}
		}
		return 2, map[string]any{"action": "backmerge", "result": "conflict", "files": conflicts}
	}

	return 1, map[string]any{"action": "backmerge", "result": "error"}
}
