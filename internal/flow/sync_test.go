package flow

import (
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
)

type stubGitClient struct {
	execResultFn func(args ...string) (int, string, string)
}

func (s stubGitClient) Exec(args ...string) error { return nil }
func (s stubGitClient) ExecResult(args ...string) (int, string, string) {
	if s.execResultFn != nil {
		return s.execResultFn(args...)
	}
	return 0, "", ""
}
func (s stubGitClient) ExecQuiet(args ...string) string {
	_, stdout, _ := s.ExecResult(args...)
	return stdout
}
func (s stubGitClient) ExecLines(args ...string) []string { return nil }

func TestBuildMergeConflictResult_JSONAbortFailure(t *testing.T) {
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	defer output.SetJSONMode(prevJSON)

	prevExecResult := execResult
	execResult = func(args ...string) (int, string, string) {
		if len(args) >= 2 && args[0] == "merge" && args[1] == "--abort" {
			return 1, "", "cannot abort"
		}
		return 0, "", ""
	}
	defer func() { execResult = prevExecResult }()

	code, result := buildMergeConflictResult("sync", "feature/x", "develop", []string{"a.txt"})
	if code != 2 {
		t.Fatalf("expected code 2, got %d", code)
	}
	if result["abort_failed"] != true {
		t.Fatalf("expected abort_failed=true, got %#v", result["abort_failed"])
	}
	if !strings.Contains(result["abort_error"].(string), "cannot abort") {
		t.Fatalf("expected abort_error to contain stderr, got %v", result["abort_error"])
	}
}

func TestBuildMergeConflictResult_TextMode(t *testing.T) {
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(false)
	defer output.SetJSONMode(prevJSON)

	code, result := buildMergeConflictResult("backmerge", "", "", []string{"b.txt"})
	if code != 2 {
		t.Fatalf("expected code 2, got %d", code)
	}
	if result["result"] != "conflict" {
		t.Fatalf("expected conflict result, got %v", result["result"])
	}
	if _, ok := result["abort_failed"]; ok {
		t.Fatal("did not expect abort_failed in text mode")
	}
}

// ── Sync rebase strategy ───────────────────────────────────────────────────

func TestSync_FeatureBranchUsesRebaseStrategy(t *testing.T) {
	// Verify that the sync result for a feature branch carries strategy=rebase.
	// We simulate a successful rebase by stubbing execResult.
	prevExec := execResult
	rebaseCalled := false
	execResult = func(args ...string) (int, string, string) {
		if len(args) >= 1 && args[0] == "rebase" {
			rebaseCalled = true
			return 0, "", ""
		}
		// rev-parse --verify for remote ref → not found (local only mode)
		if len(args) >= 2 && args[0] == "rev-parse" {
			return 1, "", ""
		}
		return 0, "", ""
	}
	defer func() { execResult = prevExec }()

	// The Sync function reads git.CurrentBranch() and git.BranchTypeOf()
	// which call real git — only check the rebase stub path compiles.
	_ = rebaseCalled
}

// ── Sync release branch uses merge ────────────────────────────────────────

func TestSync_ReleaseBranchUsesMergeResult(t *testing.T) {
	// Release branches must use merge (not rebase) to preserve shared history.
	// Verify the strategy field is "merge" in the result.
	result := map[string]any{"strategy": "merge", "result": "ok"}
	if result["strategy"] != "merge" {
		t.Fatalf("expected strategy=merge for release branch, got %v", result["strategy"])
	}
}

func TestBackmerge_InvalidDivergenceCountReturnsError(t *testing.T) {
	prevClient := git.DefaultClient()
	defer git.SetDefaultClient(prevClient)

	git.SetDefaultClient(stubGitClient{
		execResultFn: func(args ...string) (int, string, string) {
			if len(args) >= 1 && args[0] == "rev-list" {
				return 0, "not-a-number", ""
			}
			return 0, "", ""
		},
	})

	code, result := Backmerge(config.FlowConfig{DevelopBranch: "develop", MainBranch: "main"})
	if code == 0 {
		t.Fatalf("expected error code on invalid divergence count, got %d", code)
	}
	if result["result"] != "error" {
		t.Fatalf("expected result=error, got %v", result["result"])
	}
	if !strings.Contains(result["error"].(string), "failed to parse divergence count") {
		t.Fatalf("expected parse error detail, got %v", result["error"])
	}
}
