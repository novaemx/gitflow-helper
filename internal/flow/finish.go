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

var execResultFinish = git.ExecResult

func mergedBranchDeleteWarning(branchName string, err error) string {
	return fmt.Sprintf("Warning: merged branch %s not deleted automatically (%v). You can remove manually with git branch -d %s.", branchName, err, branchName)
}

func addMergeAbortDiagnostics(result map[string]any) {
	if !output.IsJSONMode() {
		return
	}
	abortCode, _, abortErr := execResultFinish("merge", "--abort")
	if abortCode != 0 {
		result["abort_failed"] = true
		if strings.TrimSpace(abortErr) != "" {
			result["abort_error"] = strings.TrimSpace(abortErr)
		}
	}
}

func bumpPatchVersion(ver string) (string, error) {
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid version %q (expected x.y.z)", ver)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major version in %q", ver)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor version in %q", ver)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version in %q", ver)
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), nil
}

func nextAvailableTagVersion(cfg config.FlowConfig, start string) (string, error) {
	candidate := start
	for i := 0; i < 1000; i++ {
		tagName := cfg.TagPrefix + candidate
		if !git.TagExists(tagName) {
			return candidate, nil
		}
		next, err := bumpPatchVersion(candidate)
		if err != nil {
			return "", err
		}
		candidate = next
	}
	return "", fmt.Errorf("unable to find available version after %s", start)
}

func autoBumpFlowVersionIfTagExists(cfg config.FlowConfig, btype, name string, result map[string]any) (string, error) {
	tagName := cfg.TagPrefix + name
	if !git.TagExists(tagName) {
		return name, nil
	}

	next, err := nextAvailableTagVersion(cfg, name)
	if err != nil {
		return "", err
	}
	if next == name {
		return name, nil
	}

	if cfg.VersionFile == "" {
		return "", fmt.Errorf("cannot auto-bump %s: version_file is not configured", btype)
	}

	oldBranch := btype + "/" + name
	newBranch := btype + "/" + next
	if git.BranchExists(newBranch) {
		return "", fmt.Errorf("cannot auto-bump: target branch %s already exists", newBranch)
	}

	output.Infof("  %s⚠ Tag %s already exists; auto-bumping %s to %s.%s", output.Yellow, tagName, btype, next, output.Reset)
	version.WriteVersionFile(cfg, next)

	if err := git.Exec("branch", "-m", oldBranch, newBranch); err != nil {
		return "", fmt.Errorf("failed to rename %s to %s: %w", oldBranch, newBranch, err)
	}

	result["auto_bumped"] = true
	result["auto_bumped_from"] = name
	result["auto_bumped_to"] = next
	result["auto_bump_reason"] = fmt.Sprintf("tag %s already exists", tagName)
	return next, nil
}

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

	if err := git.Exec("branch", "-d", branchName); err != nil {
		output.Infof("  %s%s%s", output.Yellow, mergedBranchDeleteWarning(branchName, err), output.Reset)
	}
	output.Infof("  %s✓ %s/%s → %s%s", output.Green, btype, name, cfg.DevelopBranch, output.Reset)
	return nil
}

func finishRelease(cfg config.FlowConfig, ver string) error {
	branchName := "release/" + ver
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}
	tagName := cfg.TagPrefix + ver
	if git.TagExists(tagName) {
		return fmt.Errorf("tag %s already exists", tagName)
	}

	if err := git.Exec("checkout", cfg.MainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.MainBranch, err)
	}

	mergeMsg := fmt.Sprintf("Merge release '%s' into %s", ver, cfg.MainBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", mergeMsg); err != nil {
		return fmt.Errorf("merge of %s into %s failed: %w", branchName, cfg.MainBranch, err)
	}

	if err := git.Exec("tag", "-a", tagName, "-m", fmt.Sprintf("Release %s", ver)); err != nil {
		return fmt.Errorf("tag creation failed for %s: %w", tagName, err)
	}

	if err := git.Exec("checkout", cfg.DevelopBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.DevelopBranch, err)
	}

	backmergeMsg := fmt.Sprintf("Merge tag '%s' back into %s", tagName, cfg.DevelopBranch)
	if err := git.Exec("merge", "--no-ff", tagName, "-m", backmergeMsg); err != nil {
		return fmt.Errorf("back-merge of %s into %s failed: %w", tagName, cfg.DevelopBranch, err)
	}

	if err := git.Exec("branch", "-d", branchName); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branchName, err)
	}
	output.Infof("  %s✓ release/%s → %s (tagged %s) → %s%s",
		output.Green, ver, cfg.MainBranch, tagName, cfg.DevelopBranch, output.Reset)
	return nil
}

func finishHotfix(cfg config.FlowConfig, ver string) error {
	branchName := "hotfix/" + ver
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}
	tagName := cfg.TagPrefix + ver
	if git.TagExists(tagName) {
		return fmt.Errorf("tag %s already exists", tagName)
	}

	if err := git.Exec("checkout", cfg.MainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.MainBranch, err)
	}

	mergeMsg := fmt.Sprintf("Merge hotfix '%s' into %s", ver, cfg.MainBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", mergeMsg); err != nil {
		return fmt.Errorf("merge of %s into %s failed: %w", branchName, cfg.MainBranch, err)
	}

	if err := git.Exec("tag", "-a", tagName, "-m", fmt.Sprintf("Hotfix %s", ver)); err != nil {
		return fmt.Errorf("tag creation failed for %s: %w", tagName, err)
	}

	releases := git.ActiveReleaseBranches()
	backTarget := cfg.DevelopBranch
	if len(releases) > 0 {
		backTarget = releases[0]
	}

	if err := git.Exec("checkout", backTarget); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", backTarget, err)
	}

	backmergeMsg := fmt.Sprintf("Merge hotfix '%s' into %s", ver, backTarget)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", backmergeMsg); err != nil {
		return fmt.Errorf("back-merge of hotfix into %s failed: %w", backTarget, err)
	}

	if err := git.Exec("branch", "-d", branchName); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branchName, err)
	}
	output.Infof("  %s✓ hotfix/%s → %s (tagged %s) → %s%s",
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

	// Dirty check must run before any side-effects (release notes commit, etc.)
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
		output.Infof("  %s✗ Uncommitted changes (%s) — commit or stash first.%s",
			output.Red, detail, output.Reset)
		result["result"] = "error"
		result["error"] = fmt.Sprintf("dirty working tree: %s", detail)
		result["dirty"] = map[string]int{
			"staged": wt.Staged, "modified": wt.Unstaged, "untracked": wt.Untracked,
		}
		return 1, result
	}

	if wt.Untracked > 0 {
		result["warning_untracked"] = wt.Untracked
	}

	if btype == "release" || btype == "hotfix" {
		fileVer := git.FlowVersion(version.ReadVersion(cfg))
		if fileVer != "" && fileVer != "0.0.0" && fileVer != name {
			result["result"] = "error"
			result["version_file"] = cfg.VersionFile
			result["version_from_file"] = fileVer
			result["error"] = fmt.Sprintf("version mismatch: branch %s/%s but %s=%s", btype, name, cfg.VersionFile, fileVer)
			return 1, result
		}

		updatedName, err := autoBumpFlowVersionIfTagExists(cfg, btype, name, result)
		if err != nil {
			result["result"] = "error"
			result["error"] = err.Error()
			return 1, result
		}
		if updatedName != name {
			name = updatedName
			result["name"] = name
		}
	}

	if btype == "release" || btype == "hotfix" {
		meta := WriteReleaseNotes(cfg, "")
		if meta != nil {
			result["release_notes"] = meta
			if err := git.Exec("add", "RELEASE_NOTES.md"); err != nil {
				result["result"] = "error"
				result["error"] = "failed to stage release notes: " + err.Error()
				return 1, result
			}
			if git.HasStagedChanges() {
				if err := git.Exec("commit", "-m", fmt.Sprintf("docs: release notes for %s %s", btype, name)); err != nil {
					result["result"] = "error"
					result["error"] = "failed to commit release notes: " + err.Error()
					return 1, result
				}
			}
		}
	}

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
			result["conflicts"] = conflicts
			result["needs_human"] = true
			addMergeAbortDiagnostics(result)
			return 2, result
		}
		return 1, result
	}

	cur := git.CurrentBranch()
	if cur != cfg.DevelopBranch {
		_ = git.Exec("checkout", cfg.DevelopBranch)
	}

	result["result"] = "ok"
	result["landed_on"] = cfg.DevelopBranch
	return 0, result
}
