package tui

import (
	"fmt"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/branch"
	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/state"
)

type dashLine struct {
	text  string
	style string // "normal", "error", "warn", "dim", "ok", "section", "feature", "bugfix", "release", "hotfix"
}

func buildDashboardLines(s state.RepoState, cfg config.FlowConfig) []dashLine {
	var lines []dashLine

	if s.MainAheadOfDevelop > 0 {
		lines = append(lines, dashLine{"", "normal"})
		lines = append(lines, dashLine{
			fmt.Sprintf(" ⚠  BRANCH DIVERGENCE: %s has %d commit(s) not in %s",
				cfg.MainBranch, s.MainAheadOfDevelop, cfg.DevelopBranch), "error"})
		lines = append(lines, dashLine{"    Run backmerge to restore the gitflow invariant.", "dim"})
		if len(s.MainOnlyFiles) > 0 {
			lines = append(lines, dashLine{"", "normal"})
			lines = append(lines, dashLine{fmt.Sprintf("    Files in %s missing from %s:", cfg.MainBranch, cfg.DevelopBranch), "dim"})
			limit := 8
			if len(s.MainOnlyFiles) < limit {
				limit = len(s.MainOnlyFiles)
			}
			for _, f := range s.MainOnlyFiles[:limit] {
				lines = append(lines, dashLine{"      ! " + f, "error"})
			}
			if len(s.MainOnlyFiles) > 8 {
				lines = append(lines, dashLine{fmt.Sprintf("      ... and %d more", len(s.MainOnlyFiles)-8), "dim"})
			}
		}
	}

	if s.Merge.InMerge {
		n := len(s.Merge.ConflictedFiles)
		lines = append(lines, dashLine{"", "normal"})
		lines = append(lines, dashLine{fmt.Sprintf(" ⚠  MERGE CONFLICT — %d file(s) need resolution", n), "error"})
		if s.Merge.OperationType != "" {
			lines = append(lines, dashLine{fmt.Sprintf("    During: %s finish v%s",
				s.Merge.OperationType, s.Merge.OperationVersion), "dim"})
		}
		limit := 8
		if len(s.Merge.ConflictedFiles) < limit {
			limit = len(s.Merge.ConflictedFiles)
		}
		for _, f := range s.Merge.ConflictedFiles[:limit] {
			lines = append(lines, dashLine{"      ✗ " + f, "error"})
		}
	}

	if s.DevelopAheadOfMain > 0 && s.MainAheadOfDevelop == 0 {
		lines = append(lines, dashLine{"", "normal"})
		lines = append(lines, dashLine{
			fmt.Sprintf(" %s is %d commit(s) ahead of %s", cfg.DevelopBranch, s.DevelopAheadOfMain, cfg.MainBranch), "feature"})
		if len(s.DevelopOnlyFiles) > 0 {
			lines = append(lines, dashLine{"    Unreleased files:", "dim"})
			limit := 6
			if len(s.DevelopOnlyFiles) < limit {
				limit = len(s.DevelopOnlyFiles)
			}
			for _, f := range s.DevelopOnlyFiles[:limit] {
				lines = append(lines, dashLine{"      + " + f, "feature"})
			}
			if len(s.DevelopOnlyFiles) > 6 {
				lines = append(lines, dashLine{fmt.Sprintf("      ... and %d more", len(s.DevelopOnlyFiles)-6), "dim"})
			}
		} else {
			lines = append(lines, dashLine{"    No unreleased file changes (metadata-only commit).", "dim"})
		}
	}

	type branchGroup struct {
		label    string
		branches []state.BranchInfo
		style    string
	}
	groups := []branchGroup{
		{"Features", s.Features, "feature"},
		{"Bugfixes", s.Bugfixes, "bugfix"},
		{"Releases", s.Releases, "release"},
		{"Hotfixes", s.Hotfixes, "hotfix"},
	}

	anyInflight := false
	for _, g := range groups {
		if len(g.branches) > 0 {
			anyInflight = true
			lines = append(lines, dashLine{"", "normal"})
			lines = append(lines, dashLine{" " + g.label + " in flight:", "section"})
			for _, b := range g.branches {
				remote := ""
				if b.HasRemote {
					remote = " (pushed)"
				}
				commits := fmt.Sprintf("%d commit(s)", b.CommitsAhead)
				if b.CommitsAhead == 0 {
					commits = "no commits"
				}
				lines = append(lines, dashLine{
					fmt.Sprintf("   ● %s — %s%s", b.Name, commits, remote), g.style})
			}
		}
	}
	if !anyInflight && !s.Merge.InMerge {
		lines = append(lines, dashLine{"", "normal"})
		lines = append(lines, dashLine{" No feature, bugfix, release, or hotfix branches in flight.", "dim"})
	}

	lines = append(lines, dashLine{"", "normal"})
	lines = append(lines, dashLine{strings.Repeat("─", 55), "dim"})
	lines = append(lines, dashLine{" Phase analysis:", "section"})
	lines = append(lines, dashLine{"", "normal"})

	btype := branch.TypeOf(s.Current)
	switch {
	case s.Merge.InMerge && len(s.Merge.ConflictedFiles) > 0:
		if s.Merge.OperationType != "" {
			lines = append(lines, dashLine{fmt.Sprintf("   Merge conflict during %s finish v%s.",
				s.Merge.OperationType, s.Merge.OperationVersion), "error"})
		} else {
			lines = append(lines, dashLine{fmt.Sprintf("   Merge conflict with %d file(s).", len(s.Merge.ConflictedFiles)), "error"})
		}
	case btype == "feature":
		name := strings.TrimPrefix(s.Current, "feature/")
		var bi *state.BranchInfo
		for i := range s.Features {
			if s.Features[i].ShortName == name {
				bi = &s.Features[i]
				break
			}
		}
		commits := 0
		if bi != nil {
			commits = bi.CommitsAhead
		}
		lines = append(lines, dashLine{fmt.Sprintf("   Feature: %s", name), "feature"})
		if commits == 0 {
			lines = append(lines, dashLine{"   No new commits yet.", "dim"})
		} else {
			lines = append(lines, dashLine{fmt.Sprintf("   %d commit(s) ready to merge.", commits), "ok"})
		}
	case btype == "bugfix":
		name := strings.TrimPrefix(s.Current, "bugfix/")
		lines = append(lines, dashLine{fmt.Sprintf("   Bugfix: %s", name), "bugfix"})
	case btype == "release":
		ver := strings.TrimPrefix(strings.TrimPrefix(s.Current, "release/v"), "release/")
		lines = append(lines, dashLine{fmt.Sprintf("   Release v%s — stabilization phase.", ver), "release"})
		if s.Dirty {
			lines = append(lines, dashLine{"   Uncommitted changes. Commit before finishing.", "warn"})
		} else {
			lines = append(lines, dashLine{"   Ready to finish. This tags and merges to main + develop.", "ok"})
		}
	case btype == "hotfix":
		ver := strings.TrimPrefix(strings.TrimPrefix(s.Current, "hotfix/v"), "hotfix/")
		lines = append(lines, dashLine{fmt.Sprintf("   Hotfix v%s — urgent production fix.", ver), "hotfix"})
		if s.Dirty {
			lines = append(lines, dashLine{"   Uncommitted changes. Commit your fix first.", "warn"})
		} else {
			lines = append(lines, dashLine{"   Ready to finish.", "ok"})
		}
	case s.Current == cfg.DevelopBranch:
		lines = append(lines, dashLine{"   Integration branch (develop).", "feature"})
		if s.MainAheadOfDevelop > 0 {
			lines = append(lines, dashLine{"   CRITICAL: backmerge required before any work.", "error"})
		} else if s.DevelopAheadOfMain > 0 && len(s.DevelopOnlyFiles) > 0 {
			lines = append(lines, dashLine{fmt.Sprintf("   %d unreleased commit(s). Consider a release.", s.DevelopAheadOfMain), "ok"})
		} else if s.DevelopAheadOfMain > 0 {
			lines = append(lines, dashLine{"   Ahead by metadata-only commit; no release needed.", "dim"})
		} else {
			lines = append(lines, dashLine{fmt.Sprintf("   Up to date with %s.", cfg.MainBranch), "dim"})
		}
	case s.Current == cfg.MainBranch:
		lines = append(lines, dashLine{"   Production branch (main). Do not commit directly.", "hotfix"})
		lines = append(lines, dashLine{"   Switch to develop to start work.", "dim"})
	default:
		lines = append(lines, dashLine{fmt.Sprintf("   Branch '%s' is not a standard gitflow branch.", s.Current), "dim"})
	}

	if s.Dirty {
		lines = append(lines, dashLine{"", "normal"})
		lines = append(lines, dashLine{fmt.Sprintf(" ⚠  Dirty working tree (%d file(s))", s.UncommittedCount), "warn"})
		lines = append(lines, dashLine{"    Commit or stash before start/finish operations.", "dim"})
	}

	if len(s.Releases) > 0 && btype != "release" && !s.Merge.InMerge {
		lines = append(lines, dashLine{"", "normal"})
		lines = append(lines, dashLine{fmt.Sprintf(" ⚠  Release '%s' is open. Finish it first.", s.Releases[0].Name), "warn"})
	}
	if len(s.Hotfixes) > 0 && btype != "hotfix" && !s.Merge.InMerge {
		lines = append(lines, dashLine{fmt.Sprintf(" ⚠  Hotfix '%s' is open. Finish it quickly.", s.Hotfixes[0].Name), "hotfix"})
	}

	return lines
}
