package flow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
)

func TestResolveStartVersion_AutoFallsBackToLatestSemverTag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("not-a-semver\n"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	prevExecLines := execLinesStart
	prevTagExists := tagExistsStart
	prevBranchExists := branchExistsStart
	defer func() {
		execLinesStart = prevExecLines
		tagExistsStart = prevTagExists
		branchExistsStart = prevBranchExists
	}()

	execLinesStart = func(args ...string) []string {
		return []string{"v1.4.2", "build-123", "release-candidate"}
	}
	tagExistsStart = func(tag string) bool { return tag == "v1.4.2" }
	branchExistsStart = func(string) bool { return false }

	resolved, details, err := resolveStartVersion(config.FlowConfig{
		ProjectRoot: dir,
		VersionFile: "VERSION",
		TagPrefix:   "v",
	}, "release", "auto")
	if err != nil {
		t.Fatalf("expected fallback to latest semver tag, got error: %v", err)
	}
	if resolved != "1.4.3" {
		t.Fatalf("expected 1.4.3, got %q", resolved)
	}
	if details.Source != "latest_semver_tag" {
		t.Fatalf("expected latest_semver_tag source, got %q", details.Source)
	}
	if details.BaseVersion != "1.4.2" {
		t.Fatalf("expected base version 1.4.2, got %q", details.BaseVersion)
	}
}

func TestResolveStartVersion_AutoSkipsExistingReleaseBranch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.4.2\n"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	prevExecLines := execLinesStart
	prevTagExists := tagExistsStart
	prevBranchExists := branchExistsStart
	defer func() {
		execLinesStart = prevExecLines
		tagExistsStart = prevTagExists
		branchExistsStart = prevBranchExists
	}()

	execLinesStart = func(args ...string) []string { return nil }
	tagExistsStart = func(tag string) bool { return tag == "v1.4.2" }
	branchExistsStart = func(branch string) bool { return branch == "release/1.4.3" }

	resolved, details, err := resolveStartVersion(config.FlowConfig{
		ProjectRoot: dir,
		VersionFile: "VERSION",
		TagPrefix:   "v",
	}, "release", "auto")
	if err != nil {
		t.Fatalf("expected auto version selection to skip occupied refs, got error: %v", err)
	}
	if resolved != "1.4.4" {
		t.Fatalf("expected 1.4.4, got %q", resolved)
	}
	if details.Source != "version_file" {
		t.Fatalf("expected version_file source, got %q", details.Source)
	}
	if len(details.Skipped) != 2 {
		t.Fatalf("expected 2 skipped candidates, got %v", details.Skipped)
	}
}

func TestStartBranch_StashRestoreConflictReturnsNeedsHuman(t *testing.T) {
	prevWT := workingTreeStatusStart
	prevStashSave := stashSaveStart
	prevExecResult := execResultStart
	prevStartFeature := startFeatureFn
	defer func() {
		workingTreeStatusStart = prevWT
		stashSaveStart = prevStashSave
		execResultStart = prevExecResult
		startFeatureFn = prevStartFeature
	}()

	workingTreeStatusStart = func() git.WorkTreeStatus {
		return git.WorkTreeStatus{Unstaged: 1}
	}
	stashSaveStart = func(string) error { return nil }
	startFeatureFn = func(config.FlowConfig, string) error { return nil }
	execResultStart = func(args ...string) (int, string, string) {
		if len(args) == 2 && args[0] == "stash" && args[1] == "pop" {
			return 1, "", "conflict while applying stash"
		}
		return 0, "", ""
	}

	code, result := StartBranch(config.FlowConfig{}, "feature", "abc")
	if code != 2 {
		t.Fatalf("expected code 2 for stash conflict, got %d", code)
	}
	if result["result"] != "conflict" {
		t.Fatalf("expected conflict result, got %v", result["result"])
	}
	if result["needs_human"] != true {
		t.Fatalf("expected needs_human=true, got %v", result["needs_human"])
	}
	if result["stash_restore"] != "conflict" {
		t.Fatalf("expected stash_restore=conflict, got %v", result["stash_restore"])
	}
}

func TestStartBranch_StashSaveError(t *testing.T) {
	prevWT := workingTreeStatusStart
	prevStashSave := stashSaveStart
	defer func() {
		workingTreeStatusStart = prevWT
		stashSaveStart = prevStashSave
	}()

	workingTreeStatusStart = func() git.WorkTreeStatus {
		return git.WorkTreeStatus{Unstaged: 1}
	}
	stashSaveStart = func(string) error { return errors.New("boom") }

	code, result := StartBranch(config.FlowConfig{}, "feature", "abc")
	if code != 1 {
		t.Fatalf("expected code 1 on stash save error, got %d", code)
	}
	if result["result"] != "error" {
		t.Fatalf("expected error result, got %v", result["result"])
	}
}
