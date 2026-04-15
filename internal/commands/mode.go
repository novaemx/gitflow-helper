package commands

import (
	"fmt"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mode [local|pr|toggle]",
		Short: "Get or set gitflow integration mode (local merge vs pull request)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				mode := config.NormalizeIntegrationMode(GF.Config.IntegrationMode)
				if mode == "" {
					mode = config.IntegrationModeLocalMerge
				}
				result := map[string]any{
					"action":              "mode",
					"result":              "ok",
					"integration_mode":    mode,
					"integration_display": config.IntegrationModeDisplay(mode),
				}
				if output.IsJSONMode() {
					output.JSONOutput(result)
					return nil
				}
				output.Infof("  Integration mode: %s%s%s", output.Cyan, config.IntegrationModeDisplay(mode), output.Reset)
				output.Infof("  Available: local-merge, pull-request")
				return nil
			}

			raw := strings.ToLower(strings.TrimSpace(args[0]))
			next := raw
			if raw == "toggle" {
				current := config.NormalizeIntegrationMode(GF.Config.IntegrationMode)
				if current == config.IntegrationModePullRequest {
					next = config.IntegrationModeLocalMerge
				} else {
					next = config.IntegrationModePullRequest
				}
			}

			normalized := config.NormalizeIntegrationMode(next)
			if normalized == "" {
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{
						"action": "mode",
						"result": "error",
						"error":  "invalid mode; use local|pr|toggle",
					})
				}
				return fmt.Errorf("invalid mode %q (expected local, pr, or toggle)", raw)
			}

			if err := config.SetIntegrationMode(GF.Config.ProjectRoot, normalized); err != nil {
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{"action": "mode", "result": "error", "error": err.Error()})
				}
				return err
			}

			GF.Config.IntegrationMode = normalized
			GF.Config.ModeConfigured = true

			result := map[string]any{
				"action":              "mode",
				"result":              "ok",
				"integration_mode":    normalized,
				"integration_display": config.IntegrationModeDisplay(normalized),
			}
			if output.IsJSONMode() {
				output.JSONOutput(result)
				return nil
			}
			output.Infof("  %s✓%s Integration mode set to %s%s%s",
				output.Green, output.Reset, output.Cyan, config.IntegrationModeDisplay(normalized), output.Reset)
			return nil
		},
		SilenceUsage: true,
	}
}
