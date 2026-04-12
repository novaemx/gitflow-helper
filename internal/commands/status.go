package commands

import (
	"fmt"

	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/luis-lozano/gitflow-helper/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd *cobra.Command

func newStatusCmd() *cobra.Command {
	statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show repository state",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := state.DetectState(Cfg)
			if output.IsJSONMode() {
				output.JSONOutput(s)
				return nil
			}
			printDashboard(s)
			return nil
		},
	}
	return statusCmd
}

func printDashboard(s state.RepoState) {
	btype := git.BranchTypeOf(s.Current)
	tagDisplay := s.LastTag
	if tagDisplay == "none" {
		tagDisplay = "no tags yet"
	}

	gfStatus := "initialized"
	if !s.GitFlowInitialized {
		gfStatus = output.Yellow + "not initialized" + output.Reset
	}

	dirtyColor := output.Green
	if s.Dirty {
		dirtyColor = output.Yellow
	}

	output.Infof(`
  %s╔═══════════════════════════════════════════════════╗
  ║       Git Flow Helper                             ║
  ╚═══════════════════════════════════════════════════╝%s

  %sCurrent state:%s
    Branch:    %s%s%s
    Version:   %s%s%s
    Last tag:  %s
    Git Flow:  %s
    Uncommitted files: %s%d%s`,
		output.Bold, output.Reset,
		output.Bold, output.Reset,
		output.Cyan, s.Current, output.Reset,
		output.Green, s.Version, output.Reset,
		tagDisplay, gfStatus,
		dirtyColor, s.UncommittedCount, output.Reset)

	if s.DevelopAheadOfMain > 0 {
		output.Infof("    %s is %s%d%s commit(s) ahead of %s",
			Cfg.DevelopBranch, output.Yellow, s.DevelopAheadOfMain, output.Reset, Cfg.MainBranch)
	}
	if s.MainAheadOfDevelop > 0 {
		output.Infof("    %s%s is %d commit(s) ahead of %s%s",
			output.Red, Cfg.MainBranch, s.MainAheadOfDevelop, Cfg.DevelopBranch, output.Reset)
		output.Infof("    %s⚠  Branch divergence detected!%s", output.Red, output.Reset)
	}

	if s.Merge.InMerge {
		n := len(s.Merge.ConflictedFiles)
		output.Infof(`
  %s%s╔═══════════════════════════════════════════════════╗
  ║  ⚠  MERGE CONFLICT — %2d file(s) need resolution   ║
  ╚═══════════════════════════════════════════════════╝%s`,
			output.Red, output.Bold, n, output.Reset)
		for _, f := range s.Merge.ConflictedFiles {
			output.Infof("    %s✗%s %s", output.Red, output.Reset, f)
		}
	}

	type branchGroup struct {
		label    string
		branches []state.BranchInfo
		color    string
	}
	groups := []branchGroup{
		{"Features", s.Features, output.Cyan},
		{"Bugfixes", s.Bugfixes, output.Yellow},
		{"Releases", s.Releases, output.Green},
		{"Hotfixes", s.Hotfixes, output.Red},
	}

	anyInflight := false
	for _, g := range groups {
		if len(g.branches) > 0 {
			anyInflight = true
			output.Infof("\n  %s%s in flight:%s", output.Bold, g.label, output.Reset)
			for _, b := range g.branches {
				remote := ""
				if b.HasRemote {
					remote = fmt.Sprintf(" %s(pushed)%s", output.Dim, output.Reset)
				}
				commits := fmt.Sprintf("%d commit(s)", b.CommitsAhead)
				if b.CommitsAhead == 0 {
					commits = "no new commits"
				}
				output.Infof("    %s●%s %s  —  %s%s", g.color, output.Reset, b.Name, commits, remote)
			}
		}
	}
	if !anyInflight && !s.Merge.InMerge {
		output.Infof("\n  %sNo feature, bugfix, release, or hotfix branches in flight.%s", output.Dim, output.Reset)
	}

	output.Infof("\n  %s%s%s", output.Bold, "───────────────────────────────────────────────────", output.Reset)
	output.Infof("  %sPhase analysis:%s\n", output.Bold, output.Reset)

	switch {
	case s.Merge.InMerge && len(s.Merge.ConflictedFiles) > 0:
		output.Infof("    %sYou are in a merge conflict%s with %d file(s) to resolve.",
			output.Red, output.Reset, len(s.Merge.ConflictedFiles))
	case btype == "feature":
		output.Infof("    You are on feature %s'%s'%s.", output.Cyan, s.Current, output.Reset)
	case btype == "bugfix":
		output.Infof("    You are on bugfix %s'%s'%s.", output.Yellow, s.Current, output.Reset)
	case btype == "release":
		output.Infof("    You are on release branch %s%s%s.", output.Green, s.Current, output.Reset)
	case btype == "hotfix":
		output.Infof("    You are on hotfix %s%s%s.", output.Red, s.Current, output.Reset)
	case s.Current == Cfg.DevelopBranch:
		output.Infof("    You are on %s%s%s — the integration branch.", output.Cyan, Cfg.DevelopBranch, output.Reset)
		if s.MainAheadOfDevelop > 0 {
			output.Infof("    %sCRITICAL:%s Back-merge required before any work.", output.Red, output.Reset)
		}
	case s.Current == Cfg.MainBranch:
		output.Infof("    You are on %s%s%s — the production branch.", output.Bold, Cfg.MainBranch, output.Reset)
	default:
		output.Infof("    You are on '%s', not a standard git-flow branch.", s.Current)
	}
	output.Info("")
}
