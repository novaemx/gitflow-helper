package flow

import (
	"fmt"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
)

// finishFeatureOrBugfix merges the branch into develop and deletes it.
func finishFeatureOrBugfix(cfg config.FlowConfig, btype, name string) error {
	branchName := btype + "/" + name
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}

	if err := git.Run("git checkout " + cfg.DevelopBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.DevelopBranch, err)
	}

	if err := git.Run(fmt.Sprintf(`git merge --no-ff %s -m "Merge %s '%s' into %s"`, branchName, btype, name, cfg.DevelopBranch)); err != nil {
		return fmt.Errorf("merge of %s failed (conflicts?): %w", branchName, err)
	}

	_ = git.Run("git branch -d " + branchName)
	output.Infof("  %s%s '%s' merged into %s and branch deleted.%s", output.Green, btype, name, cfg.DevelopBranch, output.Reset)
	return nil
}

// finishRelease merges release into main (tagged) and back into develop, then deletes.
func finishRelease(cfg config.FlowConfig, ver string) error {
	branchName := "release/" + ver
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}
	tagName := cfg.TagPrefix + ver

	// Merge into main
	if err := git.Run("git checkout " + cfg.MainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.MainBranch, err)
	}

	if err := git.Run(fmt.Sprintf(`git merge --no-ff %s -m "Merge release '%s' into %s"`, branchName, ver, cfg.MainBranch)); err != nil {
		return fmt.Errorf("merge of %s into %s failed: %w", branchName, cfg.MainBranch, err)
	}

	// Tag the release on main
	_ = git.Run(fmt.Sprintf(`git tag -a %s -m "Release %s"`, tagName, ver))

	// Back-merge into develop
	if err := git.Run("git checkout " + cfg.DevelopBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.DevelopBranch, err)
	}

	if err := git.Run(fmt.Sprintf(`git merge --no-ff %s -m "Merge tag '%s' back into %s"`, tagName, tagName, cfg.DevelopBranch)); err != nil {
		return fmt.Errorf("back-merge of %s into %s failed: %w", tagName, cfg.DevelopBranch, err)
	}

	_ = git.Run("git branch -d " + branchName)
	output.Infof("  %sRelease %s merged into %s (tagged %s) and back-merged into %s.%s",
		output.Green, ver, cfg.MainBranch, tagName, cfg.DevelopBranch, output.Reset)
	return nil
}

// finishHotfix merges hotfix into main (tagged), back into develop (or active release), then deletes.
func finishHotfix(cfg config.FlowConfig, ver string) error {
	branchName := "hotfix/" + ver
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}
	tagName := cfg.TagPrefix + ver

	// Merge into main
	if err := git.Run("git checkout " + cfg.MainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.MainBranch, err)
	}

	if err := git.Run(fmt.Sprintf(`git merge --no-ff %s -m "Merge hotfix '%s' into %s"`, branchName, ver, cfg.MainBranch)); err != nil {
		return fmt.Errorf("merge of %s into %s failed: %w", branchName, cfg.MainBranch, err)
	}

	_ = git.Run(fmt.Sprintf(`git tag -a %s -m "Hotfix %s"`, tagName, ver))

	// Merge into develop (or active release branch per nvie model)
	releases := git.ActiveReleaseBranches()
	backTarget := cfg.DevelopBranch
	if len(releases) > 0 {
		backTarget = releases[0]
		output.Infof("  %sNote:%s Hotfix will merge into active release '%s' (nvie rule).",
			output.Yellow, output.Reset, backTarget)
	}

	if err := git.Run("git checkout " + backTarget); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", backTarget, err)
	}

	if err := git.Run(fmt.Sprintf(`git merge --no-ff %s -m "Merge hotfix '%s' into %s"`, branchName, ver, backTarget)); err != nil {
		return fmt.Errorf("back-merge of hotfix into %s failed: %w", backTarget, err)
	}

	_ = git.Run("git branch -d " + branchName)
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
		conflicts := git.RunLines("git diff --name-only --diff-filter=U")
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
	if btype == "release" {
		meta := WriteReleaseNotes(cfg, "")
		if meta != nil {
			result["release_notes"] = meta
		}
	}

	return 0, result
}

func FinishFeature(cfg config.FlowConfig) error {
	branch := git.CurrentBranch()
	name := strings.TrimPrefix(branch, "feature/")
	if name == "" || name == branch {
		return fmt.Errorf("not on a feature branch")
	}
	return finishFeatureOrBugfix(cfg, "feature", name)
}

func FinishBugfix(cfg config.FlowConfig) error {
	branch := git.CurrentBranch()
	name := strings.TrimPrefix(branch, "bugfix/")
	if name == "" || name == branch {
		return fmt.Errorf("not on a bugfix branch")
	}
	return finishFeatureOrBugfix(cfg, "bugfix", name)
}

func FinishRelease(cfg config.FlowConfig) error {
	branch := git.CurrentBranch()
	ver := strings.TrimPrefix(branch, "release/")
	ver = strings.TrimPrefix(ver, "v")
	if ver == "" || ver == branch {
		return fmt.Errorf("not on a release branch")
	}
	err := finishRelease(cfg, ver)
	if err != nil {
		return err
	}
	meta := WriteReleaseNotes(cfg, "")
	if meta != nil {
		PrintReleaseNotes(meta)
	}
	return nil
}

func FinishHotfix(cfg config.FlowConfig) error {
	branch := git.CurrentBranch()
	ver := strings.TrimPrefix(branch, "hotfix/")
	ver = strings.TrimPrefix(ver, "v")
	if ver == "" || ver == branch {
		return fmt.Errorf("not on a hotfix branch")
	}
	return finishHotfix(cfg, ver)
}
