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

func TestBuildActions_RecommendsPushOnRemoteDefaultBranch(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:             cfg.MainBranch,
		HasDefaultRemote:    true,
		DefaultRemoteBranch: cfg.MainBranch,
		GitFlowInitialized:  true,
		Features:            []state.BranchInfo{},
		Bugfixes:            []state.BranchInfo{},
		Releases:            []state.BranchInfo{},
		Hotfixes:            []state.BranchInfo{},
		DevelopOnlyFiles:    []string{},
		MainOnlyFiles:       []string{},
		Merge:               state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	push, ok := actionByTag(actions, "push")
	if !ok {
		t.Fatal("expected push action on default remote branch")
	}
	if !push.Recommended {
		t.Fatal("expected push to be recommended on the remote default branch")
	}
}

func TestBuildActions_DirtyDevelopPrioritizesMoveToFeatureBranch(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.DevelopBranch,
		Dirty:              true,
		UncommittedCount:   2,
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
	if len(actions) == 0 {
		t.Fatal("expected actions")
	}
	if actions[0].Label != "Move current changes to a feature branch" {
		t.Fatalf("expected first critical action to move changes off develop, got %q", actions[0].Label)
	}
	if !actions[0].Recommended {
		t.Fatal("expected move-to-feature action to be recommended")
	}

	bugfix, ok := actionByTag(actions, "start")
	if !ok {
		t.Fatal("expected start-tag action")
	}
	_ = bugfix
}

func TestBuildActions_DirtyDevelopIncludesBugfixMoveOption(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.DevelopBranch,
		Dirty:              true,
		UncommittedCount:   1,
		GitFlowInitialized: true,
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{},
		Hotfixes:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	featureIdx := actionIndexByLabel(actions, "Move current changes to a feature branch")
	bugfixIdx := actionIndexByLabel(actions, "Move current changes to a bugfix branch")
	if featureIdx == -1 || bugfixIdx == -1 {
		t.Fatalf("expected both move actions, got feature=%d bugfix=%d", featureIdx, bugfixIdx)
	}
	if bugfixIdx < featureIdx {
		t.Fatalf("expected feature recommendation before bugfix alternative, got feature=%d bugfix=%d", featureIdx, bugfixIdx)
	}
	if actions[bugfixIdx].Command != "gitflow start bugfix %s" {
		t.Fatalf("expected bugfix move command, got %q", actions[bugfixIdx].Command)
	}
	if actions[bugfixIdx].Recommended {
		t.Fatal("expected bugfix alternative to be available but not the primary recommendation")
	}
}

func TestBuildActions_DirtyMainPrioritizesMoveToHotfixBranch(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            cfg.MainBranch,
		Dirty:              true,
		UncommittedCount:   1,
		GitFlowInitialized: true,
		Features:           []state.BranchInfo{},
		Bugfixes:           []state.BranchInfo{},
		Releases:           []state.BranchInfo{},
		Hotfixes:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	if len(actions) == 0 {
		t.Fatal("expected actions")
	}
	if actions[0].Label != "Move current changes to a hotfix branch" {
		t.Fatalf("expected first action to move changes off main, got %q", actions[0].Label)
	}
	if !actions[0].Recommended {
		t.Fatal("expected hotfix move action to be recommended")
	}
	if actions[0].Command != "gitflow start hotfix %s" {
		t.Fatalf("expected hotfix move command, got %q", actions[0].Command)
	}
}

func TestBuildActions_AlwaysHasAtLeastOneRecommended(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            "feature/no-remote-no-work",
		HasDefaultRemote:   false,
		GitFlowInitialized: true,
		Dirty:              true,
		Features: []state.BranchInfo{
			{Name: "feature/no-remote-no-work", ShortName: "no-remote-no-work", BranchType: "feature", CommitsAhead: 0, HasRemote: false},
		},
		Merge: state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	hasRecommended := false
	for _, a := range actions {
		if a.Recommended {
			hasRecommended = true
			break
		}
	}
	if !hasRecommended {
		t.Fatal("expected at least one recommended action in every state")
	}
}

func TestBuildActions_PRModeUsesPreparePRLabel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.IntegrationMode = config.IntegrationModePullRequest
	s := state.RepoState{
		Current:            "feature/auth-refresh",
		HasDefaultRemote:   true,
		GitFlowInitialized: true,
		Dirty:              false,
		Features: []state.BranchInfo{
			{Name: "feature/auth-refresh", ShortName: "auth-refresh", BranchType: "feature", CommitsAhead: 2, HasRemote: true},
		},
		Merge: state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	idx := actionIndexByLabel(actions, "Prepare PR for feature 'auth-refresh'")
	if idx == -1 {
		t.Fatalf("expected PR mode finish label, got %#v", actions)
	}
	if actions[idx].Command != "gitflow finish" {
		t.Fatalf("expected command to remain gitflow finish in PR mode, got %q", actions[idx].Command)
	}
}

func TestBuildActions_RecommendsFinishForCleanFeatureWithoutAheadCommits(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            "feature/clean-branch",
		HasDefaultRemote:   true,
		GitFlowInitialized: true,
		Dirty:              false,
		Features:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	finish, ok := actionByTag(actions, "finish")
	if !ok {
		t.Fatal("expected finish action for current feature branch")
	}
	if !finish.Recommended {
		t.Fatal("expected finish to be recommended for a clean feature branch")
	}
}

func TestBuildActions_RecommendsFinishForCleanBugfixWithoutAheadCommits(t *testing.T) {
	cfg := config.DefaultConfig()
	s := state.RepoState{
		Current:            "bugfix/clean-branch",
		HasDefaultRemote:   true,
		GitFlowInitialized: true,
		Dirty:              false,
		Bugfixes:           []state.BranchInfo{},
		Merge:              state.MergeState{ConflictedFiles: []string{}},
	}

	actions := buildActions(s, cfg)
	finish, ok := actionByTag(actions, "finish")
	if !ok {
		t.Fatal("expected finish action for current bugfix branch")
	}
	if !finish.Recommended {
		t.Fatal("expected finish to be recommended for a clean bugfix branch")
	}
}
