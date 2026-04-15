package tui

import (
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/state"
)

func containsLine(lines []dashLine, needle string) bool {
	for _, l := range lines {
		if strings.Contains(l.text, needle) {
			return true
		}
	}
	return false
}

func TestBuildDashboardLines_MetadataOnlyAheadExplainsBackmergeCommit(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.DevelopBranch,
		DevelopAheadOfMain: 1,
		MainAheadOfDevelop: 0,
		DevelopOnlyFiles:   []string{},
		MainOnlyFiles:      []string{},
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{},
		Hotfixes:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	lines := buildDashboardLines(s, cfg)
	if !containsLine(lines, "Expected after finish release/hotfix") {
		t.Fatalf("expected metadata-only explanation line")
	}
	if !containsLine(lines, "metadata-only back-merge commit; no release needed") {
		t.Fatalf("expected phase analysis metadata explanation")
	}
}

func TestBuildDashboardLines_MainAheadShowsCauseAndAction(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.DevelopBranch,
		DevelopAheadOfMain: 0,
		MainAheadOfDevelop: 2,
		DevelopOnlyFiles:   []string{},
		MainOnlyFiles:      []string{"x.go"},
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{},
		Hotfixes:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	lines := buildDashboardLines(s, cfg)
	if !containsLine(lines, "Common causes: finished hotfix/release") {
		t.Fatalf("expected common causes guidance")
	}
	if !containsLine(lines, "Next step: gitflow backmerge") {
		t.Fatalf("expected explicit backmerge next step")
	}
}

func TestBuildDashboardLines_OpenReleaseShowsHowToProceed(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.DevelopBranch,
		DevelopAheadOfMain: 0,
		MainAheadOfDevelop: 0,
		DevelopOnlyFiles:   []string{},
		MainOnlyFiles:      []string{},
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{{Name: "release/1.2.3"}},
		Hotfixes:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	lines := buildDashboardLines(s, cfg)
	if !containsLine(lines, "Release 'release/1.2.3' is open") {
		t.Fatalf("expected release-open warning")
	}
	if !containsLine(lines, "Switch to release branch and run finish") {
		t.Fatalf("expected actionable release next step")
	}
}

func TestBuildDashboardLines_DirtyDevelopShowsProtectedBranchViolation(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.DevelopBranch,
		Dirty:              true,
		UncommittedCount:   2,
		DevelopAheadOfMain: 0,
		MainAheadOfDevelop: 0,
		DevelopOnlyFiles:   []string{},
		MainOnlyFiles:      []string{},
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{},
		Hotfixes:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	lines := buildDashboardLines(s, cfg)
	if !containsLine(lines, "PROTECTED BRANCH VIOLATION: local changes detected on develop") {
		t.Fatalf("expected protected branch violation warning")
	}
	if !containsLine(lines, "Recommended: gitflow start feature <name>") {
		t.Fatalf("expected remediation guidance")
	}
	if !containsLine(lines, "Use the recommended TUI action to move them without losing local work") {
		t.Fatalf("expected TUI remediation guidance")
	}
}

func TestBuildDashboardLines_DirtyMainShowsTUIHotfixGuidance(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.MainBranch,
		Dirty:              true,
		UncommittedCount:   1,
		DevelopAheadOfMain: 0,
		MainAheadOfDevelop: 0,
		DevelopOnlyFiles:   []string{},
		MainOnlyFiles:      []string{},
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{},
		Hotfixes:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	lines := buildDashboardLines(s, cfg)
	if !containsLine(lines, "PROTECTED BRANCH VIOLATION: local changes detected on main") {
		t.Fatalf("expected protected branch violation warning on main")
	}
	if !containsLine(lines, "Use the recommended TUI action to move them without losing local work") {
		t.Fatalf("expected TUI remediation guidance on main")
	}
	if !containsLine(lines, "Recommended: gitflow start hotfix <version>") {
		t.Fatalf("expected hotfix remediation guidance")
	}
}
