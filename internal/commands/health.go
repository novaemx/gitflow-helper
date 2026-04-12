package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Comprehensive repo health check",
		RunE: func(cmd *cobra.Command, args []string) error {
			var issues, warnings, okItems []string

			gitVer := git.RunQuiet("git --version")
			if gitVer == "" {
				issues = append(issues, "git is not installed or not in PATH")
			} else {
				okItems = append(okItems, "git: "+strings.Replace(gitVer, "git version ", "", 1))
			}

			if !git.IsGitFlowInitialized() {
				issues = append(issues, "gitflow not initialized — missing main+develop branches (run: gitflow init)")
			} else {
				okItems = append(okItems, "gitflow structure: main + develop branches present")
			}

			allLocal := git.AllLocalBranches()
			localSet := make(map[string]bool)
			for _, b := range allLocal {
				localSet[b] = true
			}
			if !localSet[Cfg.DevelopBranch] {
				issues = append(issues, fmt.Sprintf("'%s' branch missing", Cfg.DevelopBranch))
			}
			if !localSet[Cfg.MainBranch] {
				issues = append(issues, fmt.Sprintf("'%s' branch missing", Cfg.MainBranch))
			}

			fetchCode, _, _ := git.RunResult(fmt.Sprintf("git ls-remote --exit-code %s HEAD", Cfg.Remote))
			if fetchCode != 0 {
				warnings = append(warnings, fmt.Sprintf("Remote '%s' unreachable", Cfg.Remote))
			} else {
				okItems = append(okItems, fmt.Sprintf("remote '%s' reachable", Cfg.Remote))
			}

			if localSet[Cfg.DevelopBranch] && localSet[Cfg.MainBranch] {
				mainAhead := git.RunQuiet(fmt.Sprintf("git rev-list --count %s..%s", Cfg.DevelopBranch, Cfg.MainBranch))
				n, _ := strconv.Atoi(mainAhead)
				if n > 0 {
					files := git.RunLines(fmt.Sprintf("git diff --name-only %s...%s", Cfg.DevelopBranch, Cfg.MainBranch))
					issues = append(issues, fmt.Sprintf("%s is %d commit(s) ahead of %s (%d file(s)) — run backmerge",
						Cfg.MainBranch, n, Cfg.DevelopBranch, len(files)))
				} else {
					okItems = append(okItems, fmt.Sprintf("%s contains all of %s", Cfg.DevelopBranch, Cfg.MainBranch))
				}
			}

			for _, branch := range []string{Cfg.MainBranch, Cfg.DevelopBranch} {
				if localSet[branch] {
					unpushed := git.RunQuiet(fmt.Sprintf("git rev-list --count %s/%s..%s 2>/dev/null", Cfg.Remote, branch, branch))
					n, _ := strconv.Atoi(unpushed)
					if n > 0 {
						warnings = append(warnings, fmt.Sprintf("'%s' has %d unpushed commit(s)", branch, n))
					} else {
						okItems = append(okItems, fmt.Sprintf("'%s' up to date with remote", branch))
					}
				}
			}

			for _, b := range allLocal {
				if strings.HasPrefix(b, "feature/") || strings.HasPrefix(b, "bugfix/") ||
					strings.HasPrefix(b, "release/") || strings.HasPrefix(b, "hotfix/") {
					ts := git.RunQuiet(fmt.Sprintf("git log -1 --format=%%ct %s 2>/dev/null", b))
					if epoch, err := strconv.ParseInt(ts, 10, 64); err == nil {
						ageDays := int(time.Since(time.Unix(epoch, 0)).Hours() / 24)
						if ageDays > 30 {
							warnings = append(warnings, fmt.Sprintf("stale branch: %s (inactive %d days)", b, ageDays))
						}
					}
				}
			}

			dirtyCount := len(git.RunLines("git status --porcelain"))
			if dirtyCount > 0 {
				warnings = append(warnings, fmt.Sprintf("%d uncommitted file(s) in working tree", dirtyCount))
			}

			if output.IsJSONMode() {
				output.JSONOutput(map[string]any{
					"action":   "health",
					"issues":   issues,
					"warnings": warnings,
					"ok":       okItems,
					"healthy":  len(issues) == 0,
				})
				if len(issues) > 0 {
					os.Exit(1)
				}
				return nil
			}

			output.Infof("\n  %s╔═══════════════════════════════════════════════════╗", output.Bold)
			output.Infof("  ║              Repository Health Check              ║")
			output.Infof("  ╚═══════════════════════════════════════════════════╝%s\n", output.Reset)

			for _, item := range okItems {
				output.Infof("    %s✓%s %s", output.Green, output.Reset, item)
			}
			for _, w := range warnings {
				output.Infof("    %s⚠%s %s", output.Yellow, output.Reset, w)
			}
			for _, iss := range issues {
				output.Infof("    %s✗%s %s", output.Red, output.Reset, iss)
			}

			if len(issues) == 0 && len(warnings) == 0 {
				output.Infof("\n  %s%sAll checks passed — repo is healthy!%s", output.Green, output.Bold, output.Reset)
			} else if len(issues) == 0 {
				output.Infof("\n  %sNo critical issues, %d warning(s).%s", output.Yellow, len(warnings), output.Reset)
			} else {
				output.Infof("\n  %s%d issue(s) need attention.%s", output.Red, len(issues), output.Reset)
				os.Exit(1)
			}
			return nil
		},
	}
}
