package flow

import (
	"errors"
	"reflect"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/output"
)

func TestPushJSONRequiresTargetWhenMultipleCandidates(t *testing.T) {
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	defer output.SetJSONMode(prevJSON)

	prevCurrentBranch := currentBranch
	prevRemoteExists := remoteExists
	prevExecQuiet := execQuiet
	prevExec := exec
	defer func() {
		currentBranch = prevCurrentBranch
		remoteExists = prevRemoteExists
		execQuiet = prevExecQuiet
		exec = prevExec
	}()

	currentBranch = func() string { return "feature/x" }
	remoteExists = func(name string) bool { return name == "origin" }
	execQuiet = func(args ...string) string {
		if len(args) >= 3 && args[0] == "config" && args[1] == "--get" {
			return "refs/heads/develop"
		}
		return ""
	}
	exec = func(args ...string) error { return nil }

	code, result := Push(config.DefaultConfig(), "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if result["result"] != "target_required" {
		t.Fatalf("expected target_required, got %v", result["result"])
	}
	targets, ok := result["available_targets"].([]string)
	if !ok {
		t.Fatalf("expected []string available_targets, got %#v", result["available_targets"])
	}
	want := []string{"feature/x", "develop"}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("expected targets %v, got %v", want, targets)
	}
}

func TestPushJSONBranchMismatchReturnsNeedsHuman(t *testing.T) {
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	defer output.SetJSONMode(prevJSON)

	prevCurrentBranch := currentBranch
	prevRemoteExists := remoteExists
	prevExecQuiet := execQuiet
	defer func() {
		currentBranch = prevCurrentBranch
		remoteExists = prevRemoteExists
		execQuiet = prevExecQuiet
	}()

	currentBranch = func() string { return "feature/x" }
	remoteExists = func(name string) bool { return true }
	execQuiet = func(args ...string) string { return "" }

	code, result := Push(config.DefaultConfig(), "develop")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if result["result"] != "branch_mismatch" {
		t.Fatalf("expected branch_mismatch, got %v", result["result"])
	}
	if result["needs_human"] != true {
		t.Fatalf("expected needs_human=true, got %v", result["needs_human"])
	}
}

func TestPushTextModeCanSwitchBranchAndPush(t *testing.T) {
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(false)
	defer output.SetJSONMode(prevJSON)

	prevCurrentBranch := currentBranch
	prevRemoteExists := remoteExists
	prevExecQuiet := execQuiet
	prevExec := exec
	prevReadChoice := readChoice
	defer func() {
		currentBranch = prevCurrentBranch
		remoteExists = prevRemoteExists
		execQuiet = prevExecQuiet
		exec = prevExec
		readChoice = prevReadChoice
	}()

	currentBranch = func() string { return "feature/x" }
	remoteExists = func(name string) bool { return true }
	execQuiet = func(args ...string) string { return "" }

	choices := []string{"1"}
	readChoice = func() string {
		if len(choices) == 0 {
			return ""
		}
		v := choices[0]
		choices = choices[1:]
		return v
	}

	var calls [][]string
	exec = func(args ...string) error {
		calls = append(calls, append([]string{}, args...))
		return nil
	}

	code, result := Push(config.DefaultConfig(), "develop")
	if code != 0 {
		t.Fatalf("expected code 0, got %d (%v)", code, result)
	}
	if result["result"] != "ok" {
		t.Fatalf("expected ok result, got %v", result["result"])
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 exec calls (checkout + push), got %d", len(calls))
	}
	if !reflect.DeepEqual(calls[0], []string{"checkout", "develop"}) {
		t.Fatalf("unexpected first call: %v", calls[0])
	}
	if !reflect.DeepEqual(calls[1], []string{"push", "-u", "origin", "develop:develop"}) {
		t.Fatalf("unexpected second call: %v", calls[1])
	}
}

func TestPushReturnsErrorWhenPushFails(t *testing.T) {
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(false)
	defer output.SetJSONMode(prevJSON)

	prevCurrentBranch := currentBranch
	prevRemoteExists := remoteExists
	prevExecQuiet := execQuiet
	prevExec := exec
	defer func() {
		currentBranch = prevCurrentBranch
		remoteExists = prevRemoteExists
		execQuiet = prevExecQuiet
		exec = prevExec
	}()

	currentBranch = func() string { return "feature/x" }
	remoteExists = func(name string) bool { return true }
	execQuiet = func(args ...string) string { return "" }
	exec = func(args ...string) error {
		return errors.New("push failed")
	}

	code, result := Push(config.DefaultConfig(), "feature/x")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if result["result"] != "error" {
		t.Fatalf("expected result error, got %v", result["result"])
	}
}
