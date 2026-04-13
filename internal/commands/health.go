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
			cfg := GF.Config
			var issues, warnings, okItems []string

			gitVer := git.ExecQuiet("--version")
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
			if !localSet[cfg.DevelopBranch] {
				issues = append(issues, fmt.Sprintf("'%s' branch missing", cfg.DevelopBranch))
			}
			if !localSet[cfg.MainBranch] {
				issues = append(issues, fmt.Sprintf("'%s' branch missing", cfg.MainBranch))
			}

			remoteExists := git.RemoteExists(cfg.Remote)
			if !remoteExists {
				warnings = append(warnings, fmt.Sprintf("Remote '%s' not configured — fix: git remote add %s <url>", cfg.Remote, cfg.Remote))
			} else {
				fetchCode, _, _ := git.ExecResult("ls-remote", "--exit-code", cfg.Remote, "HEAD")
				if fetchCode != 0 {
					warnings = append(warnings, fmt.Sprintf("Remote '%s' unreachable — fix: verify network/credentials or run 'git remote -v'", cfg.Remote))
				} else {
					okItems = append(okItems, fmt.Sprintf("remote '%s' reachable", cfg.Remote))
				}
			}

			if localSet[cfg.DevelopBranch] && localSet[cfg.MainBranch] {
				mainAhead := git.ExecQuiet("rev-list", "--count", cfg.DevelopBranch+".."+cfg.MainBranch)
				n, _ := strconv.Atoi(mainAhead)
				if n > 0 {
					files := git.ExecLines("diff", "--name-only", cfg.DevelopBranch+"..."+cfg.MainBranch)
					issues = append(issues, fmt.Sprintf("%s is %d commit(s) ahead of %s (%d file(s)) — run backmerge",
						cfg.MainBranch, n, cfg.DevelopBranch, len(files)))
				} else {
					okItems = append(okItems, fmt.Sprintf("%s contains all of %s", cfg.DevelopBranch, cfg.MainBranch))
				}
			}

			if remoteExists {
				for _, branch := range []string{cfg.MainBranch, cfg.DevelopBranch} {
					if localSet[branch] {
						unpushed := git.ExecQuiet("rev-list", "--count", cfg.Remote+"/"+branch+".."+branch)
						n, _ := strconv.Atoi(unpushed)
						if n > 0 {
							warnings = append(warnings, fmt.Sprintf("'%s' has %d unpushed commit(s) — fix: git push %s %s", branch, n, cfg.Remote, branch))
						} else {
							okItems = append(okItems, fmt.Sprintf("'%s' up to date with remote", branch))
						}
					}
				}
			}

			for _, b := range allLocal {
				if strings.HasPrefix(b, "feature/") || strings.HasPrefix(b, "bugfix/") ||
					strings.HasPrefix(b, "release/") || strings.HasPrefix(b, "hotfix/") {
					ts := git.ExecQuiet("log", "-1", "--format=%ct", b)
					if epoch, err := strconv.ParseInt(ts, 10, 64); err == nil {
						ageDays := int(time.Since(time.Unix(epoch, 0)).Hours() / 24)
						if ageDays > 30 {
							warnings = append(warnings, fmt.Sprintf("stale branch: %s (inactive %d days)", b, ageDays))
						}
						if ageDays > 14 {
							parentBehind := git.ExecQuiet("rev-list", "--count", b+".."+cfg.DevelopBranch)
							if pn, _ := strconv.Atoi(parentBehind); pn > 20 {
								warnings = append(warnings, fmt.Sprintf("merge-hell risk: %s is %d commits behind %s (inactive %d days)",
									b, pn, cfg.DevelopBranch, ageDays))
							}
						}
					}
				}
			}

			dirtyCount := len(git.ExecLines("status", "--porcelain"))
			if dirtyCount > 0 {
				warnings = append(warnings, fmt.Sprintf("%d uncommitted file(s) in working tree", dirtyCount))
			}

			okItems = append(okItems, fmt.Sprintf("IDE: %s", GF.IDEDisplay()))

			if output.IsJSONMode() {
				output.JSONOutput(map[string]any{
					"action":   "health",
					"issues":   issues,
					"warnings": warnings,
					"ok":       okItems,
					"healthy":  len(issues) == 0,
					"ide":      GF.IDE,
				})
				if len(issues) > 0 {
					os.Exit(1)
				}
				return nil
			}

			output.Infof("")
			output.Infof("  %sHealth Check%s", output.Bold, output.Reset)
			output.Infof("  %s", strings.Repeat("─", 40))
			output.Infof("")

			if len(okItems) > 0 {
				output.Infof("  %sPassing:%s", output.Bold, output.Reset)
				for _, item := range okItems {
					output.Infof("    %s✓%s %s", output.Green, output.Reset, item)
				}
				output.Infof("")
			}

			if len(warnings) > 0 {
				output.Infof("  %sWarnings:%s", output.Bold, output.Reset)
				for _, w := range warnings {
					output.Infof("    %s⚠%s %s", output.Yellow, output.Reset, w)
				}
				output.Infof("")
			}

			if len(issues) > 0 {
				output.Infof("  %sIssues:%s", output.Bold, output.Reset)
				for _, iss := range issues {
					output.Infof("    %s✗%s %s", output.Red, output.Reset, iss)
				}
				output.Infof("")
			}

			total := len(okItems) + len(warnings) + len(issues)
			output.Infof("  %s─%s", output.Dim, strings.Repeat("─", 39))
			if len(issues) == 0 && len(warnings) == 0 {
				output.Infof("  %s✓ %d/%d checks passed%s", output.Green, total, total, output.Reset)
			} else if len(issues) == 0 {
				output.Infof("  %s⚠ %d/%d passed, %d warning(s)%s", output.Yellow, len(okItems), total, len(warnings), output.Reset)
			} else {
				output.Infof("  %s✗ %d issue(s) need attention%s", output.Red, len(issues), output.Reset)
				os.Exit(1)
			}
			return nil
		},
	}
}
