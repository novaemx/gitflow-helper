package gitflow

import (
	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/flow"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/ide"
	"github.com/luis-lozano/gitflow-helper/internal/state"
)

// Logic is the top-level facade that coordinates all gitflow workflow
// operations. It owns the config, repo state, and detected IDE, and
// delegates to the existing sub-packages for actual logic.
type Logic struct {
	Config config.FlowConfig
	State  state.RepoState
	IDE    ide.DetectedIDE
}

// New creates a Gitflow facade from a project root path.
// If projectRoot is empty, it auto-detects from the current directory.
func New(projectRoot string) *Logic {
	if projectRoot == "" {
		projectRoot = config.FindProjectRoot()
	}
	cfg := config.LoadConfig(projectRoot)
	git.ProjectRoot = cfg.ProjectRoot

	gf := &Logic{
		Config: cfg,
		IDE:    ide.DetectPrimary(cfg.ProjectRoot),
	}
	return gf
}

// NewFromConfig creates a Gitflow facade from an existing FlowConfig.
func NewFromConfig(cfg config.FlowConfig) *Logic {
	git.ProjectRoot = cfg.ProjectRoot
	return &Logic{
		Config: cfg,
		IDE:    ide.DetectPrimary(cfg.ProjectRoot),
	}
}

// IsGitAvailable returns true if git is installed and in PATH.
func (gf *Logic) IsGitAvailable() bool {
	return git.ExecQuiet("--version") != ""
}

// IsGitRepo returns true if the project root is inside a git repository.
func (gf *Logic) IsGitRepo() bool {
	return git.IsGitRepo()
}

// IsGitFlowInitialized returns true if main+develop branches exist.
func (gf *Logic) IsGitFlowInitialized() bool {
	return git.IsGitFlowInitialized()
}

// Refresh re-detects the full repo state (branches, merge state, divergence).
func (gf *Logic) Refresh() {
	gf.State = state.DetectState(gf.Config)
}

// Status returns the current repo state after a fresh detection.
func (gf *Logic) Status() state.RepoState {
	gf.Refresh()
	return gf.State
}

// Init sets up the main/develop branch structure using raw git commands.
func (gf *Logic) Init() (bool, string) {
	ok, msg := flow.InitGitFlow(gf.Config)
	if ok {
		gf.Refresh()
	}
	return ok, msg
}

// EnsureReady validates the working directory and initializes gitflow if needed.
func (gf *Logic) EnsureReady() (bool, string) {
	ok, msg := flow.EnsureGitFlowReady(gf.Config)
	if ok {
		gf.Refresh()
	}
	return ok, msg
}

// Start creates a new flow branch (feature/bugfix/release/hotfix).
func (gf *Logic) Start(branchType, name string) (int, map[string]any) {
	code, result := flow.StartBranch(gf.Config, branchType, name)
	gf.Refresh()
	return code, result
}

// Finish completes the current (or named) flow branch.
func (gf *Logic) Finish(name string) (int, map[string]any) {
	code, result := flow.FinishCurrent(gf.Config, name)
	gf.Refresh()
	return code, result
}

// Pull performs a safe fetch + fast-forward merge.
func (gf *Logic) Pull() (int, map[string]any) {
	code, result := flow.Pull(gf.Config)
	gf.Refresh()
	return code, result
}

// Sync merges the parent branch into the current flow branch.
func (gf *Logic) Sync() (int, map[string]any) {
	code, result := flow.Sync(gf.Config)
	gf.Refresh()
	return code, result
}

// Backmerge merges main into develop to restore the gitflow invariant.
func (gf *Logic) Backmerge() (int, map[string]any) {
	code, result := flow.Backmerge(gf.Config)
	gf.Refresh()
	return code, result
}

// Cleanup deletes local branches that have been merged into develop/main.
func (gf *Logic) Cleanup() (int, map[string]any) {
	code, result := flow.Cleanup(gf.Config)
	gf.Refresh()
	return code, result
}

// ReleaseNotes generates release notes from git history.
func (gf *Logic) ReleaseNotes(fromTag string) map[string]any {
	return flow.WriteReleaseNotes(gf.Config, fromTag)
}

// ListSwitchable returns branches available for switching.
func (gf *Logic) ListSwitchable() []string {
	allLocal := git.AllLocalBranches()
	cur := git.CurrentBranch()
	return flow.ListSwitchableBranches(allLocal, gf.Config, cur)
}

// IDEDisplay returns the human-readable IDE name for TUI display.
func (gf *Logic) IDEDisplay() string {
	return gf.IDE.DisplayName
}

// EnsureRules checks whether IDE-specific gitflow instruction files exist
// for the detected IDE. If missing, it creates them silently. Also ensures
// AGENTS.md has the gitflow section as a universal fallback.
// Returns the list of files created (empty if all already existed).
func (gf *Logic) EnsureRules() ([]string, error) {
	return ide.EnsureRulesForIDE(gf.Config.ProjectRoot, gf.IDE)
}
