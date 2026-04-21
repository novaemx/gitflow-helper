package flow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
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
