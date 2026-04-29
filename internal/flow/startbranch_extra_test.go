package flow

import (
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
)

// ── StartBranch invalid type ───────────────────────────────────────────────

func TestStartBranch_InvalidType(t *testing.T) {
	code, result := StartBranch(config.FlowConfig{}, "unknown", "foo")
	if code != 1 {
		t.Fatalf("expected code 1 for invalid type, got %d", code)
	}
	if result["action"] != "start" {
		t.Fatalf("expected action=start, got %v", result["action"])
	}
}

// ── StartBranch feature success (clean tree) ──────────────────────────────

func TestStartBranch_Success_Feature_CleanTree(t *testing.T) {
	prevWT := workingTreeStatusStart
	prevBranchExists := branchExistsStart
	prevTagExists := tagExistsStart
	prevStartFeature := startFeatureFn
	defer func() {
		workingTreeStatusStart = prevWT
		branchExistsStart = prevBranchExists
		tagExistsStart = prevTagExists
		startFeatureFn = prevStartFeature
	}()

	workingTreeStatusStart = func() git.WorkTreeStatus { return git.WorkTreeStatus{} }
	branchExistsStart = func(string) bool { return false }
	tagExistsStart = func(string) bool { return false }
	startFeatureFn = func(config.FlowConfig, string) error { return nil }

	code, result := StartBranch(config.FlowConfig{}, "feature", "my-feature")
	if code != 0 {
		t.Fatalf("expected code 0 for successful feature start, got %d: %v", code, result)
	}
	if result["result"] != "ok" {
		t.Fatalf("expected result=ok, got %v", result["result"])
	}
	if result["branch"] != "feature/my-feature" {
		t.Fatalf("expected branch=feature/my-feature, got %v", result["branch"])
	}
}

// ── StartBranch branch already exists ─────────────────────────────────────

func TestStartBranch_BranchAlreadyExists(t *testing.T) {
	prevWT := workingTreeStatusStart
	prevBranchExists := branchExistsStart
	prevTagExists := tagExistsStart
	defer func() {
		workingTreeStatusStart = prevWT
		branchExistsStart = prevBranchExists
		tagExistsStart = prevTagExists
	}()

	workingTreeStatusStart = func() git.WorkTreeStatus { return git.WorkTreeStatus{} }
	branchExistsStart = func(branch string) bool { return branch == "feature/existing" }
	tagExistsStart = func(string) bool { return false }

	code, result := StartBranch(config.FlowConfig{}, "feature", "existing")
	if code != 1 {
		t.Fatalf("expected code 1 when branch exists, got %d", code)
	}
	if result["result"] != "error" {
		t.Fatalf("expected result=error, got %v", result["result"])
	}
}

// ── StartBranch release tag already exists ────────────────────────────────

func TestStartBranch_ReleaseTagAlreadyExists(t *testing.T) {
	prevWT := workingTreeStatusStart
	prevBranchExists := branchExistsStart
	prevTagExists := tagExistsStart
	defer func() {
		workingTreeStatusStart = prevWT
		branchExistsStart = prevBranchExists
		tagExistsStart = prevTagExists
	}()

	workingTreeStatusStart = func() git.WorkTreeStatus { return git.WorkTreeStatus{} }
	branchExistsStart = func(string) bool { return false }
	tagExistsStart = func(tag string) bool { return tag == "v1.2.3" }

	code, result := StartBranch(config.FlowConfig{TagPrefix: "v"}, "release", "1.2.3")
	if code != 1 {
		t.Fatalf("expected code 1 when tag exists, got %d", code)
	}
	if result["result"] != "error" {
		t.Fatalf("expected result=error, got %v", result["result"])
	}
}

// ── StartBranch start function error ──────────────────────────────────────

func TestStartBranch_StartFnError_NoStash(t *testing.T) {
	prevWT := workingTreeStatusStart
	prevBranchExists := branchExistsStart
	prevTagExists := tagExistsStart
	prevStartFeature := startFeatureFn
	defer func() {
		workingTreeStatusStart = prevWT
		branchExistsStart = prevBranchExists
		tagExistsStart = prevTagExists
		startFeatureFn = prevStartFeature
	}()

	workingTreeStatusStart = func() git.WorkTreeStatus { return git.WorkTreeStatus{} }
	branchExistsStart = func(string) bool { return false }
	tagExistsStart = func(string) bool { return false }
	startFeatureFn = func(_ config.FlowConfig, _ string) error {
		return &startFnErr{msg: "git checkout failed"}
	}

	code, result := StartBranch(config.FlowConfig{}, "feature", "broken")
	if code != 1 {
		t.Fatalf("expected code 1 when startFn fails, got %d", code)
	}
	if result["result"] != "error" {
		t.Fatalf("expected result=error, got %v", result["result"])
	}
}

// ── StartBranch success with stash ────────────────────────────────────────

func TestStartBranch_SuccessWithStash(t *testing.T) {
	prevWT := workingTreeStatusStart
	prevStashSave := stashSaveStart
	prevStashPop := stashPopStart
	prevBranchExists := branchExistsStart
	prevTagExists := tagExistsStart
	prevStartFeature := startFeatureFn
	prevExecResult := execResultStart
	defer func() {
		workingTreeStatusStart = prevWT
		stashSaveStart = prevStashSave
		stashPopStart = prevStashPop
		branchExistsStart = prevBranchExists
		tagExistsStart = prevTagExists
		startFeatureFn = prevStartFeature
		execResultStart = prevExecResult
	}()

	workingTreeStatusStart = func() git.WorkTreeStatus {
		return git.WorkTreeStatus{Staged: 1}
	}
	stashSaveStart = func(string) error { return nil }
	branchExistsStart = func(string) bool { return false }
	tagExistsStart = func(string) bool { return false }
	startFeatureFn = func(config.FlowConfig, string) error { return nil }
	execResultStart = func(args ...string) (int, string, string) {
		if len(args) == 2 && args[0] == "stash" && args[1] == "pop" {
			return 0, "", ""
		}
		return 0, "", ""
	}

	code, result := StartBranch(config.FlowConfig{}, "feature", "with-stash")
	if code != 0 {
		t.Fatalf("expected code 0 for success with stash, got %d: %v", code, result)
	}
	if result["stash_restore"] != "ok" {
		t.Fatalf("expected stash_restore=ok, got %v", result["stash_restore"])
	}
}

// ── StartBranch bugfix success ────────────────────────────────────────────

func TestStartBranch_Success_Bugfix(t *testing.T) {
	prevWT := workingTreeStatusStart
	prevBranchExists := branchExistsStart
	prevTagExists := tagExistsStart
	prevStartBugfix := startBugfixFn
	defer func() {
		workingTreeStatusStart = prevWT
		branchExistsStart = prevBranchExists
		tagExistsStart = prevTagExists
		startBugfixFn = prevStartBugfix
	}()

	workingTreeStatusStart = func() git.WorkTreeStatus { return git.WorkTreeStatus{} }
	branchExistsStart = func(string) bool { return false }
	tagExistsStart = func(string) bool { return false }
	startBugfixFn = func(config.FlowConfig, string) error { return nil }

	code, result := StartBranch(config.FlowConfig{}, "bugfix", "fix-123")
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %v", code, result)
	}
	if result["branch"] != "bugfix/fix-123" {
		t.Fatalf("expected bugfix/fix-123, got %v", result["branch"])
	}
}

// ── StartFeature / StartBugfix validation ─────────────────────────────────

func TestStartFeature_EmptyName(t *testing.T) {
	err := StartFeature(config.FlowConfig{}, "")
	if err == nil {
		t.Fatal("expected error for empty feature name")
	}
}

func TestStartBugfix_EmptyName(t *testing.T) {
	err := StartBugfix(config.FlowConfig{}, "")
	if err == nil {
		t.Fatal("expected error for empty bugfix name")
	}
}

func TestStartRelease_EmptyVersion(t *testing.T) {
	err := StartRelease(config.FlowConfig{}, "")
	if err == nil {
		t.Fatal("expected error for empty release version")
	}
}

func TestStartHotfix_EmptyVersion(t *testing.T) {
	err := StartHotfix(config.FlowConfig{}, "")
	if err == nil {
		t.Fatal("expected error for empty hotfix version")
	}
}

// startFnErr is a test-local error type for start function failures.
type startFnErr struct{ msg string }

func (e *startFnErr) Error() string { return e.msg }
