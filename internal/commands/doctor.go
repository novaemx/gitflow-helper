package commands

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate prerequisites (git, branches, gitflow structure)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := GF.Config
			type check struct {
				Name  string `json:"name"`
				Value string `json:"value"`
				OK    bool   `json:"ok"`
			}
			var checks []check

			goVer := runtime.Version()
			checks = append(checks, check{"Go runtime", goVer, true})

			gitVer := git.ExecQuiet("--version")
			checks = append(checks, check{"git", strings.Replace(gitVer, "git version ", "", 1), gitVer != ""})

			_, err := os.Stat(fmt.Sprintf("%s/.git", cfg.ProjectRoot))
			inRepo := err == nil
			root := cfg.ProjectRoot
			if !inRepo {
				root = "NOT A REPO"
			}
			checks = append(checks, check{"git repo", root, inRepo})

			remotes := git.ExecLines("remote")
			hasRemote := false
			for _, r := range remotes {
				if r == cfg.Remote {
					hasRemote = true
					break
				}
			}
			remoteVal := cfg.Remote
			if !hasRemote {
				remoteVal = fmt.Sprintf("'%s' not found", cfg.Remote)
			}
			checks = append(checks, check{"remote", remoteVal, hasRemote})

			allBranches := git.AllLocalBranches()
			branchSet := make(map[string]bool)
			for _, b := range allBranches {
				branchSet[b] = true
			}
			hasMain := branchSet[cfg.MainBranch]
			hasDev := branchSet[cfg.DevelopBranch]
			mainVal := "exists"
			if !hasMain {
				mainVal = "MISSING"
			}
			devVal := "exists"
			if !hasDev {
				devVal = "MISSING"
			}
			checks = append(checks, check{cfg.MainBranch, mainVal, hasMain})
			checks = append(checks, check{cfg.DevelopBranch, devVal, hasDev})

			gfInit := git.IsGitFlowInitialized()
			gfInitVal := "yes"
			if !gfInit {
				gfInitVal = "NOT INITIALIZED (run: gitflow init)"
			}
			checks = append(checks, check{"gitflow structure", gfInitVal, gfInit})

			checks = append(checks, check{"IDE", GF.IDEDisplay(), true})

			allOK := true
			for _, c := range checks {
				if !c.OK {
					allOK = false
					break
				}
			}

			if output.IsJSONMode() {
				output.JSONOutput(map[string]any{
					"action": "doctor",
					"checks": checks,
					"all_ok": allOK,
					"ide":    GF.IDE,
				})
				if !allOK {
					os.Exit(1)
				}
				return nil
			}

			output.Infof("\n  %sGitflow Doctor — prerequisite check%s\n", output.Bold, output.Reset)
			for _, c := range checks {
				icon := output.Green + "✓" + output.Reset
				if !c.OK {
					icon = output.Red + "✗" + output.Reset
				}
				output.Infof("    %s %s: %s", icon, c.Name, c.Value)
			}

			if allOK {
				output.Infof("\n  %sAll prerequisites met.%s", output.Green, output.Reset)
			} else {
				output.Infof("\n  %sSome prerequisites are missing. Fix them before proceeding.%s", output.Red, output.Reset)
				os.Exit(1)
			}
			return nil
		},
	}
}
