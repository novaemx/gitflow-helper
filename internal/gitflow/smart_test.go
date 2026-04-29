package gitflow

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
)

type smartStubGitClient struct {
	execResultFn func(args ...string) (int, string, string)
}

func (s smartStubGitClient) Exec(args ...string) error { return nil }
func (s smartStubGitClient) ExecResult(args ...string) (int, string, string) {
	if s.execResultFn != nil {
		return s.execResultFn(args...)
	}
	return 0, "", ""
}
func (s smartStubGitClient) ExecQuiet(args ...string) string {
	_, stdout, _ := s.ExecResult(args...)
	return stdout
}
func (s smartStubGitClient) ExecLines(args ...string) []string { return nil }

func setupSmartTestRepo(t *testing.T) (string, *Logic) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "initial commit")
	run("git", "branch", "develop")

	cfg := config.FlowConfig{
		ProjectRoot:   dir,
		MainBranch:    "main",
		DevelopBranch: "develop",
		Remote:        "origin",
		TagPrefix:     "v",
	}
	gf := NewFromConfig(cfg)
	return dir, gf
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestPreMergeCheck_NotOnFlowBranch(t *testing.T) {
	_, gf := setupSmartTestRepo(t)

	_, err := gf.PreMergeCheck(false)
	if err == nil {
		t.Error("expected error when not on a flow branch")
	}
}

func TestPreMergeCheck_FeatureBranchNotBehind(t *testing.T) {
	dir, gf := setupSmartTestRepo(t)

	gitRun(t, dir, "checkout", "develop")
	gitRun(t, dir, "checkout", "-b", "feature/test")

	report, err := gf.PreMergeCheck(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.BehindParent != 0 {
		t.Errorf("expected 0 behind, got %d", report.BehindParent)
	}
	if report.RiskLevel != "low" {
		t.Errorf("expected 'low' risk, got %q", report.RiskLevel)
	}
}

func TestPreMergeCheck_FeatureBehindDevelop(t *testing.T) {
	dir, gf := setupSmartTestRepo(t)

	gitRun(t, dir, "checkout", "develop")
	gitRun(t, dir, "checkout", "-b", "feature/test")

	// Advance develop
	gitRun(t, dir, "checkout", "develop")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "advance develop")
	gitRun(t, dir, "checkout", "feature/test")

	report, err := gf.PreMergeCheck(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.BehindParent != 1 {
		t.Errorf("expected 1 behind, got %d", report.BehindParent)
	}
}

func TestPreMergeCheck_AutoSync(t *testing.T) {
	dir, gf := setupSmartTestRepo(t)

	gitRun(t, dir, "checkout", "develop")
	gitRun(t, dir, "checkout", "-b", "feature/test")

	gitRun(t, dir, "checkout", "develop")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "advance develop")
	gitRun(t, dir, "checkout", "feature/test")

	report, err := gf.PreMergeCheck(true)
	if err != nil {
		t.Fatalf("auto-sync failed: %v", err)
	}
	if !report.AutoSynced {
		t.Error("expected AutoSynced to be true")
	}
}

func TestAutoHeal_NoAction(t *testing.T) {
	_, gf := setupSmartTestRepo(t)

	action, _, err := gf.AutoHeal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != "none" {
		t.Errorf("expected 'none', got %q", action)
	}
}

func TestAutoHeal_BackmergeNeeded(t *testing.T) {
	dir, gf := setupSmartTestRepo(t)

	// Advance main ahead of develop
	gitRun(t, dir, "checkout", "main")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "hotfix on main")

	action, result, err := gf.AutoHeal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != "backmerge" {
		t.Errorf("expected 'backmerge', got %q", action)
	}
	if result["result"] != "ok" {
		t.Errorf("expected result 'ok', got %v", result["result"])
	}

	// Verify divergence is healed
	gf.Refresh()
	if gf.State.MainAheadOfDevelop > 0 {
		t.Error("expected divergence to be healed")
	}
}

func TestStatusWithHealing_Healthy(t *testing.T) {
	_, gf := setupSmartTestRepo(t)

	result := gf.StatusWithHealing(false)
	if _, ok := result["action_required"]; ok {
		t.Error("expected no action_required when healthy")
	}
}

func TestStatusWithHealing_Diverged(t *testing.T) {
	dir, gf := setupSmartTestRepo(t)

	gitRun(t, dir, "checkout", "main")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "hotfix on main")

	result := gf.StatusWithHealing(false)
	if result["action_required"] != "backmerge" {
		t.Errorf("expected action_required=backmerge, got %v", result["action_required"])
	}
}

func TestStatusWithHealing_AutoHealDiverge(t *testing.T) {
	dir, gf := setupSmartTestRepo(t)

	gitRun(t, dir, "checkout", "main")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "hotfix on main")

	result := gf.StatusWithHealing(true)
	if result["auto_heal_action"] != "backmerge" {
		t.Errorf("expected auto_heal_action=backmerge, got %v", result["auto_heal_action"])
	}
}

func TestSafeHotfixFinish_NotOnHotfix(t *testing.T) {
	_, gf := setupSmartTestRepo(t)

	code, result := gf.SafeHotfixFinish("")
	if code == 0 {
		t.Error("expected error when not on hotfix branch")
	}
	if result["error"] != "not on a hotfix branch" {
		t.Errorf("unexpected error: %v", result["error"])
	}
}

func TestSafeHotfixFinish_Basic(t *testing.T) {
	dir, gf := setupSmartTestRepo(t)

	gitRun(t, dir, "checkout", "main")
	gitRun(t, dir, "checkout", "-b", "hotfix/1.0.1")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "fix: critical bug")

	code, result := gf.SafeHotfixFinish("")
	if code != 0 {
		t.Errorf("expected code 0, got %d: %v", code, result)
	}
}

func TestSmartFinish_Feature(t *testing.T) {
	dir, gf := setupSmartTestRepo(t)

	gitRun(t, dir, "checkout", "develop")
	gitRun(t, dir, "checkout", "-b", "feature/smart-test")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "feat: smart test")

	code, result := gf.SmartFinish("")
	if code != 0 {
		t.Errorf("expected code 0, got %d: %v", code, result)
	}
	if result["premerge"] == nil {
		t.Error("expected premerge report in result")
	}

	cur := git.CurrentBranch()
	if cur != "develop" {
		t.Errorf("expected to be on develop after finish, got %q", cur)
	}
}

func TestPreMergeCheck_InvalidDivergenceCountReturnsError(t *testing.T) {
	prevClient := git.DefaultClient()
	defer git.SetDefaultClient(prevClient)

	git.SetDefaultClient(smartStubGitClient{
		execResultFn: func(args ...string) (int, string, string) {
			if len(args) >= 2 && args[0] == "branch" && args[1] == "--show-current" {
				return 0, "feature/parse-check", ""
			}
			if len(args) >= 1 && args[0] == "rev-list" {
				return 0, "bad", ""
			}
			return 0, "", ""
		},
	})

	gf := NewFromConfig(config.FlowConfig{MainBranch: "main", DevelopBranch: "develop"})
	_, err := gf.PreMergeCheck(false)
	if err == nil {
		t.Fatal("expected parse error from PreMergeCheck")
	}
	if !strings.Contains(err.Error(), "failed to parse divergence count") {
		t.Fatalf("unexpected error: %v", err)
	}
}
