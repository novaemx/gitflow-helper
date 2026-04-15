package commands

import (
	"os"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Comprehensive repo health check",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := GF.HealthReport()

			if output.IsJSONMode() {
				output.JSONOutput(report.ToMap())
				if len(report.Issues) > 0 {
					os.Exit(1)
				}
				return nil
			}

			output.Infof("")
			output.Infof("  %sHealth Check%s", output.Bold, output.Reset)
			output.Infof("  %s", strings.Repeat("─", 40))
			output.Infof("")

			if len(report.OK) > 0 {
				output.Infof("  %sPassing:%s", output.Bold, output.Reset)
				for _, item := range report.OK {
					output.Infof("    %s✓%s %s", output.Green, output.Reset, item)
				}
				output.Infof("")
			}

			if len(report.Warnings) > 0 {
				output.Infof("  %sWarnings:%s", output.Bold, output.Reset)
				for _, w := range report.Warnings {
					output.Infof("    %s⚠%s %s", output.Yellow, output.Reset, w)
				}
				output.Infof("")
			}

			if len(report.Issues) > 0 {
				output.Infof("  %sIssues:%s", output.Bold, output.Reset)
				for _, iss := range report.Issues {
					output.Infof("    %s✗%s %s", output.Red, output.Reset, iss)
				}
				output.Infof("")
			}

			total := len(report.OK) + len(report.Warnings) + len(report.Issues)
			output.Infof("  %s─%s", output.Dim, strings.Repeat("─", 39))
			if len(report.Issues) == 0 && len(report.Warnings) == 0 {
				output.Infof("  %s✓ %d/%d checks passed%s", output.Green, total, total, output.Reset)
			} else if len(report.Issues) == 0 {
				output.Infof("  %s⚠ %d/%d passed, %d warning(s)%s", output.Yellow, len(report.OK), total, len(report.Warnings), output.Reset)
			} else {
				output.Infof("  %s✗ %d issue(s) need attention%s", output.Red, len(report.Issues), output.Reset)
				os.Exit(1)
			}
			return nil
		},
	}
}
