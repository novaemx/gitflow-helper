package flow

import (
	"fmt"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/luis-lozano/gitflow-helper/internal/version"
)

func startFlowBranch(cfg config.FlowConfig, branchType, name string) error {
	parent := cfg.DevelopBranch
	if branchType == "hotfix" {
		parent = cfg.MainBranch
	}

	branchName := branchType + "/" + name
	cur := git.CurrentBranch()

	if cur != parent {
		output.Infof("  Switching to %s before creating %s...", parent, branchName)
		if err := git.Exec("checkout", parent); err != nil {
			return fmt.Errorf("failed to checkout %s: %w", parent, err)
		}
	}

	if err := git.Exec("checkout", "-b", branchName); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}
	return nil
}

func StartFeature(cfg config.FlowConfig, name string) error {
	if name == "" {
		return fmt.Errorf("feature name required")
	}
	return startFlowBranch(cfg, "feature", name)
}

func StartBugfix(cfg config.FlowConfig, name string) error {
	if name == "" {
		return fmt.Errorf("bugfix name required")
	}
	return startFlowBranch(cfg, "bugfix", name)
}

func StartRelease(cfg config.FlowConfig, ver string) error {
	if ver == "" {
		return fmt.Errorf("release version required")
	}

	if cfg.BumpCommand != "" {
		version.RunBumpCommand(cfg, "minor")
		version.RunBuildBumpCommand(cfg)
		if cfg.VersionFile != "" {
			_ = git.Exec("add", cfg.VersionFile)
		}
		_ = git.Exec("commit", "-m", fmt.Sprintf("chore: bump version to %s", ver))
	}

	return startFlowBranch(cfg, "release", git.FlowVersion(ver))
}

func StartHotfix(cfg config.FlowConfig, ver string) error {
	if ver == "" {
		return fmt.Errorf("hotfix version required")
	}

	if cfg.BumpCommand != "" {
		version.RunBumpCommand(cfg, "patch")
		version.RunBuildBumpCommand(cfg)
		if cfg.VersionFile != "" {
			_ = git.Exec("add", cfg.VersionFile)
		}
		_ = git.Exec("commit", "-m", fmt.Sprintf("chore: bump version to %s (hotfix)", ver))
	}

	return startFlowBranch(cfg, "hotfix", git.FlowVersion(ver))
}

func StartBranch(cfg config.FlowConfig, branchType, name string) (int, map[string]any) {
	validTypes := []string{"feature", "bugfix", "release", "hotfix"}
	valid := false
	for _, t := range validTypes {
		if branchType == t {
			valid = true
			break
		}
	}
	if !valid {
		return 1, map[string]any{"action": "start", "error": fmt.Sprintf("invalid type: %s", branchType)}
	}

	branch := git.CurrentBranch()
	expectedParent := cfg.DevelopBranch
	if branchType == "hotfix" {
		expectedParent = cfg.MainBranch
	}

	result := map[string]any{
		"action": "start",
		"type":   branchType,
		"name":   name,
	}

	if branch != expectedParent {
		output.Infof("  %sWarning:%s '%s' branches should start from '%s' (currently on '%s').",
			output.Yellow, output.Reset, branchType, expectedParent, branch)
		result["hint"] = "switch to " + expectedParent + " first"
	}

	if branchType == "feature" || branchType == "bugfix" {
		releases := git.ActiveReleaseBranches()
		if len(releases) > 0 {
			output.Infof("  %sWarning:%s Release branch '%s' is in progress.",
				output.Yellow, output.Reset, releases[0])
			result["warning"] = "release_in_progress"
		}
	}

	var err error
	switch branchType {
	case "feature":
		err = StartFeature(cfg, name)
	case "bugfix":
		err = StartBugfix(cfg, name)
	case "release":
		err = StartRelease(cfg, name)
	case "hotfix":
		err = StartHotfix(cfg, name)
	}

	if err != nil {
		result["result"] = "error"
		result["error"] = err.Error()
		return 1, result
	}

	branchName := branchType + "/" + name
	output.Infof("\n  %sBranch '%s' created.%s", output.Green, branchName, output.Reset)
	result["branch"] = branchName
	result["result"] = "ok"
	return 0, result
}

func InitGitFlow(cfg config.FlowConfig) (bool, string) {
	if git.IsGitFlowInitialized() {
		return true, "already_initialized"
	}

	allLocal := git.AllLocalBranches()
	localSet := make(map[string]bool)
	for _, b := range allLocal {
		localSet[b] = true
	}

	code, _, _ := git.ExecResult("rev-parse", "HEAD")
	if code != 0 {
		output.Infof("  %sCreating initial commit...%s", output.Dim, output.Reset)
		_ = git.Exec("checkout", "-b", cfg.MainBranch)
		_ = git.Exec("commit", "--allow-empty", "-m", "chore: initial commit")
		localSet[cfg.MainBranch] = true
	}

	if !localSet[cfg.MainBranch] {
		cur := git.CurrentBranch()
		if cur == "master" && cfg.MainBranch == "main" {
			_ = git.Exec("branch", "-m", "master", "main")
		} else {
			_ = git.Exec("branch", cfg.MainBranch)
		}
	}

	if !localSet[cfg.DevelopBranch] {
		output.Infof("  %sCreating %s branch from %s...%s", output.Dim, cfg.DevelopBranch, cfg.MainBranch, output.Reset)
		_ = git.Exec("branch", cfg.DevelopBranch, cfg.MainBranch)
	}

	ok := git.IsGitFlowInitialized()
	if ok {
		output.Infof("  %sGitflow structure initialized (main + develop).%s", output.Green, output.Reset)
		return true, "ok"
	}
	output.Infof("  %sGitflow initialization failed.%s", output.Red, output.Reset)
	return false, "error"
}

func EnsureGitFlowReady(cfg config.FlowConfig) (bool, string) {
	gitVer := git.ExecQuiet("--version")
	if gitVer == "" {
		return false, "git not found in PATH"
	}

	if !git.IsGitRepo() {
		return false, "not a git repository"
	}

	if !git.IsGitFlowInitialized() {
		ok, msg := InitGitFlow(cfg)
		if !ok {
			return false, msg
		}
	}

	return true, "ready"
}

func ListSwitchableBranches(allLocal []string, cfg config.FlowConfig, current string) []string {
	var result []string
	permanent := []string{cfg.MainBranch, cfg.DevelopBranch}
	for _, b := range permanent {
		if b != current {
			for _, local := range allLocal {
				if local == b {
					result = append(result, b)
					break
				}
			}
		}
	}
	prefixes := []string{"feature/", "bugfix/", "release/", "hotfix/"}
	for _, prefix := range prefixes {
		for _, b := range allLocal {
			if strings.HasPrefix(b, prefix) && b != current {
				result = append(result, b)
			}
		}
	}
	return result
}
