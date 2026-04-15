package flow

import (
	"errors"
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/output"
)

func TestMergedBranchDeleteWarning(t *testing.T) {
	msg := mergedBranchDeleteWarning("feature/x", errors.New("still needed"))
	if !strings.Contains(msg, "feature/x") {
		t.Fatalf("expected branch name in warning, got: %s", msg)
	}
	if !strings.Contains(msg, "git branch -d feature/x") {
		t.Fatalf("expected manual delete hint, got: %s", msg)
	}
}

func TestAddMergeAbortDiagnostics_JSONAbortFailure(t *testing.T) {
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	defer output.SetJSONMode(prevJSON)

	prevExec := execResultFinish
	execResultFinish = func(args ...string) (int, string, string) {
		if len(args) >= 2 && args[0] == "merge" && args[1] == "--abort" {
			return 1, "", "abort failed"
		}
		return 0, "", ""
	}
	defer func() { execResultFinish = prevExec }()

	result := map[string]any{"result": "conflict"}
	addMergeAbortDiagnostics(result)

	if result["abort_failed"] != true {
		t.Fatalf("expected abort_failed=true, got %#v", result["abort_failed"])
	}
	if !strings.Contains(result["abort_error"].(string), "abort failed") {
		t.Fatalf("expected abort_error content, got %v", result["abort_error"])
	}
}

func TestAddMergeAbortDiagnostics_TextModeNoop(t *testing.T) {
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(false)
	defer output.SetJSONMode(prevJSON)

	result := map[string]any{"result": "conflict"}
	addMergeAbortDiagnostics(result)
	if _, ok := result["abort_failed"]; ok {
		t.Fatal("did not expect abort diagnostics in text mode")
	}
}

// ── nonAtomicCommitWarnings ────────────────────────────────────────────────

func TestNonAtomicCommitWarnings_DetectsAndInBody(t *testing.T) {
	subjects := []string{"feat(tui): add activity panel and improve selector"}
	warns := nonAtomicCommitWarnings(subjects)
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning for ' and ' in body, got %d: %v", len(warns), warns)
	}
	if warns[0] != subjects[0] {
		t.Fatalf("expected warning to contain original subject, got %q", warns[0])
	}
}

func TestNonAtomicCommitWarnings_DetectsSemicolonInBody(t *testing.T) {
	subjects := []string{"chore: remove deprecated binaries; enhance conflict handling"}
	warns := nonAtomicCommitWarnings(subjects)
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning for '; ' in body, got %d: %v", len(warns), warns)
	}
}

func TestNonAtomicCommitWarnings_CleanSubjectsReturnNil(t *testing.T) {
	subjects := []string{
		"feat(flow): add guard for release branch naming",
		"fix(commands): handle empty merge_head in status",
		"docs(skill): clarify conflict escalation path",
		"chore: remove unused imports",
		"refactor(tui): simplify action ordering",
	}
	warns := nonAtomicCommitWarnings(subjects)
	if len(warns) != 0 {
		t.Fatalf("expected no warnings for clean subjects, got: %v", warns)
	}
}

func TestNonAtomicCommitWarnings_AndInTypePrefix_NotFlagged(t *testing.T) {
	// "and" appears in the conventional commit scope, not the body — should not warn.
	// After stripping ": " the body is "improve rendering" which is clean.
	subjects := []string{"feat(select-and-filter): improve rendering"}
	warns := nonAtomicCommitWarnings(subjects)
	if len(warns) != 0 {
		t.Fatalf("expected no warning when 'and' is only in scope prefix, got: %v", warns)
	}
}

func TestNonAtomicCommitWarnings_MixedBatch(t *testing.T) {
	subjects := []string{
		"feat(tui): add toggle and fix resize bug", // non-atomic
		"fix(flow): correct nil pointer",           // clean
		"chore: cleanup files; update ci",          // non-atomic
	}
	warns := nonAtomicCommitWarnings(subjects)
	if len(warns) != 2 {
		t.Fatalf("expected 2 warnings in mixed batch, got %d: %v", len(warns), warns)
	}
}

func TestNonAtomicCommitWarnings_EmptyInput(t *testing.T) {
	warns := nonAtomicCommitWarnings(nil)
	if warns != nil {
		t.Fatalf("expected nil for empty input, got: %v", warns)
	}
}
