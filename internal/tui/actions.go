package tui

import (
	"fmt"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/state"
)

type action struct {
	Label       string
	Tag         string
	Recommended bool
	Command     string // shell command to run when selected
}

func buildActions(s state.RepoState, cfg config.FlowConfig) []action {
	btype := git.BranchTypeOf(s.Current)
	ms := s.Merge
	var actions []action

	if ms.InMerge && len(ms.ConflictedFiles) > 0 {
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

	if ms.InMerge && len(ms.ConflictedFiles) == 0 && ms.OperationType != "" {
		actions = append(actions, action{
			Label:       fmt.Sprintf("Continue %s finish v%s", ms.OperationType, ms.OperationVersion),
			Tag:         "continue",
			Recommended: true,
			Command:     "gitflow finish",
		})
		actions = append(actions, action{Label: "Exit", Tag: "exit"})
		return actions
	}

	actions = append(actions, action{Label: "Pull latest (safe fetch + merge)", Tag: "pull", Command: "gitflow pull"})

	if s.MainAheadOfDevelop > 0 {
		actions = append(actions, action{
			Label:       fmt.Sprintf("Back-merge %s into %s (%d commit(s) behind)", cfg.MainBranch, cfg.DevelopBranch, s.MainAheadOfDevelop),
			Tag:         "backmerge",
			Recommended: true,
			Command:     "gitflow backmerge",
		})
	}

	if !s.GitFlowInitialized {
		actions = append(actions, action{
			Label:       "Initialize git-flow",
			Tag:         "init",
			Recommended: true,
			Command:     "gitflow init",
		})
	}

	switch btype {
	case "feature":
		name := strings.TrimPrefix(s.Current, "feature/")
		var bi *state.BranchInfo
		for i := range s.Features {
			if s.Features[i].ShortName == name {
				bi = &s.Features[i]
				break
			}
		}
		hasWork := bi != nil && bi.CommitsAhead > 0 && !s.Dirty
		actions = append(actions, action{
			Label: fmt.Sprintf("Finish feature '%s'", name), Tag: "finish",
			Recommended: hasWork, Command: "gitflow finish",
		})
		actions = append(actions, action{Label: fmt.Sprintf("Sync with %s", cfg.DevelopBranch), Tag: "sync", Command: "gitflow sync"})

	case "bugfix":
		name := strings.TrimPrefix(s.Current, "bugfix/")
		var bi *state.BranchInfo
		for i := range s.Bugfixes {
			if s.Bugfixes[i].ShortName == name {
				bi = &s.Bugfixes[i]
				break
			}
		}
		hasWork := bi != nil && bi.CommitsAhead > 0 && !s.Dirty
		actions = append(actions, action{
			Label: fmt.Sprintf("Finish bugfix '%s'", name), Tag: "finish",
			Recommended: hasWork, Command: "gitflow finish",
		})
		actions = append(actions, action{Label: fmt.Sprintf("Sync with %s", cfg.DevelopBranch), Tag: "sync", Command: "gitflow sync"})

	case "release":
		ver := strings.TrimPrefix(strings.TrimPrefix(s.Current, "release/v"), "release/")
		actions = append(actions, action{
			Label: fmt.Sprintf("Finish release v%s", ver), Tag: "finish",
			Recommended: !s.Dirty, Command: "gitflow finish",
		})

	case "hotfix":
		ver := strings.TrimPrefix(strings.TrimPrefix(s.Current, "hotfix/v"), "hotfix/")
		actions = append(actions, action{
			Label: fmt.Sprintf("Finish hotfix v%s", ver), Tag: "finish",
			Recommended: !s.Dirty, Command: "gitflow finish",
		})

	default:
		if s.Current == cfg.DevelopBranch {
			if len(s.Releases) > 0 {
				rel := s.Releases[0]
				actions = append(actions, action{
					Label: fmt.Sprintf("Switch to release '%s' and finish it", rel.Name), Tag: "finish",
					Recommended: true, Command: fmt.Sprintf("git checkout %s && gitflow finish", rel.Name),
				})
			}
			if len(s.Releases) == 0 {
				actions = append(actions, action{Label: "Start a new feature", Tag: "start", Recommended: true})
			}
			actions = append(actions, action{Label: "Start a bugfix", Tag: "start"})
			if s.DevelopAheadOfMain > 0 && len(s.Releases) == 0 {
				actions = append(actions, action{Label: "Start a release", Tag: "release", Recommended: true})
			}
		} else if s.Current == cfg.MainBranch {
			actions = append(actions, action{Label: "Start a hotfix (urgent)", Tag: "hotfix"})
			actions = append(actions, action{
				Label: fmt.Sprintf("Switch to %s", cfg.DevelopBranch), Tag: "switch",
				Recommended: true, Command: "git checkout " + cfg.DevelopBranch,
			})
		}
	}

	// Switch actions
	if s.Current != cfg.DevelopBranch {
		actions = append(actions, action{Label: fmt.Sprintf("Switch to %s", cfg.DevelopBranch), Tag: "switch", Command: "git checkout " + cfg.DevelopBranch})
	}
	if s.Current != cfg.MainBranch {
		actions = append(actions, action{Label: fmt.Sprintf("Switch to %s", cfg.MainBranch), Tag: "switch", Command: "git checkout " + cfg.MainBranch})
	}
	for _, b := range s.Features {
		if b.Name != s.Current {
			actions = append(actions, action{Label: fmt.Sprintf("Switch to feature '%s'", b.ShortName), Tag: "switch", Command: "git checkout " + b.Name})
		}
	}
	for _, b := range s.Bugfixes {
		if b.Name != s.Current {
			actions = append(actions, action{Label: fmt.Sprintf("Switch to bugfix '%s'", b.ShortName), Tag: "switch", Command: "git checkout " + b.Name})
		}
	}

	actions = append(actions, action{Label: "Clean up merged branches", Tag: "cleanup", Command: "gitflow cleanup"})
	actions = append(actions, action{Label: "View commit log", Tag: "log", Command: "gitflow log"})
	actions = append(actions, action{Label: "Repo health check", Tag: "health", Command: "gitflow health"})
	actions = append(actions, action{Label: "Undo last operation", Tag: "undo", Command: "gitflow undo"})
	actions = append(actions, action{Label: "Exit", Tag: "exit"})

	return actions
}
