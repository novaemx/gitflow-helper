package tui

import (
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/state"
)

func actionIndexByLabel(actions []action, label string) int {
	for i, a := range actions {
		if a.Label == label {
			return i
		}
	}
	return -1
}

func actionByTag(actions []action, tag string) (action, bool) {
	for _, a := range actions {
		if a.Tag == tag {
			return a, true
		}
	}
	return action{}, false
}

func TestBuildActions_PreservesTierPriorityOverGlobalRecommended(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.DevelopBranch,
		HasDefaultRemote:   true,
		GitFlowInitialized: true,
		MainAheadOfDevelop: 2,
		MainOnlyFiles:      []string{"a.go", "b.go"},
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{},
		Hotfixes:           []state.BranchInfo{},
		DevelopOnlyFiles:   []string{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	diffLabel := "View diff: main vs develop (2 file(s))"
	startFeatureLabel := "Start a new feature"

	diffIdx := actionIndexByLabel(actions, diffLabel)
	startIdx := actionIndexByLabel(actions, startFeatureLabel)
	if diffIdx == -1 || startIdx == -1 {
		t.Fatalf("expected both '%s' and '%s' actions", diffLabel, startFeatureLabel)
	}
	if diffIdx > startIdx {
		t.Fatalf("expected critical-tier diff action before normal-tier recommended action, got diff=%d start=%d", diffIdx, startIdx)
	}
}

func TestBuildActions_RecommendsPushForUnpublishedFlowBranch(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            "feature/auth-refresh",
		HasDefaultRemote:   true,
		GitFlowInitialized: true,
		Dirty:              false,
		Features: []state.BranchInfo{
			{Name: "feature/auth-refresh", ShortName: "auth-refresh", BranchType: "feature", CommitsAhead: 3, HasRemote: false},
		},
		Bugfixes:         []state.BranchInfo{},
		Releases:         []state.BranchInfo{},
		Hotfixes:         []state.BranchInfo{},
		DevelopOnlyFiles: []string{},
		MainOnlyFiles:    []string{},
		Merge:            state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	push, ok := actionByTag(actions, "push")
	if !ok {
		t.Fatal("expected push action for flow branch")
	}
	if push.Command != "gitflow push" {
		t.Fatalf("expected push command 'gitflow push', got %q", push.Command)
	}
	if !push.Recommended {
		t.Fatalf("expected push to be recommended when flow branch has no remote")
	}
}

func TestBuildActions_ShowsPushOnDevelop(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.DevelopBranch,
		HasDefaultRemote:   true,
		GitFlowInitialized: true,
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{},
		Hotfixes:           []state.BranchInfo{},
		DevelopOnlyFiles:   []string{},
		MainOnlyFiles:      []string{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	push, ok := actionByTag(actions, "push")
	if !ok {
		t.Fatal("expected push action on develop branch")
	}
	if push.Command != "gitflow push" {
		t.Fatalf("expected push command 'gitflow push', got %q", push.Command)
	}
}
