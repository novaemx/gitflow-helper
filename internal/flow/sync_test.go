package flow

import (
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/output"
)

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
