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
