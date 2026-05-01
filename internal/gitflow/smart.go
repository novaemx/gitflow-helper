package gitflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/flow"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
)

// PreMergeReport describes the divergence risk before finishing a branch.
type PreMergeReport struct {
	Branch       string   `json:"branch"`
	BranchType   string   `json:"branch_type"`
	Parent       string   `json:"parent"`
	BehindParent int      `json:"behind_parent"`
	OverlapFiles []string `json:"overlap_files,omitempty"`
	RiskLevel    string   `json:"risk_level"`
	AutoSynced   bool     `json:"auto_synced"`
}

// PreMergeCheck detects whether the current flow branch has diverged from
// its parent. Returns a report with conflict risk assessment. If autoSync
// is true and the branch is behind, it attempts to sync automatically
// before the caller runs finish.
func (gf *Logic) PreMergeCheck(autoSync bool) (*PreMergeReport, error) {
	branch := git.CurrentBranch()
	btype := git.BranchTypeOf(branch)

	if btype != "feature" && btype != "bugfix" && btype != "release" && btype != "hotfix" {
		return nil, fmt.Errorf("not on a flow branch (current: %s)", branch)
	}

	parent := gf.Config.DevelopBranch
	if btype == "hotfix" {
		parent = gf.Config.MainBranch
	}

	behindStr := git.ExecQuiet("rev-list", "--count", branch+".."+parent)
	behind, err := strconv.Atoi(strings.TrimSpace(behindStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse divergence count %q: %w", strings.TrimSpace(behindStr), err)
	}

	report := &PreMergeReport{
		Branch:       branch,
		BranchType:   btype,
		Parent:       parent,
		BehindParent: behind,
	}

	if behind == 0 {
		report.RiskLevel = "low"
		return report, nil
	}

	branchFiles := git.ExecLines("diff", "--name-only", parent+".."+branch)
	parentFiles := git.ExecLines("diff", "--name-only", branch+".."+parent)

	branchSet := make(map[string]bool)
	for _, f := range branchFiles {
		branchSet[f] = true
	}
	for _, f := range parentFiles {
		if branchSet[f] {
			report.OverlapFiles = append(report.OverlapFiles, f)
		}
	}

	switch {
	case len(report.OverlapFiles) > 5:
		report.RiskLevel = "high"
	case len(report.OverlapFiles) > 0:
		report.RiskLevel = "medium"
	default:
		report.RiskLevel = "low"
	}

	if autoSync && behind > 0 {
		output.Infof("  %sAuto-syncing: %s is %d commit(s) behind %s...%s",
			output.Yellow, branch, behind, parent, output.Reset)
		code, _ := flow.Sync(gf.Config)
		if code == 0 {
			report.AutoSynced = true
			report.BehindParent = 0
		} else {
			return report, fmt.Errorf("auto-sync failed with conflicts; resolve manually before finishing")
		}
	}

	return report, nil
}

// SmartFinish wraps Finish() with a PreMergeCheck followed by a rebase-first
// strategy. If the branch is behind its parent it rebases (via Sync) to resolve
// conflicts incrementally. After a successful finish the remote tracking branch
// is deleted automatically when a remote is configured.
func (gf *Logic) SmartFinish(name string) (int, map[string]any) {
	report, err := gf.PreMergeCheck(true)
	if err != nil {
		return 1, map[string]any{
			"action":      "finish",
			"error":       err.Error(),
			"premerge":    report,
			"needs_human": true,
		}
	}

	code, result := gf.Finish(name, flow.FinishOptions{
		Rebase:       true,
		DeleteRemote: true,
	})
	if report != nil {
		result["premerge"] = report
	}
	return code, result
}

// SafeHotfixFinish wraps hotfix finish with additional nvie-model safety:
//   - Warns if a release branch exists (hotfix goes to release, not develop)
//   - Verifies backmerge after finish (main must not be ahead of develop)
func (gf *Logic) SafeHotfixFinish(name string) (int, map[string]any) {
	branch := git.CurrentBranch()
	btype := git.BranchTypeOf(branch)
	if btype != "hotfix" {
		return 1, map[string]any{"action": "finish", "error": "not on a hotfix branch"}
	}

	releases := git.ActiveReleaseBranches()
	warnings := []string{}
	if len(releases) > 0 {
		msg := fmt.Sprintf("Active release '%s' detected: hotfix will merge into release branch instead of develop (nvie rule)", releases[0])
		warnings = append(warnings, msg)
		output.Infof("  %s⚠ %s%s", output.Yellow, msg, output.Reset)
	}

	code, result := gf.SmartFinish(name)

	gf.Refresh()
	if gf.State.MainAheadOfDevelop > 0 {
		msg := fmt.Sprintf("main is still %d commit(s) ahead of develop after hotfix -- backmerge needed", gf.State.MainAheadOfDevelop)
		warnings = append(warnings, msg)
		result["action_required"] = "backmerge"
		output.Infof("  %s⚠ %s%s", output.Red, msg, output.Reset)
	}

	if len(warnings) > 0 {
		result["warnings"] = warnings
	}
	return code, result
}

// AutoHeal checks for gitflow invariant violations and fixes them automatically.
// Currently handles: main ahead of develop (backmerge).
// Returns the action taken ("backmerge", "none") and any error.
func (gf *Logic) AutoHeal() (string, map[string]any, error) {
	gf.Refresh()

	if gf.State.MainAheadOfDevelop > 0 {
		output.Infof("  %sAuto-healing: main is %d commit(s) ahead of develop...%s",
			output.Yellow, gf.State.MainAheadOfDevelop, output.Reset)
		code, result := gf.Backmerge()
		if code == 0 {
			return "backmerge", result, nil
		}
		return "backmerge", result, fmt.Errorf("auto-heal backmerge failed (conflicts)")
	}

	return "none", map[string]any{"result": "healthy"}, nil
}

// StatusWithHealing extends Status() with an action_required field when
// divergence is detected, enabling agents to react programmatically.
func (gf *Logic) StatusWithHealing(autoHeal bool) map[string]any {
	s := gf.Status()

	result := map[string]any{
		"state": s,
	}

	if s.MainAheadOfDevelop > 0 {
		result["action_required"] = "backmerge"
		result["divergence"] = map[string]any{
			"main_ahead": s.MainAheadOfDevelop,
			"files":      s.MainOnlyFiles,
		}

		if autoHeal {
			action, healResult, err := gf.AutoHeal()
			result["auto_heal_action"] = action
			result["auto_heal_result"] = healResult
			if err != nil {
				result["auto_heal_error"] = err.Error()
			}
		}
	}

	return result
}

// detectTestCommand returns the test command for the project, preferring the
// configured value, then auto-detecting from project structure.
func detectTestCommand(projectRoot, configured string) string {
	if configured != "" {
		return configured
	}
	// Go module
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
		return "go test ./..."
	}
	// Node.js
	if _, err := os.Stat(filepath.Join(projectRoot, "package.json")); err == nil {
		return "npm test"
	}
	// Python / pytest
	if _, err := os.Stat(filepath.Join(projectRoot, "pyproject.toml")); err == nil {
		return "pytest"
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "setup.py")); err == nil {
		return "pytest"
	}
	// Makefile with a test target
	if _, err := os.Stat(filepath.Join(projectRoot, "Makefile")); err == nil {
		return "make test"
	}
	return ""
}

// RunTestSuite executes the project test suite and returns whether all tests
// passed, the resolved command used, and any execution output.
func (gf *Logic) RunTestSuite() (passed bool, testCmd string, testOutput string) {
	testCmd = detectTestCommand(gf.Config.ProjectRoot, gf.Config.TestCommand)
	if testCmd == "" {
		return false, "", "no test command configured or detected"
	}

	output.Infof("  %sRunning test suite: %s%s", output.Dim, testCmd, output.Reset)

	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/C"
	} else {
		shell = "sh"
		flag = "-c"
	}

	cmd := exec.Command(shell, flag, testCmd) //nolint:gosec // command is from trusted config
	cmd.Dir = gf.Config.ProjectRoot
	out, err := cmd.CombinedOutput()
	testOutput = strings.TrimSpace(string(out))

	if err != nil {
		output.Infof("  %s✗ Tests failed: %s%s", output.Red, err.Error(), output.Reset)
		return false, testCmd, testOutput
	}
	output.Infof("  %s✓ Tests passed%s", output.Green, output.Reset)
	return true, testCmd, testOutput
}

// TestGatedFinish runs the test suite for the current flow branch and, when
// all tests pass, automatically finishes the branch using SmartFinish (for
// feature/bugfix) or SafeHotfixFinish (for hotfix).
//
// This is a no-op for release branches — releases require explicit human sign-off.
func (gf *Logic) TestGatedFinish(name string) (int, map[string]any) {
	branch := git.CurrentBranch()
	btype := git.BranchTypeOf(branch)

	if btype != "feature" && btype != "bugfix" && btype != "hotfix" {
		return 1, map[string]any{
			"action": "test-gated-finish",
			"error":  fmt.Sprintf("test-gated-finish only applies to feature/bugfix/hotfix (current: %s)", branch),
		}
	}

	passed, testCmd, testOut := gf.RunTestSuite()
	if !passed {
		return 1, map[string]any{
			"action":       "test-gated-finish",
			"result":       "tests_failed",
			"branch":       branch,
			"branch_type":  btype,
			"test_command": testCmd,
			"test_output":  testOut,
			"error":        "tests failed — branch not finished",
		}
	}

	var code int
	var result map[string]any
	if btype == "hotfix" {
		code, result = gf.SafeHotfixFinish(name)
	} else {
		code, result = gf.SmartFinish(name)
	}

	result["test_command"] = testCmd
	result["tests_passed"] = true
	return code, result
}
