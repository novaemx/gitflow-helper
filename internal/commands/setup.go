package commands

import (
	"github.com/novaemx/gitflow-helper/internal/ide"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var forceIDE string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Detect IDE and install gitflow rules, MCP config, and embedded skill",
		Long:  "Detects which IDE is running, generates the appropriate rule/instruction files for gitflow preflight enforcement, and installs or updates the embedded gitflow skill in the project or ~/.agents fallback.",
		RunE: func(cmd *cobra.Command, args []string) error {
			detected := forceIDE
			if detected == "" {
				detected = ide.DetectPrimary(GF.Config.ProjectRoot).ID
			}

			output.Infof("  Detected IDE: %s%s%s", output.Cyan, detected, output.Reset)

			files, err := ide.Generate(GF.Config.ProjectRoot, detected)
			if err != nil {
				output.Infof("  %sError generating files: %v%s", output.Red, err, output.Reset)
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "setup", "result": "error", "error": err.Error()})
				}
				return err
			}

			for _, f := range files {
				output.Infof("  %s✓%s Created %s", output.Green, output.Reset, f)
			}

			if output.IsJSONMode() {
				output.JSONOutput(map[string]any{
					"action":       "setup",
					"result":       "ok",
					"ide_detected": detected,
					"files":        files,
				})
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&forceIDE, "ide", "", "Force IDE type (cursor, copilot, both, claude-code, windsurf, cline, zed, neovim, jetbrains)")
	return cmd
}
