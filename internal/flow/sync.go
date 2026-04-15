package flow

import (
	"fmt"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
)

var execResult = git.ExecResult

func buildMergeConflictResult(action, branch, parent string, conflicts []string) (int, map[string]any) {
	result := map[string]any{
		"action":      action,
		"result":      "conflict",
		"files":       conflicts,
		"needs_human": true,
	}
	if branch != "" {
		result["branch"] = branch
	}
	if parent != "" {
		result["parent"] = parent
	}

	if output.IsJSONMode() {
		abortCode, _, abortErr := execResult("merge", "--abort")
		if abortCode != 0 {
			result["abort_failed"] = true
			if strings.TrimSpace(abortErr) != "" {
				result["abort_error"] = strings.TrimSpace(abortErr)
			}
		}
	}

	return 2, result
}

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

	remoteBranch := git.ExecQuiet("config", "--get", "branch."+branch+".remote")
	if remoteBranch == "" {
		remoteBranch = cfg.Remote
	}
	if remoteBranch == "" || !git.RemoteExists(remoteBranch) {
		output.Infof("  %sNo remote '%s' configured. Pull skipped (local-only mode).%s", output.Dim, remoteBranch, output.Reset)
		if stashed {
			_ = git.Exec("stash", "pop")
		}
		return 0, map[string]any{"action": "pull", "branch": branch, "result": "no_remote", "remote": remoteBranch}
	}

	output.Infof("\n  %sFetching from %s...%s", output.Bold, remoteBranch, output.Reset)
	_ = git.Exec("fetch", remoteBranch, "--prune")

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
	code, _, _ := execResult("merge", "--ff-only", mergeRef)
	if code == 0 {
		output.Infof("  %sFast-forward merge successful for '%s'.%s", output.Green, branch, output.Reset)
		if stashed {
			popCode, _, _ := execResult("stash", "pop")
			if popCode != 0 {
				return 2, map[string]any{"action": "pull", "branch": branch, "result": "ok_stash_conflict"}
			}
		}
		return 0, map[string]any{"action": "pull", "branch": branch, "result": "fast_forward"}
	}

	output.Infof("  %sFast-forward not possible — branches have diverged.%s", output.Yellow, output.Reset)
	output.Infof("  %sAttempting rebase...%s", output.Dim, output.Reset)

	rebaseCode, _, _ := execResult("rebase", mergeRef)
	if rebaseCode == 0 {
		output.Infof("  %sRebase successful for '%s'.%s", output.Green, branch, output.Reset)
		if stashed {
			popCode, _, _ := execResult("stash", "pop")
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
	// Fetch only when configured remote exists
	mergeRef := parent
	if cfg.Remote != "" && git.RemoteExists(cfg.Remote) {
		_ = git.Exec("fetch", cfg.Remote, parent)
		remoteRef := fmt.Sprintf("%s/%s", cfg.Remote, parent)
		code, _, _ := execResult("rev-parse", "--verify", remoteRef)
		if code == 0 {
			mergeRef = remoteRef
		}
	} else {
		output.Infof("  %sNo remote '%s' configured. Sync uses local '%s' branch.%s", output.Dim, cfg.Remote, parent, output.Reset)
	}

	// Feature and bugfix branches use rebase to keep the branch's commits
	// contiguous and conflict-resolution incremental (proactive-sync policy).
	// Release branches use merge because they may be shared with other developers.
	if btype == "feature" || btype == "bugfix" {
		rebaseCode, _, _ := execResult("rebase", mergeRef)
		if rebaseCode == 0 {
			output.Infof("  %sRebase sync successful.%s", output.Green, output.Reset)
			return 0, map[string]any{"action": "sync", "branch": branch, "parent": parent, "strategy": "rebase", "result": "ok"}
		}
		output.Infof("  %sRebase conflicts during sync — aborting. Resolve conflicts manually or run 'gitflow sync' after resolving.%s", output.Red, output.Reset)
		_ = git.Exec("rebase", "--abort")
		conflicts := git.ExecLines("diff", "--name-only", "--diff-filter=U")
		return buildMergeConflictResult("sync", branch, parent, conflicts)
	}

	code, _, _ := execResult("merge", "--no-ff", mergeRef)
	if code == 0 {
		output.Infof("  %sSync successful.%s", output.Green, output.Reset)
		return 0, map[string]any{"action": "sync", "branch": branch, "parent": parent, "strategy": "merge", "result": "ok"}
	}

	conflicts := git.ExecLines("diff", "--name-only", "--diff-filter=U")
	if len(conflicts) > 0 {
		output.Infof("  %sMerge conflicts during sync:%s", output.Red, output.Reset)
		return buildMergeConflictResult("sync", branch, parent, conflicts)
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

	// Fetch only when configured remote exists
	if cfg.Remote != "" && git.RemoteExists(cfg.Remote) {
		_ = git.Exec("fetch", cfg.Remote, cfg.MainBranch)
	} else {
		output.Infof("  %sNo remote '%s' configured. Back-merge uses local '%s' branch.%s", output.Dim, cfg.Remote, cfg.MainBranch, output.Reset)
	}
	mergeMsg := fmt.Sprintf("Merge %s into %s (backmerge)", cfg.MainBranch, cfg.DevelopBranch)
	code, _, _ := execResult("merge", "--no-ff", cfg.MainBranch, "-m", mergeMsg)

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
		return buildMergeConflictResult("backmerge", "", "", conflicts)
	}

	return 1, map[string]any{"action": "backmerge", "result": "error"}
}
