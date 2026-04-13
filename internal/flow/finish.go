package flow

import (
	"fmt"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
)

func finishFeatureOrBugfix(cfg config.FlowConfig, btype, name string) error {
	branchName := btype + "/" + name
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}

	if err := git.Exec("checkout", cfg.DevelopBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.DevelopBranch, err)
	}

	mergeMsg := fmt.Sprintf("Merge %s '%s' into %s", btype, name, cfg.DevelopBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", mergeMsg); err != nil {
		return fmt.Errorf("merge of %s failed (conflicts?): %w", branchName, err)
	}

	_ = git.Exec("branch", "-d", branchName)
	output.Infof("  %s%s '%s' merged into %s and branch deleted.%s", output.Green, btype, name, cfg.DevelopBranch, output.Reset)
	return nil
}

func finishRelease(cfg config.FlowConfig, ver string) error {
	branchName := "release/" + ver
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}
	tagName := cfg.TagPrefix + ver

	if err := git.Exec("checkout", cfg.MainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.MainBranch, err)
	}

	mergeMsg := fmt.Sprintf("Merge release '%s' into %s", ver, cfg.MainBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", mergeMsg); err != nil {
		return fmt.Errorf("merge of %s into %s failed: %w", branchName, cfg.MainBranch, err)
	}

	_ = git.Exec("tag", "-a", tagName, "-m", fmt.Sprintf("Release %s", ver))

	if err := git.Exec("checkout", cfg.DevelopBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.DevelopBranch, err)
	}

	backmergeMsg := fmt.Sprintf("Merge tag '%s' back into %s", tagName, cfg.DevelopBranch)
	if err := git.Exec("merge", "--no-ff", tagName, "-m", backmergeMsg); err != nil {
		return fmt.Errorf("back-merge of %s into %s failed: %w", tagName, cfg.DevelopBranch, err)
	}

	_ = git.Exec("branch", "-d", branchName)
	output.Infof("  %sRelease %s merged into %s (tagged %s) and back-merged into %s.%s",
		output.Green, ver, cfg.MainBranch, tagName, cfg.DevelopBranch, output.Reset)
	return nil
}

func finishHotfix(cfg config.FlowConfig, ver string) error {
	branchName := "hotfix/" + ver
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}
	tagName := cfg.TagPrefix + ver

	if err := git.Exec("checkout", cfg.MainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.MainBranch, err)
	}

	mergeMsg := fmt.Sprintf("Merge hotfix '%s' into %s", ver, cfg.MainBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", mergeMsg); err != nil {
		return fmt.Errorf("merge of %s into %s failed: %w", branchName, cfg.MainBranch, err)
	}

	_ = git.Exec("tag", "-a", tagName, "-m", fmt.Sprintf("Hotfix %s", ver))

	releases := git.ActiveReleaseBranches()
	backTarget := cfg.DevelopBranch
	if len(releases) > 0 {
		backTarget = releases[0]
		output.Infof("  %sNote:%s Hotfix will merge into active release '%s' (nvie rule).",
			output.Yellow, output.Reset, backTarget)
	}

	if err := git.Exec("checkout", backTarget); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", backTarget, err)
	}

	backmergeMsg := fmt.Sprintf("Merge hotfix '%s' into %s", ver, backTarget)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", backmergeMsg); err != nil {
		return fmt.Errorf("back-merge of hotfix into %s failed: %w", backTarget, err)
	}

	_ = git.Exec("branch", "-d", branchName)
	output.Infof("  %sHotfix %s merged into %s (tagged %s) and back-merged into %s.%s",
		output.Green, ver, cfg.MainBranch, tagName, backTarget, output.Reset)
	return nil
}

func FinishCurrent(cfg config.FlowConfig, name string) (int, map[string]any) {
	branch := git.CurrentBranch()
	btype := git.BranchTypeOf(branch)

	if btype != "feature" && btype != "bugfix" && btype != "release" && btype != "hotfix" {
		if name != "" {
			for _, prefix := range []string{"feature/", "bugfix/", "release/", "hotfix/"} {
				if strings.HasPrefix(name, prefix) {
					btype = strings.TrimSuffix(prefix, "/")
					name = strings.TrimPrefix(name, prefix)
					break
				}
			}
		}
		if btype != "feature" && btype != "bugfix" && btype != "release" && btype != "hotfix" {
			return 1, map[string]any{"action": "finish", "error": "not on flow branch"}
		}
	}

	if name == "" {
		prefixes := map[string]string{
			"feature": "feature/",
			"bugfix":  "bugfix/",
			"release": "release/",
			"hotfix":  "hotfix/",
		}
		name = strings.TrimLeft(strings.TrimPrefix(branch, prefixes[btype]), "v")
	}

	result := map[string]any{
		"action": "finish",
		"type":   btype,
		"name":   name,
	}

	// Phase 1: gitflow auto-commits (release notes, version bump)
	// These run BEFORE dirty check so gitflow's own generated files don't block finish.
	if btype == "release" || btype == "hotfix" {
		output.Infof("  %sGenerating release notes...%s", output.Dim, output.Reset)
		meta := WriteReleaseNotes(cfg, "")
		if meta != nil {
			result["release_notes"] = meta
			_ = git.Exec("add", "RELEASE_NOTES.md")
			_ = git.Exec("commit", "-m", fmt.Sprintf("docs: release notes for %s %s", btype, name))
			output.Infof("  %s✓ RELEASE_NOTES.md committed to %s branch.%s", output.Green, btype, output.Reset)
		}
	}

	// Phase 2: dirty check — only blocks on USER changes (gitflow files already committed above)
	wt := git.WorkingTreeStatus()
	if wt.Staged > 0 || wt.Unstaged > 0 {
		var parts []string
		if wt.Staged > 0 {
			parts = append(parts, fmt.Sprintf("%d staged", wt.Staged))
		}
		if wt.Unstaged > 0 {
			parts = append(parts, fmt.Sprintf("%d modified", wt.Unstaged))
		}
		if wt.Untracked > 0 {
			parts = append(parts, fmt.Sprintf("%d untracked", wt.Untracked))
		}
		detail := strings.Join(parts, ", ")
		output.Infof("  %s✗ Cannot finish: working tree has uncommitted changes (%s).%s",
			output.Red, detail, output.Reset)
		output.Infof("  %sCommit or stash your changes first, then retry.%s",
			output.Dim, output.Reset)
		result["result"] = "error"
		result["error"] = fmt.Sprintf("dirty working tree: %s", detail)
		result["dirty"] = map[string]int{
			"staged": wt.Staged, "modified": wt.Unstaged, "untracked": wt.Untracked,
		}
		return 1, result
	}

	if wt.Untracked > 0 {
		output.Infof("  %sWarning:%s %d untracked file(s) — won't affect merge but consider committing.",
			output.Yellow, output.Reset, wt.Untracked)
		result["warning_untracked"] = wt.Untracked
	}

	// Phase 3: merge
	var err error
	switch btype {
	case "feature", "bugfix":
		err = finishFeatureOrBugfix(cfg, btype, name)
	case "release":
		err = finishRelease(cfg, name)
	case "hotfix":
		err = finishHotfix(cfg, name)
	}

	if err != nil {
		result["result"] = "error"
		result["error"] = err.Error()
		conflicts := git.ExecLines("diff", "--name-only", "--diff-filter=U")
		if len(conflicts) > 0 {
			output.Infof("\n  %sMerge conflict detected during %s finish.%s",
				output.Red, btype, output.Reset)
			result["conflicts"] = conflicts
			result["needs_human"] = true
			return 2, result
		}
		return 1, result
	}

	result["result"] = "ok"
	return 0, result
}
