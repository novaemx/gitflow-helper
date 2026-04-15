package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	gfconfig "github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/debug"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	"github.com/novaemx/gitflow-helper/internal/ide"
	"github.com/novaemx/gitflow-helper/internal/mcp"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

var (
	jsonFlag bool
	GF       *gitflow.Logic
)

func logCLIActivity(cmd *cobra.Command, args []string) {
	if GF == nil || GF.Config.ProjectRoot == "" {
		return
	}
	tool := cmd.Name()
	if tool == "" {
		tool = "gitflow"
	}
	if cmd.Parent() == nil && len(args) == 0 {
		tool = "interactive-tui"
	}
	entry := mcp.ActivityEntry{
		Tool:   tool,
		Args:   strings.Join(args, " "),
		Result: "started",
		Source: "cli",
	}
	_ = mcp.AppendActivityLog(GF.Config.ProjectRoot, entry)
}

func isInteractiveTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func chooseIntegrationMode(defaultMode string) string {
	if defaultMode == "" {
		defaultMode = gfconfig.IntegrationModeLocalMerge
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\nSelect integration mode [1=local merge, 2=pull request] (default 1): ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return defaultMode
		}
		answer := strings.TrimSpace(strings.ToLower(line))
		switch answer {
		case "", "1", "local", "local-merge":
			return gfconfig.IntegrationModeLocalMerge
		case "2", "pr", "pull-request", "pull request":
			return gfconfig.IntegrationModePullRequest
		default:
			fmt.Println("Invalid option. Enter 1 or 2.")
		}
	}
}

func ensureIntegrationModeConfigured(cmd *cobra.Command) {
	if GF == nil || GF.Config.ModeConfigured {
		return
	}
	if output.IsJSONMode() {
		_ = gfconfig.SetIntegrationMode(GF.Config.ProjectRoot, GF.Config.IntegrationMode)
		GF.Config.ModeConfigured = true
		return
	}

	mode := GF.Config.IntegrationMode
	if isInteractiveTTY() && cmd.Name() != "mode" {
		mode = chooseIntegrationMode(mode)
	}

	normalized := gfconfig.NormalizeIntegrationMode(mode)
	if normalized == "" {
		normalized = gfconfig.IntegrationModeLocalMerge
	}
	_ = gfconfig.SetIntegrationMode(GF.Config.ProjectRoot, normalized)
	GF.Config.IntegrationMode = normalized
	GF.Config.ModeConfigured = true

	if cmd.Name() != "mode" {
		output.Infof("  %sIntegration mode:%s %s", output.Dim, output.Reset, gfconfig.IntegrationModeDisplay(normalized))
	}
}

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "gitflow",
		Short: "Git Flow helper — interactive TUI + CLI subcommands",
		Long:  "A comprehensive Git Flow workflow helper with an interactive TUI and CLI subcommands for agent and human use.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			deferTotal := debug.Start("root.PersistentPreRun.total")
			defer deferTotal()

			output.SetJSONMode(jsonFlag)

			deferNew := debug.Start("root.PersistentPreRun.gitflow.New")
			GF = gitflow.New("")
			deferNew()
			GF.AppVersion = version

			// Skip git checks for help/version/completion subcommands.
			name := cmd.Name()
			if name == "help" || name == "completion" {
				return
			}

			logCLIActivity(cmd, args)

			deferGitAvail := debug.Start("root.PersistentPreRun.IsGitAvailable")
			if !GF.IsGitAvailable() {
				deferGitAvail()
				fmt.Fprintln(os.Stderr, "fatal: git is not installed or not in PATH")
				os.Exit(1)
			}
			deferGitAvail()

			deferIsRepo := debug.Start("root.PersistentPreRun.IsGitRepo")
			if !GF.IsGitRepo() {
				deferIsRepo()
				if output.IsJSONMode() {
					output.JSONOutput(map[string]any{
						"error":   "not_a_git_repo",
						"cwd":     GF.Config.ProjectRoot,
						"message": "Run 'git init' first, then 'gitflow init'.",
					})
					os.Exit(1)
				}
				output.Infof("  %sNot inside a git repository.%s", output.Red, output.Reset)
				output.Infof("  Run %sgit init%s first, then %sgitflow init%s.", output.Bold, output.Reset, output.Bold, output.Reset)
				os.Exit(1)
			}
			deferIsRepo()

			// Auto-initialize gitflow structure if running a command that needs it
			if name != "doctor" && name != "health" && name != "setup" {
				deferInitCheck := debug.Start("root.PersistentPreRun.IsGitFlowInitialized")
				if !GF.IsGitFlowInitialized() {
					deferInitCheck()
					if !output.IsJSONMode() {
						output.Infof("  %sGitflow structure not detected. Auto-initializing...%s", output.Yellow, output.Reset)
					}
					deferInit := debug.Start("root.PersistentPreRun.GF.Init")
					ok, msg := GF.Init()
					deferInit()
					if !ok {
						if output.IsJSONMode() {
							output.JSONOutput(map[string]any{
								"error":   "init_failed",
								"message": msg,
							})
						}
						os.Exit(1)
					}
				} else {
					deferInitCheck()
				}
			}

			// Auto-provision IDE rules when missing (silent, idempotent)
			if name != "doctor" && name != "health" {
				deferEnsure := debug.Start("root.PersistentPreRun.GF.EnsureRules")
				_, _ = ide.EnsureRulesWithAIConsent(GF.Config.ProjectRoot, GF.IDE, !output.IsJSONMode())
				deferEnsure()
			}

			ensureIntegrationModeConfigured(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonFlag {
				return statusCmd.RunE(cmd, args)
			}
			return runTUI()
		},
		Version: version,
	}

	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Machine-readable JSON output (for agents)")

	root.AddCommand(
		newStatusCmd(),
		newPullCmd(),
		newPushCmd(),
		newInitCmd(),
		newStartCmd(),
		newFinishCmd(),
		newFastReleaseCmd(),
		newSyncCmd(),
		newSwitchCmd(),
		newBackmergeCmd(),
		newCleanupCmd(),
		newHealthCmd(),
		newDoctorCmd(),
		newLogCmd(),
		newUndoCmd(),
		newReleaseNotesCmd(),
		newModeCmd(),
		newSetupCmd(),
		newServeCmd(),
	)

	return root
}
