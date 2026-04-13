package flow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/luis-lozano/gitflow-helper/internal/version"
)

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func startFlowBranch(cfg config.FlowConfig, branchType, name string) error {
	parent := cfg.DevelopBranch
	if branchType == "hotfix" {
		parent = cfg.MainBranch
	}

	branchName := branchType + "/" + name
	cur := git.CurrentBranch()

	if cur != parent {
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
	return startFlowBranch(cfg, "release", git.FlowVersion(ver))
}

func StartHotfix(cfg config.FlowConfig, ver string) error {
	if ver == "" {
		return fmt.Errorf("hotfix version required")
	}
	return startFlowBranch(cfg, "hotfix", git.FlowVersion(ver))
}

func resolveStartVersion(cfg config.FlowConfig, branchType, requested string) (string, error) {
	resolved := git.FlowVersion(strings.TrimSpace(requested))

	if branchType != "release" && branchType != "hotfix" {
		return resolved, nil
	}

	if resolved == "" || strings.EqualFold(resolved, "auto") {
		auto := git.FlowVersion(version.ReadVersion(cfg))
		if auto == "" || auto == "0.0.0" {
			return "", fmt.Errorf("could not auto-detect %s version from %s", branchType, cfg.VersionFile)
		}
		resolved = auto
	}

	if !semverPattern.MatchString(resolved) {
		return "", fmt.Errorf("invalid %s version %q (expected x.y.z)", branchType, resolved)
	}

	return resolved, nil
}

func bumpVersionOnBranch(cfg config.FlowConfig, branchType, ver string) {
	if branchType != "release" && branchType != "hotfix" {
		return
	}
	part := "minor"
	if branchType == "hotfix" {
		part = "patch"
	}
	if cfg.BumpCommand != "" {
		version.RunBumpCommand(cfg, part)
		version.RunBuildBumpCommand(cfg)
	} else if cfg.VersionFile != "" {
		version.WriteVersionFile(cfg, ver)
	}
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

	result := map[string]any{
		"action": "start",
		"type":   branchType,
		"name":   name,
	}
	originalRequested := strings.TrimSpace(name)

	resolvedName, err := resolveStartVersion(cfg, branchType, name)
	if err != nil {
		result["result"] = "error"
		result["error"] = err.Error()
		return 1, result
	}
	if resolvedName != "" {
		name = resolvedName
		result["name"] = name
	}
	if strings.EqualFold(originalRequested, "auto") || originalRequested == "" {
		result["resolved_name"] = name
	}

	if branchType == "release" || branchType == "hotfix" {
		tagName := cfg.TagPrefix + name
		if git.TagExists(tagName) {
			result["result"] = "error"
			result["error"] = fmt.Sprintf("tag %s already exists", tagName)
			return 1, result
		}
	}

	wt := git.WorkingTreeStatus()
	stashed := false
	if wt.Staged > 0 || wt.Unstaged > 0 {
		if err := git.StashSave("gitflow: auto-stash before " + branchType + "/" + name); err != nil {
			result["result"] = "error"
			result["error"] = "failed to stash changes: " + err.Error()
			return 1, result
		}
		stashed = true
	}

	var startErr error
	switch branchType {
	case "feature":
		startErr = StartFeature(cfg, name)
	case "bugfix":
		startErr = StartBugfix(cfg, name)
	case "release":
		startErr = StartRelease(cfg, name)
	case "hotfix":
		startErr = StartHotfix(cfg, name)
	}

	if startErr != nil {
		if stashed {
			_ = git.StashPop()
		}
		result["result"] = "error"
		result["error"] = startErr.Error()
		return 1, result
	}

	if stashed {
		popCode, _, _ := git.ExecResult("stash", "pop")
		if popCode != 0 {
			_ = git.Exec("checkout", "--theirs", ".")
			_ = git.Exec("add", ".")
			result["stash_restore"] = "resolved"
		} else {
			result["stash_restore"] = "ok"
		}
	}

	bumpVersionOnBranch(cfg, branchType, name)

	branchName := branchType + "/" + name
	output.Infof("  %s✓ Branch '%s' created.%s", output.Green, branchName, output.Reset)
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
