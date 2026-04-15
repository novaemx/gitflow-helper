package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/branch"
	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/state"
)

type action struct {
	Label        string
	Tag          string
	Recommended  bool
	Command      string
	NeedsInput   bool
	InputPrompt  string
	InputDefault string
}

func hasTag(actions []action, tag string) bool {
	for _, a := range actions {
		if a.Tag == tag {
			return true
		}
	}
	return false
}

func hasTagAndLabel(actions []action, tag, needle string) bool {
	for _, a := range actions {
		if a.Tag == tag && strings.Contains(a.Label, needle) {
			return true
		}
	}
	return false
}

func currentFlowBranchInfo(s state.RepoState, btype string) *state.BranchInfo {
	var list []state.BranchInfo
	switch btype {
	case "feature":
		list = s.Features
	case "bugfix":
		list = s.Bugfixes
	case "release":
		list = s.Releases
	case "hotfix":
		list = s.Hotfixes
	default:
		return nil
	}
	for i := range list {
		if list[i].Name == s.Current {
			return &list[i]
		}
	}
	return nil
}

func appendTierWithPriority(dst []action, tier []action) []action {
	for _, a := range tier {
		if a.Recommended {
			dst = append(dst, a)
		}
	}
	for _, a := range tier {
		if !a.Recommended {
			dst = append(dst, a)
		}
	}
	return dst
}

// buildActions builds the action list ordered by priority tiers:
//
//	T1 CRITICAL  — backmerge / continue merge / init
//	T2 HIGH      — finish flow branch / sync with parent
//	T3 NORMAL    — pull, start recommended work, view diffs
//	T4 LOW       — switch branches, utilities, exit
func buildActions(s state.RepoState, cfg config.FlowConfig) []action {
	btype := branch.TypeOf(s.Current)
	ms := s.Merge

	// ── Merge conflict — locked menu ────────────────────────
	if ms.InMerge && len(ms.ConflictedFiles) > 0 {
		var actions []action
		actions = append(actions, action{
			Label:       fmt.Sprintf("Resolve %d merge conflict(s)", len(ms.ConflictedFiles)),
			Tag:         "resolve",
			Recommended: true,
		})
		op := "merge"
		if ms.OperationType != "" {
			op = ms.OperationType + " finish"
		}
		actions = append(actions, action{Label: "Abort the " + op, Tag: "abort", Command: "git merge --abort"})
		actions = append(actions, action{Label: "Exit", Tag: "exit"})
		return actions
	}

	// ── In-merge, no conflicts — continue finish ────────────
	if ms.InMerge && len(ms.ConflictedFiles) == 0 && ms.OperationType != "" {
		return []action{
			{
				Label:       fmt.Sprintf("Continue %s finish v%s", ms.OperationType, ms.OperationVersion),
				Tag:         "continue",
				Recommended: true,
				Command:     "gitflow finish",
			},
			{Label: "Exit", Tag: "exit"},
		}
	}

	// ── Tiered action accumulation ──────────────────────────
	var critical, high, normal, low []action

	// T1 CRITICAL: gitflow invariant violations
	if s.MainAheadOfDevelop > 0 {
		critical = append(critical, action{
			Label:       fmt.Sprintf("Back-merge %s into %s (%d commit(s) behind)", cfg.MainBranch, cfg.DevelopBranch, s.MainAheadOfDevelop),
			Tag:         "backmerge",
			Recommended: true,
			Command:     "gitflow backmerge",
		})
		critical = append(critical, action{
			Label:   fmt.Sprintf("View diff: %s vs %s (%d file(s))", cfg.MainBranch, cfg.DevelopBranch, len(s.MainOnlyFiles)),
			Tag:     "diff",
			Command: fmt.Sprintf("git diff --stat %s...%s", cfg.DevelopBranch, cfg.MainBranch),
		})
	}

	if !s.GitFlowInitialized {
		critical = append(critical, action{
			Label:       "Initialize gitflow",
			Tag:         "init",
			Recommended: true,
			Command:     "gitflow init",
		})
	}

	// T2 HIGH: finish current flow branch / sync with parent
	dirtyNote := ""
	if s.Dirty {
		dirtyNote = " ⚠ commit changes first"
	}
	curFlow := currentFlowBranchInfo(s, btype)
	switch btype {
	case "feature":
		name := strings.TrimPrefix(s.Current, "feature/")
		hasWork := curFlow != nil && curFlow.CommitsAhead > 0 && !s.Dirty
		label := fmt.Sprintf("Finish feature '%s'", name)
		if s.Dirty {
			label += dirtyNote
		}
		if s.HasDefaultRemote {
			high = append(high, action{
				Label:       "Push current branch",
				Tag:         "push",
				Recommended: curFlow != nil && !curFlow.HasRemote && !s.Dirty,
				Command:     "gitflow push",
			})
		}
		high = append(high, action{
			Label: label, Tag: "finish",
			Recommended: hasWork, Command: "gitflow finish",
		})
		high = append(high, action{Label: fmt.Sprintf("Sync with %s", cfg.DevelopBranch), Tag: "sync", Command: "gitflow sync"})

	case "bugfix":
		name := strings.TrimPrefix(s.Current, "bugfix/")
		hasWork := curFlow != nil && curFlow.CommitsAhead > 0 && !s.Dirty
		label := fmt.Sprintf("Finish bugfix '%s'", name)
		if s.Dirty {
			label += dirtyNote
		}
		if s.HasDefaultRemote {
			high = append(high, action{
				Label:       "Push current branch",
				Tag:         "push",
				Recommended: curFlow != nil && !curFlow.HasRemote && !s.Dirty,
				Command:     "gitflow push",
			})
		}
		high = append(high, action{
			Label: label, Tag: "finish",
			Recommended: hasWork, Command: "gitflow finish",
		})
		high = append(high, action{Label: fmt.Sprintf("Sync with %s", cfg.DevelopBranch), Tag: "sync", Command: "gitflow sync"})

	case "release":
		ver := strings.TrimPrefix(strings.TrimPrefix(s.Current, "release/v"), "release/")
		label := fmt.Sprintf("Finish release v%s", ver)
		if s.Dirty {
			label += dirtyNote
		}
		if s.HasDefaultRemote {
			high = append(high, action{
				Label:       "Push current branch",
				Tag:         "push",
				Recommended: curFlow != nil && !curFlow.HasRemote && !s.Dirty,
				Command:     "gitflow push",
			})
		}
		high = append(high, action{
			Label: label, Tag: "finish",
			Recommended: !s.Dirty, Command: "gitflow finish",
		})

	case "hotfix":
		ver := strings.TrimPrefix(strings.TrimPrefix(s.Current, "hotfix/v"), "hotfix/")
		label := fmt.Sprintf("Finish hotfix v%s", ver)
		if s.Dirty {
			label += dirtyNote
		}
		if s.HasDefaultRemote {
			high = append(high, action{
				Label:       "Push current branch",
				Tag:         "push",
				Recommended: curFlow != nil && !curFlow.HasRemote && !s.Dirty,
				Command:     "gitflow push",
			})
		}
		high = append(high, action{
			Label: label, Tag: "finish",
			Recommended: !s.Dirty, Command: "gitflow finish",
		})
	}

	// T3 NORMAL: pull, start work, diffs
	if s.MainAheadOfDevelop > 0 {
		normal = append(normal, action{Label: "Pull deferred: fix backmerge first", Tag: "pull"})
	} else if s.HasDefaultRemote {
		normal = append(normal, action{Label: "Pull latest (safe fetch + merge)", Tag: "pull", Command: "gitflow pull"})
	} else {
		normal = append(normal, action{Label: fmt.Sprintf("Pull disabled (no '%s' remote configured)", cfg.Remote), Tag: "pull"})
	}
	hasReleasableDiff := s.DevelopAheadOfMain > 0 && len(s.DevelopOnlyFiles) > 0

	if s.DevelopAheadOfMain > 0 {
		normal = append(normal, action{
			Label:   fmt.Sprintf("View diff: %s vs %s (%d file(s))", cfg.DevelopBranch, cfg.MainBranch, len(s.DevelopOnlyFiles)),
			Tag:     "diff",
			Command: fmt.Sprintf("git diff --stat %s...%s", cfg.MainBranch, cfg.DevelopBranch),
		})
	}

	switch {
	case btype == "base" && s.Current == cfg.DevelopBranch:
		if len(s.Releases) > 0 {
			var candidate *state.BranchInfo
			for i := range s.Releases {
				rel := &s.Releases[i]
				tagName := cfg.TagPrefix + rel.ShortName
				if !git.TagExists(tagName) {
					candidate = rel
					break
				}
			}
			if candidate != nil {
				normal = append(normal, action{
					Label: fmt.Sprintf("Switch to release '%s' and finish it", candidate.Name), Tag: "finish",
					Recommended: true, Command: fmt.Sprintf("git checkout %s && gitflow finish", candidate.Name),
				})
			}
		}
		if len(s.Releases) == 0 {
			normal = append(normal, action{
				Label: "Start a new feature", Tag: "start", Recommended: true,
				NeedsInput: true, InputPrompt: "Feature name:", Command: "gitflow start feature %s",
			})
		}
		normal = append(normal, action{
			Label: "Start a bugfix", Tag: "start",
			NeedsInput: true, InputPrompt: "Bugfix name:", Command: "gitflow start bugfix %s",
		})
		if hasReleasableDiff && len(s.Releases) == 0 {
			normal = append(normal, action{
				Label:       fmt.Sprintf("Start a release (%d unreleased commit(s))", s.DevelopAheadOfMain),
				Tag:         "release",
				Recommended: true,
				Command:     "gitflow start release auto",
			})
		}

	case btype == "base" && s.Current == cfg.MainBranch:
		normal = append(normal, action{
			Label: "Start a hotfix (urgent)", Tag: "hotfix",
			Command: "gitflow start hotfix auto",
		})
		normal = append(normal, action{
			Label: fmt.Sprintf("Switch to %s", cfg.DevelopBranch), Tag: "switch",
			Recommended: true, Command: "git checkout " + cfg.DevelopBranch,
		})

	case btype == "feature":
		normal = append(normal, action{
			Label: "Start a new feature", Tag: "start",
			NeedsInput: true, InputPrompt: "Feature name:", Command: "gitflow start feature %s",
		})
	}

	// Recommend release from any branch when develop is ahead and no release exists
	if hasReleasableDiff && len(s.Releases) == 0 && !hasTag(normal, "release") {
		normal = append(normal, action{
			Label:       fmt.Sprintf("Start a release (%d unreleased commit(s))", s.DevelopAheadOfMain),
			Tag:         "release",
			Recommended: btype == "base" && s.Current == cfg.DevelopBranch,
			Command:     "gitflow start release auto",
		})
	}

	// T4 LOW: switch branches
	if s.Current != cfg.DevelopBranch {
		if !hasTagAndLabel(normal, "switch", cfg.DevelopBranch) {
			low = append(low, action{Label: fmt.Sprintf("Switch to %s", cfg.DevelopBranch), Tag: "switch", Command: "git checkout " + cfg.DevelopBranch})
		}
	}
	if s.Current != cfg.MainBranch {
		low = append(low, action{Label: fmt.Sprintf("Switch to %s", cfg.MainBranch), Tag: "switch", Command: "git checkout " + cfg.MainBranch})
	}
	for _, b := range s.Features {
		if b.Name != s.Current {
			low = append(low, action{Label: fmt.Sprintf("Switch to feature '%s'", b.ShortName), Tag: "switch", Command: "git checkout " + b.Name})
		}
	}
	for _, b := range s.Bugfixes {
		if b.Name != s.Current {
			low = append(low, action{Label: fmt.Sprintf("Switch to bugfix '%s'", b.ShortName), Tag: "switch", Command: "git checkout " + b.Name})
		}
	}
	for _, b := range s.Releases {
		if b.Name != s.Current {
			low = append(low, action{Label: fmt.Sprintf("Switch to release '%s'", b.ShortName), Tag: "switch", Command: "git checkout " + b.Name})
		}
	}
	for _, b := range s.Hotfixes {
		if b.Name != s.Current {
			low = append(low, action{Label: fmt.Sprintf("Switch to hotfix '%s'", b.ShortName), Tag: "switch", Command: "git checkout " + b.Name})
		}
	}

	// Fallback start actions
	if btype != "release" && btype != "hotfix" && len(s.Releases) == 0 {
		if !hasTag(low, "release") && !hasTag(normal, "release") {
			low = append(low, action{
				Label: "Start a release", Tag: "release",
				Command: "gitflow start release auto",
			})
		}
	}
	if btype != "hotfix" && len(s.Hotfixes) == 0 {
		if !hasTag(low, "hotfix") && !hasTag(normal, "hotfix") {
			low = append(low, action{
				Label: "Start a hotfix (urgent)", Tag: "hotfix",
				Command: "gitflow start hotfix auto",
			})
		}
	}
	if !hasTagAndLabel(normal, "start", "feature") && !hasTagAndLabel(low, "start", "feature") {
		low = append(low, action{
			Label: "Start a new feature", Tag: "start",
			NeedsInput: true, InputPrompt: "Feature name:", Command: "gitflow start feature %s",
		})
	}

	// Utilities
	low = append(low,
		action{Label: "List tags / releases", Tag: "tags", Command: "git tag --sort=-version:refname -n1"},
		action{Label: "View commit log", Tag: "log", Command: "gitflow log"},
		action{Label: "Repo health check", Tag: "health", Command: "gitflow health"},
		action{Label: "Clean up merged branches", Tag: "cleanup", Command: "gitflow cleanup"},
		action{Label: "Undo last operation", Tag: "undo", Command: "gitflow undo"},
		action{Label: "Exit", Tag: "exit"},
	)

	// Concatenate tiers while preserving gitflow phase priority.
	// Recommended actions are floated only within each tier.
	ordered := make([]action, 0, len(critical)+len(high)+len(normal)+len(low))
	ordered = appendTierWithPriority(ordered, critical)
	ordered = appendTierWithPriority(ordered, high)
	ordered = appendTierWithPriority(ordered, normal)
	ordered = appendTierWithPriority(ordered, low)
	return ordered
}

func suggestReleaseVersion(s state.RepoState) string {
	if s.Version != "" && s.Version != "0.0.0" {
		return strings.TrimPrefix(s.Version, "v")
	}

	tag := s.LastTag
	if tag == "" || tag == "none" {
		return "0.1.0"
	}
	tag = strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(tag, ".", 3)
	if len(parts) == 3 {
		if minor, err := strconv.Atoi(parts[1]); err == nil {
			return fmt.Sprintf("%s.%d.0", parts[0], minor+1)
		}
	}
	return tag
}
