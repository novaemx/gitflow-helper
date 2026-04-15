package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/novaemx/gitflow-helper/internal/version"
)

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func bumpPatch(ver string) (string, error) {
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

func nextAvailableStartVersion(cfg config.FlowConfig, start string) (string, error) {
	candidate := start
	for i := 0; i < 1000; i++ {
		if !git.TagExists(cfg.TagPrefix + candidate) {
			return candidate, nil
		}
		next, err := bumpPatch(candidate)
		if err != nil {
			return "", err
		}
		candidate = next
	}
	return "", fmt.Errorf("unable to find available version after %s", start)
}

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

		next, err := nextAvailableStartVersion(cfg, resolved)
		if err != nil {
			return "", err
		}
		resolved = next
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

	// ── Step 1: ensure at least one commit exists on main ───────────────────
	code, _, _ := git.ExecResult("rev-parse", "HEAD")
	if code != 0 {
		// Brand-new empty repo — create main and the root commit silently.
		_ = git.ExecSilent("checkout", "-b", cfg.MainBranch)
		if err := git.ExecSilent("commit", "--allow-empty", "-m", "chore: initial commit"); err != nil {
			output.Infof("  %s✗ Failed to create initial commit on %s%s", output.Red, cfg.MainBranch, output.Reset)
			return false, "error"
		}
		localSet[cfg.MainBranch] = true
		output.Infof("  %s✓ %s%s — initial commit", output.Green, cfg.MainBranch, output.Reset)
	}

	// ── Step 2: ensure main branch exists with the right name ───────────────
	if !localSet[cfg.MainBranch] {
		cur := git.CurrentBranch()
		if cur == "master" && cfg.MainBranch == "main" {
			_ = git.ExecSilent("branch", "-m", "master", "main")
		} else {
			_ = git.ExecSilent("branch", cfg.MainBranch)
		}
		output.Infof("  %s✓ %s%s — renamed/created", output.Green, cfg.MainBranch, output.Reset)
	}

	// ── Step 3: create develop from main ────────────────────────────────────
	if !localSet[cfg.DevelopBranch] {
		if err := git.ExecSilent("branch", cfg.DevelopBranch, cfg.MainBranch); err != nil {
			output.Infof("  %s✗ Failed to create %s branch%s", output.Red, cfg.DevelopBranch, output.Reset)
			return false, "error"
		}
		output.Infof("  %s✓ %s%s — created from %s", output.Green, cfg.DevelopBranch, output.Reset, cfg.MainBranch)
	}

	// ── Step 4: switch to develop — all further changes land here ───────────
	if err := git.ExecSilent("checkout", cfg.DevelopBranch); err != nil {
		output.Infof("  %s✗ Failed to switch to %s%s", output.Red, cfg.DevelopBranch, output.Reset)
		return false, "error"
	}
	output.Infof("  %s✓ switched to %s%s", output.Green, cfg.DevelopBranch, output.Reset)

	// ── Step 5: create VERSION file if absent ───────────────────────────────
	verPath := filepath.Join(cfg.ProjectRoot, "VERSION")
	if _, err := os.Stat(verPath); os.IsNotExist(err) {
		initVer := "0.0.1"
		if writeErr := os.WriteFile(verPath, []byte(initVer+"\n"), 0644); writeErr == nil {
			_ = git.ExecSilent("add", "VERSION")
			_ = git.ExecSilent("commit", "-m", fmt.Sprintf("chore: initial version %s", initVer))
			output.Infof("  %s✓ VERSION%s — %s", output.Green, output.Reset, initVer)
		}
	}

	if !git.IsGitFlowInitialized() {
		output.Infof("  %s✗ Gitflow initialization failed%s", output.Red, output.Reset)
		return false, "error"
	}
	return true, "ok"
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
