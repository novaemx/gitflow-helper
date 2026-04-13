package commands

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	var count int
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Gitflow-aware commit log with release boundaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			logFmt := "%H|%h|%s|%an|%ar|%D"
			entries := git.ExecLines("log", "--all", fmt.Sprintf("--format=%s", logFmt), "-n", fmt.Sprintf("%d", count))

			if len(entries) == 0 {
				output.Infof("  %sNo commits found.%s", output.Dim, output.Reset)
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "log", "entries": []any{}})
				}
				return nil
			}

			tagRe := regexp.MustCompile(`tag:\s*([^\s,)]+)`)
			type logEntry struct {
				SHA       string `json:"sha"`
				FullSHA   string `json:"full_sha"`
				Subject   string `json:"subject"`
				Author    string `json:"author"`
				Date      string `json:"date"`
				Refs      string `json:"refs"`
				Tag       string `json:"tag"`
				IsRelease bool   `json:"is_release"`
			}

			var parsed []logEntry
			for _, entry := range entries {
				parts := strings.SplitN(entry, "|", 6)
				if len(parts) < 6 {
					continue
				}
				e := logEntry{
					FullSHA: parts[0],
					SHA:     parts[1],
					Subject: parts[2],
					Author:  parts[3],
					Date:    parts[4],
					Refs:    strings.TrimSpace(parts[5]),
				}
				if e.Refs != "" && strings.Contains(e.Refs, "tag:") {
					m := tagRe.FindStringSubmatch(e.Refs)
					if len(m) > 1 {
						e.Tag = m[1]
						e.IsRelease = true
					}
				}
				parsed = append(parsed, e)
			}

			if output.IsJSONMode() {
				output.JSONOutput(map[string]any{"action": "log", "entries": parsed})
				return nil
			}

			output.Infof("\n  %sGitflow commit log (last %d):%s\n", output.Bold, count, output.Reset)
			for _, p := range parsed {
				if p.IsRelease {
					output.Infof("  %s", strings.Repeat("─", 55))
					output.Infof("  %s%s▼ RELEASE %s%s", output.Green, output.Bold, p.Tag, output.Reset)
				}

				prefix := ""
				subLower := strings.ToLower(p.Subject)
				switch {
				case strings.Contains(subLower, "merge branch 'feature/") || strings.HasPrefix(subLower, "feature"):
					prefix = output.Cyan + "[feature]" + output.Reset + " "
				case strings.Contains(subLower, "merge branch 'bugfix/") || strings.HasPrefix(subLower, "fix"):
					prefix = output.Yellow + "[bugfix]" + output.Reset + "  "
				case strings.Contains(subLower, "merge branch 'hotfix/") || strings.HasPrefix(subLower, "hotfix"):
					prefix = output.Red + "[hotfix]" + output.Reset + "  "
				case strings.Contains(subLower, "merge branch 'release/"):
					prefix = output.Green + "[release]" + output.Reset + " "
				case strings.Contains(subLower, "backmerge"):
					prefix = output.Magenta + "[sync]" + output.Reset + "    "
				}

				refsDisplay := ""
				if p.Refs != "" {
					refsDisplay = " " + output.Dim + "(" + p.Refs + ")" + output.Reset
				}

				output.Infof("  %s%s%s %s%s%s", output.Yellow, p.SHA, output.Reset, prefix, p.Subject, refsDisplay)
				output.Infof("         %s%s · %s%s", output.Dim, p.Author, p.Date, output.Reset)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&count, "count", "n", 20, "Number of entries")
	return cmd
}
