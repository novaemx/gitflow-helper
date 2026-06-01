package commands

import (
	"github.com/novaemx/gitflow-helper/internal/ide"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var (
		forceIDE  string
		autoYes   bool
		forceRW   bool
		checkOnly bool
		uninstall bool
	)
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Detect IDE and install gitflow rules, MCP config, and embedded skill",
		Long: `Detects which IDE is running, generates the appropriate rule/instruction files
for gitflow preflight enforcement, and installs or updates the embedded gitflow
skill in the project or ~/.agents fallback.

Flags:
  -y, --yes        Auto-accept the AI integration consent dialog (for non-interactive
                   agents and CI). Skips the interactive prompt.
      --force      Re-write all rule files and skills unconditionally, even if the
                   on-disk content already matches the expected byte stream. Useful
                   for template migration testing.
      --check      Dry run. Compute what files would be created or updated without
                   writing to disk. Prints the would-be file list.
      --uninstall  Remove all installed artifacts (IDE rules, MCP config, project
                   skill, AGENTS.md) and clear the consent entry from
                   .gitflow/config.json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			detected := ide.DetectedIDE{ID: forceIDE, DisplayName: forceIDE}
			if detected.ID == "" {
				detected = ide.DetectPrimary(GF.Config.ProjectRoot)
			}

			output.Infof("  Detected IDE: %s%s%s", output.Cyan, detected.ID, output.Reset)

			// --uninstall is a separate path: remove all artifacts and clear
			// consent. It does NOT consult the consent dialog.
			if uninstall {
				removed, err := ide.RemoveRulesForIDE(GF.Config.ProjectRoot, detected)
				if err != nil {
					output.Infof("  %sError removing files: %v%s", output.Red, err, output.Reset)
					if output.IsJSONMode() {
						output.JSONOutput(map[string]any{"action": "setup", "result": "error", "error": err.Error()})
					}
					return err
				}
				if err := ide.RemoveConsent(GF.Config.ProjectRoot); err != nil {
					output.Infof("  %sWarning: could not clear consent: %v%s", output.Yellow, err, output.Reset)
				}
				for _, f := range removed {
					output.Infof("  %s✗%s Removed %s", output.Red, output.Reset, f)
				}
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{
						"action":       "setup",
						"result":       "ok",
						"ide_detected": detected.ID,
						"mode":         "uninstall",
						"files":        removed,
					})
				}
				return nil
			}

			// Wire the --force flag into the package-level var that
			// generateCursorRule checks. The skill generator already accepts
			// a force param via ensureEmbeddedSkillWithForce, called from
			// ensureRulesForSingleIDE when force is in effect.
			if forceRW {
				ide.SetForceRuleWrite(true)
				defer ide.SetForceRuleWrite(false)
			}

			opts := ide.EnsureOptions{
				AutoAccept: autoYes,
				Force:      forceRW,
				CheckOnly:  checkOnly,
			}

			files, err := ide.EnsureRulesWithAIConsent(GF.Config.ProjectRoot, detected, !output.IsJSONMode(), GF.AppVersion, opts)
			if err != nil {
				output.Infof("  %sError generating files: %v%s", output.Red, err, output.Reset)
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "setup", "result": "error", "error": err.Error()})
				}
				return err
			}

			verb := "Created"
			if checkOnly {
				verb = "Would create"
			}
			for _, f := range files {
				output.Infof("  %s✓%s %s %s", output.Green, output.Reset, verb, f)
			}

			if output.IsJSONMode() {
				result := map[string]any{
					"action":       "setup",
					"result":       "ok",
					"ide_detected": detected.ID,
					"files":        files,
				}
				if checkOnly {
					result["mode"] = "check"
				}
				if forceRW {
					result["force"] = true
				}
				if autoYes {
					result["auto_accept"] = true
				}
				output.JSONOutput(result)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&forceIDE, "ide", "", "Force IDE type (cursor, copilot, both, claude-code, windsurf, cline, zed, neovim, jetbrains)")
	cmd.Flags().BoolVarP(&autoYes, "yes", "y", false, "Auto-accept the AI integration consent dialog (non-interactive / CI)")
	cmd.Flags().BoolVarP(&forceRW, "force", "f", false, "Re-write rule files and skills even when content matches the expected byte stream")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Dry run: report what would be created or updated without writing to disk")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Remove all installed artifacts and clear the consent entry")
	return cmd
}
