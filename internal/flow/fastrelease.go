package flow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/novaemx/gitflow-helper/internal/version"
)

// FastRelease merges a feature/bugfix branch directly to main, bypassing the
// release/ staging phase. Useful for small, self-contained changes that are
// ready for production without a batched release.
//
// Flow:
//  1. Invariant check (main must not be ahead of develop)
//  2. Merge feature → main (--no-ff)
//  3. Tag main with the current version
//  4. Merge feature → develop (back-merge, keeps develop consistent)
//  5. Delete the feature branch (local; remote via tryDeleteRemote)
//  6. Return to develop
func FastRelease(cfg config.FlowConfig, featureName string) (int, map[string]any) {
	result := map[string]any{
		"action": "fast-release",
		"name":   featureName,
	}

	// Normalise branch name
	branchName := featureName
	if !strings.Contains(featureName, "/") {
		branchName = "feature/" + featureName
	}
	btype := git.BranchTypeOf(branchName)
	if btype != "feature" && btype != "bugfix" {
		result["result"] = "error"
		result["error"] = "fast-release only supports feature or bugfix branches"
		return 1, result
	}

	if !git.BranchExists(branchName) {
		result["result"] = "error"
		result["error"] = fmt.Sprintf("branch %s does not exist", branchName)
		return 1, result
	}

	result["branch"] = branchName

	// Dirty-tree guard
	wt := git.WorkingTreeStatus()
	if wt.Staged > 0 || wt.Unstaged > 0 {
		result["result"] = "error"
		result["error"] = "dirty working tree: commit or stash first"
		return 1, result
	}

	// Invariant: main must not be ahead of develop
	if raw := git.ExecQuiet("rev-list", "--count", cfg.DevelopBranch+".."+cfg.MainBranch); true {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			result["result"] = "error"
			result["error"] = fmt.Sprintf("failed to parse divergence count %q: %v", strings.TrimSpace(raw), err)
			return 1, result
		}
		if n > 0 {
			output.Infof("  %s✗ %s is %d commit(s) ahead of %s — run 'gitflow backmerge' first.%s",
				output.Red, cfg.MainBranch, n, cfg.DevelopBranch, output.Reset)
			result["result"] = "error"
			result["error"] = fmt.Sprintf("%s is %d commit(s) ahead of %s — backmerge required", cfg.MainBranch, n, cfg.DevelopBranch)
			result["action_required"] = "backmerge"
			return 1, result
		}
	}

	// Determine version for tagging
	ver := version.ReadVersion(cfg)
	if ver == "" || ver == "0.0.0" {
		result["result"] = "error"
		result["error"] = fmt.Sprintf("cannot determine version; update %s", cfg.VersionFile)
		return 1, result
	}

	tagName := cfg.TagPrefix + ver
	if git.TagExists(tagName) {
		next, err := nextAvailableTagVersion(cfg, ver)
		if err != nil {
			result["result"] = "error"
			result["error"] = err.Error()
			return 1, result
		}
		output.Infof("  %s⚠ Tag %s already exists; auto-bumping to %s.%s", output.Yellow, tagName, next, output.Reset)
		ver = next
		tagName = cfg.TagPrefix + ver
		if cfg.VersionFile != "" {
			version.WriteVersionFile(cfg, ver)
			if err := git.Exec("add", cfg.VersionFile); err != nil {
				result["result"] = "error"
				result["error"] = "failed to stage version file: " + err.Error()
				return 1, result
			}
			if git.HasStagedChanges() {
				branchSave := git.CurrentBranch()
				if branchSave != branchName {
					if err := git.Exec("checkout", branchName); err != nil {
						result["result"] = "error"
						result["error"] = "failed to checkout feature branch for version bump commit"
						return 1, result
					}
				}
				_ = git.Exec("commit", "-m", fmt.Sprintf("chore: bump version to %s", ver))
			}
		}
		result["auto_bumped_to"] = ver
	}

	result["version"] = ver
	result["tag"] = tagName
	shortName := strings.TrimPrefix(branchName, btype+"/")

	// Step 1: merge feature → main
	output.Infof("\n  %sFast-release: merging %s into %s...%s", output.Bold, branchName, cfg.MainBranch, output.Reset)
	if err := git.Exec("checkout", cfg.MainBranch); err != nil {
		result["result"] = "error"
		result["error"] = "failed to checkout " + cfg.MainBranch + ": " + err.Error()
		return 1, result
	}
	mainMergeMsg := fmt.Sprintf("Merge %s '%s' into %s (fast-release %s)", btype, shortName, cfg.MainBranch, ver)
	if code, _, stderr := git.ExecResult("merge", "--no-ff", branchName, "-m", mainMergeMsg); code != 0 {
		conflicts := git.ExecLines("diff", "--name-only", "--diff-filter=U")
		result["result"] = "conflict"
		result["error"] = stderr
		result["needs_human"] = true
		if len(conflicts) > 0 {
			result["conflicts"] = conflicts
		}
		addMergeAbortDiagnostics(result)
		return 2, result
	}

	// Step 2: tag main
	if err := git.Exec("tag", "-a", tagName, "-m", fmt.Sprintf("Fast-release %s", ver)); err != nil {
		result["result"] = "error"
		result["error"] = "tag creation failed: " + err.Error()
		return 1, result
	}

	// Step 3: merge feature → develop (keeps develop consistent with main)
	output.Infof("  %sMerging %s into %s...%s", output.Dim, branchName, cfg.DevelopBranch, output.Reset)
	if err := git.Exec("checkout", cfg.DevelopBranch); err != nil {
		result["result"] = "error"
		result["error"] = "failed to checkout " + cfg.DevelopBranch + ": " + err.Error()
		return 1, result
	}
	devMergeMsg := fmt.Sprintf("Merge %s '%s' into %s (fast-release back-merge)", btype, shortName, cfg.DevelopBranch)
	if code, _, stderr := git.ExecResult("merge", "--no-ff", branchName, "-m", devMergeMsg); code != 0 {
		conflicts := git.ExecLines("diff", "--name-only", "--diff-filter=U")
		result["result"] = "conflict"
		result["error"] = stderr
		result["needs_human"] = true
		result["action_required"] = "backmerge"
		result["warning"] = fmt.Sprintf("feature was merged into %s and tagged %s; back-merge to %s failed", cfg.MainBranch, tagName, cfg.DevelopBranch)
		if len(conflicts) > 0 {
			result["conflicts"] = conflicts
		}
		addMergeAbortDiagnostics(result)
		return 2, result
	}

	// Step 4: delete branch (local; remote best-effort)
	tryDeleteRemote(cfg, branchName, git.RemoteExists(cfg.Remote))
	if err := git.Exec("branch", "-d", branchName); err != nil {
		output.Infof("  %s%s%s", output.Yellow, mergedBranchDeleteWarning(branchName, err), output.Reset)
	}

	output.Infof("  %s✓ Fast-release %s: %s → %s (tagged %s) → %s%s",
		output.Green, ver, branchName, cfg.MainBranch, tagName, cfg.DevelopBranch, output.Reset)

	result["result"] = "ok"
	result["landed_on"] = cfg.DevelopBranch
	return 0, result
}
