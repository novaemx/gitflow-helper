package commands

import (
	"os"

	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

var startExitFunc = os.Exit

func startFailureLines(result map[string]any) []string {
	var lines []string
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		lines = append(lines, errMsg)
	}
	if hint, ok := result["hint"].(string); ok && hint != "" {
		lines = append(lines, "Hint: "+hint)
	}
	if diagnostics, ok := result["diagnostics"].([]string); ok && len(diagnostics) > 0 {
		lines = append(lines, "Diagnostics:")
		for _, diagnostic := range diagnostics {
			lines = append(lines, "- "+diagnostic)
		}
	}
	return lines
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <type> <name>",
		Short: "Start a feature/bugfix/release/hotfix",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, result := GF.Start(args[0], args[1])
			if output.IsJSONMode() {
				output.JSONOutput(result)
			}
			if code != 0 {
				if !output.IsJSONMode() {
					for _, line := range startFailureLines(result) {
						output.Infof("  %s", line)
					}
				}
				startExitFunc(code)
			}
			return nil
		},
	}
}
