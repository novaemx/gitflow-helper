package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/debug"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/novaemx/gitflow-helper/internal/version"
)

var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
var semverExtractPattern = regexp.MustCompile(`\d+\.\d+\.\d+`)

var (
	tagExistsStart    = git.TagExists
	branchExistsStart = git.BranchExists
	execLinesStart    = git.ExecLines
	latestTagStart    = git.LatestTag
	workingTreeStatusStart = git.WorkingTreeStatus
	stashSaveStart         = git.StashSave
	stashPopStart          = git.StashPop
	execResultStart        = git.ExecResult
)

var (
	startFeatureFn = StartFeature
	startBugfixFn  = StartBugfix
	startReleaseFn = StartRelease
	startHotfixFn  = StartHotfix
)

type startResolutionDetails struct {
	Requested   string
	Resolved    string
	BaseVersion string
	Source      string
	VersionFile string
	LatestTag   string
	Skipped     []string
	Diagnostics []string
	Hint        string
}

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

func nextAvailableStartVersion(cfg config.FlowConfig, branchType, start string) (string, []string, error) {
	candidate := start
	var skipped []string
	for i := 0; i < 1000; i++ {
		tagName := cfg.TagPrefix + candidate
		branchName := branchType + "/" + candidate
		if !tagExistsStart(tagName) && !branchExistsStart(branchName) {
			return candidate, skipped, nil
		}
		if tagExistsStart(tagName) {
			skipped = append(skipped, fmt.Sprintf("%s (tag %s already exists)", candidate, tagName))
		} else {
			skipped = append(skipped, fmt.Sprintf("%s (branch %s already exists)", candidate, branchName))
		}
		next, err := bumpPatch(candidate)
		if err != nil {
			return "", skipped, err
		}
		candidate = next
	}
	return "", skipped, fmt.Errorf("unable to find available version after %s", start)
}

func extractSemver(value string) string {
	return semverExtractPattern.FindString(strings.TrimSpace(value))
}

func latestSemverTag() string {
	for _, tag := range execLinesStart("tag", "--sort=-version:refname") {
		if semver := extractSemver(tag); semver != "" {
			return semver
		}
	}
	return ""
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

func resolveStartVersion(cfg config.FlowConfig, branchType, requested string) (string, startResolutionDetails, error) {
	resolved := git.FlowVersion(strings.TrimSpace(requested))
	details := startResolutionDetails{
		Requested:   requested,
		Resolved:    resolved,
		VersionFile: cfg.VersionFile,
		LatestTag:   latestTagStart(),
	}

	if branchType != "release" && branchType != "hotfix" {
		return resolved, details, nil
	}

	if resolved == "" || strings.EqualFold(resolved, "auto") {
		versionFileValue := git.FlowVersion(version.ReadVersion(cfg))
		if semverPattern.MatchString(versionFileValue) {
			details.BaseVersion = versionFileValue
			details.Source = "version_file"
		} else {
			if strings.TrimSpace(cfg.VersionFile) != "" {
				details.Diagnostics = append(details.Diagnostics, fmt.Sprintf("version_file=%s value=%s", cfg.VersionFile, strings.TrimSpace(versionFileValue)))
			} else {
				details.Diagnostics = append(details.Diagnostics, "version_file=unset")
			}
		}

		if details.BaseVersion == "" {
			if semverTag := latestSemverTag(); semverTag != "" {
				details.BaseVersion = semverTag
				details.Source = "latest_semver_tag"
			} else {
				details.Diagnostics = append(details.Diagnostics, fmt.Sprintf("latest_tag=%s", details.LatestTag))
			}
		}

		if details.BaseVersion == "" {
			details.Hint = fmt.Sprintf("Pass an explicit %s version like 'gitflow start %s 1.2.3' or configure a VERSION file / semver tag.", branchType, branchType)
			return "", details, fmt.Errorf("could not auto-detect %s version", branchType)
		}

		debug.Logf("Auto-resolving %s version from %s=%s", branchType, details.Source, details.BaseVersion)
		next, skipped, err := nextAvailableStartVersion(cfg, branchType, details.BaseVersion)
		if err != nil {
			return "", details, err
		}
		details.Skipped = skipped
		resolved = next
	}

	if !semverPattern.MatchString(resolved) {
		details.Hint = fmt.Sprintf("Use a semantic version like 'gitflow start %s 1.2.3'.", branchType)
		return "", details, fmt.Errorf("invalid %s version %q (expected x.y.z)", branchType, resolved)
	}

	if details.Source == "" {
		details.Source = "explicit"
		details.BaseVersion = resolved
	}
	details.Resolved = resolved
	if len(details.Skipped) > 0 && details.Hint == "" {
		details.Hint = fmt.Sprintf("Reused the next available %s version after detecting existing tags or branches.", branchType)
	}
	return resolved, details, nil
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

	resolvedName, details, err := resolveStartVersion(cfg, branchType, name)
	if err != nil {
		result["result"] = "error"
		result["error"] = err.Error()
		if details.Hint != "" {
			result["hint"] = details.Hint
		}
		if len(details.Diagnostics) > 0 {
			result["diagnostics"] = details.Diagnostics
		}
		return 1, result
	}
	if resolvedName != "" {
		name = resolvedName
		result["name"] = name
	}
	if details.Source != "" && details.Source != "explicit" {
		result["resolution_source"] = details.Source
	}
	if details.BaseVersion != "" && details.BaseVersion != name {
		result["base_version"] = details.BaseVersion
	}
	if len(details.Skipped) > 0 {
		result["skipped_versions"] = details.Skipped
	}
	if strings.EqualFold(originalRequested, "auto") || originalRequested == "" {
		result["resolved_name"] = name
	}

	if branchExistsStart(branchType + "/" + name) {
		result["result"] = "error"
		result["error"] = fmt.Sprintf("branch %s/%s already exists", branchType, name)
		result["hint"] = fmt.Sprintf("Switch to the existing %s/%s branch or finish/delete it before starting a new one.", branchType, name)
		return 1, result
	}

	if branchType == "release" || branchType == "hotfix" {
		tagName := cfg.TagPrefix + name
		if tagExistsStart(tagName) {
			result["result"] = "error"
			result["error"] = fmt.Sprintf("tag %s already exists", tagName)
			result["hint"] = fmt.Sprintf("Use 'auto' or pass the next available semantic version after %s.", name)
			return 1, result
		}
	}

	wt := workingTreeStatusStart()
	stashed := false
	if wt.Staged > 0 || wt.Unstaged > 0 {
		if err := stashSaveStart("gitflow: auto-stash before " + branchType + "/" + name); err != nil {
			result["result"] = "error"
			result["error"] = "failed to stash changes: " + err.Error()
			return 1, result
		}
		stashed = true
	}

	var startErr error
	switch branchType {
	case "feature":
		startErr = startFeatureFn(cfg, name)
	case "bugfix":
		startErr = startBugfixFn(cfg, name)
	case "release":
		startErr = startReleaseFn(cfg, name)
	case "hotfix":
		startErr = startHotfixFn(cfg, name)
	}

	if startErr != nil {
		if stashed {
			_ = stashPopStart()
		}
		result["result"] = "error"
		result["error"] = startErr.Error()
		return 1, result
	}

	if stashed {
		popCode, _, popErr := execResultStart("stash", "pop")
		if popCode != 0 {
			result["result"] = "conflict"
			result["needs_human"] = true
			result["stash_restore"] = "conflict"
			if strings.TrimSpace(popErr) != "" {
				result["error"] = "failed to restore stashed changes: " + strings.TrimSpace(popErr)
			} else {
				result["error"] = "failed to restore stashed changes: resolve stash conflicts manually"
			}
			return 2, result
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
