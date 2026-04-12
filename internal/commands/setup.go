package commands

import (
	"github.com/luis-lozano/gitflow-helper/internal/ide"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var forceIDE string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Detect IDE and generate gitflow preflight rules/instructions",
		Long:  "Detects whether you're in Cursor or VSCode (Copilot) and generates the appropriate rule/instruction files for gitflow preflight enforcement.",
		RunE: func(cmd *cobra.Command, args []string) error {
			detected := forceIDE
			if detected == "" {
				detected = ide.Detect(Cfg.ProjectRoot)
			}

			output.Infof("  Detected IDE: %s%s%s", output.Cyan, detected, output.Reset)

			files, err := ide.Generate(Cfg.ProjectRoot, detected)
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
	cmd.Flags().StringVar(&forceIDE, "ide", "", "Force IDE type (cursor, copilot, both)")
	return cmd
}
