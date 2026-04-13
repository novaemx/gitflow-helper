package gitflow

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/debug"
	"github.com/novaemx/gitflow-helper/internal/flow"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/ide"
	"github.com/novaemx/gitflow-helper/internal/state"
)

// Logic is the top-level facade that coordinates all gitflow workflow
// operations. It owns the config, repo state, and detected IDE, and
// delegates to the existing sub-packages for actual logic.
type Logic struct {
	Config     config.FlowConfig
	State      state.RepoState
	IDE        ide.DetectedIDE
	AppVersion string

	// Caches for git checks (immutable during command execution)
	gitAvailCache  *bool
	isgitRepoCache *bool
	gfInitCache    *bool
}

// New creates a Gitflow facade from a project root path.
// If projectRoot is empty, it auto-detects from the current directory.
func New(projectRoot string) *Logic {
	deferTotal := debug.Start("gitflow.New.total")
	defer deferTotal()

	deferFind := debug.Start("gitflow.New.FindProjectRoot")
	if projectRoot == "" {
		projectRoot = config.FindProjectRoot()
	}
	deferFind()

	deferLoad := debug.Start("gitflow.New.LoadConfig")
	cfg := config.LoadConfig(projectRoot)
	deferLoad()

	git.ProjectRoot = cfg.ProjectRoot

	deferDetect := debug.Start("gitflow.New.DetectPrimary")
	detectedIDE := ide.DetectPrimary(cfg.ProjectRoot)
	deferDetect()

	debug.Printf("Gitflow initialized with IDE: %s", detectedIDE.ID)

	gf := &Logic{
		Config: cfg,
		IDE:    detectedIDE,
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
// Result is cached after first call.
func (gf *Logic) IsGitAvailable() bool {
	if gf.gitAvailCache != nil {
		return *gf.gitAvailCache
	}
	result := git.ExecQuiet("--version") != ""
	gf.gitAvailCache = &result
	return result
}

// IsGitRepo returns true if the project root is inside a git repository.
// Result is cached after first call.
func (gf *Logic) IsGitRepo() bool {
	if gf.isgitRepoCache != nil {
		return *gf.isgitRepoCache
	}
	result := git.IsGitRepo()
	gf.isgitRepoCache = &result
	return result
}

// IsGitFlowInitialized returns true if main+develop branches exist.
// Result is cached after first call.
func (gf *Logic) IsGitFlowInitialized() bool {
	if gf.gfInitCache != nil {
		return *gf.gfInitCache
	}
	result := git.IsGitFlowInitialized()
	gf.gfInitCache = &result
	return result
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

// Switch changes to the target branch with auto-stash support.
func (gf *Logic) Switch(target string) (int, map[string]any) {
	cur := git.CurrentBranch()
	available := gf.ListSwitchable()

	var chosen string
	for _, b := range available {
		if b == target || (len(b) > len(target) && b[len(b)-len(target)-1:] == "/"+target) {
			chosen = b
			break
		}
	}
	if chosen == "" {
		return 1, map[string]any{"action": "switch", "result": "not_found", "target": target, "available": available}
	}

	stashed := false
	if git.HasUncommittedChanges() {
		stashed = flow.SmartStashSave(cur)
	}

	code, _, _ := git.ExecResult("checkout", chosen)
	if code != 0 {
		if stashed {
			flow.SmartStashPop(cur)
		}
		return 1, map[string]any{"action": "switch", "result": "error", "target": chosen}
	}

	flow.SmartStashPop(chosen)
	gf.Refresh()
	return 0, map[string]any{"action": "switch", "result": "ok", "branch": chosen, "previous": cur}
}

// Health runs a comprehensive repo health check, returning structured results.
func (gf *Logic) Health() map[string]any {
	cfg := gf.Config
	var issues, warnings, okItems []string

	gitVer := git.ExecQuiet("--version")
	if gitVer == "" {
		issues = append(issues, "git is not installed or not in PATH")
	} else {
		okItems = append(okItems, "git: "+strings.Replace(gitVer, "git version ", "", 1))
	}

	if !git.IsGitFlowInitialized() {
		issues = append(issues, "gitflow not initialized (run: gitflow init)")
	} else {
		okItems = append(okItems, "gitflow structure: main + develop branches present")
	}

	allLocal := git.AllLocalBranches()
	localSet := make(map[string]bool)
	for _, b := range allLocal {
		localSet[b] = true
	}
	if !localSet[cfg.DevelopBranch] {
		issues = append(issues, fmt.Sprintf("'%s' branch missing", cfg.DevelopBranch))
	}
	if !localSet[cfg.MainBranch] {
		issues = append(issues, fmt.Sprintf("'%s' branch missing", cfg.MainBranch))
	}

	remoteExists := git.RemoteExists(cfg.Remote)
	if !remoteExists {
		warnings = append(warnings, fmt.Sprintf("remote '%s' not configured — fix: git remote add %s <url>", cfg.Remote, cfg.Remote))
	} else {
		fetchCode, _, _ := git.ExecResult("ls-remote", "--exit-code", cfg.Remote, "HEAD")
		if fetchCode != 0 {
			warnings = append(warnings, fmt.Sprintf("remote '%s' unreachable — fix: verify network/credentials or run 'git remote -v'", cfg.Remote))
		} else {
			okItems = append(okItems, fmt.Sprintf("remote '%s' reachable", cfg.Remote))
		}
	}

	if localSet[cfg.DevelopBranch] && localSet[cfg.MainBranch] {
		mainAhead := git.ExecQuiet("rev-list", "--count", cfg.DevelopBranch+".."+cfg.MainBranch)
		n, _ := strconv.Atoi(mainAhead)
		if n > 0 {
			files := git.ExecLines("diff", "--name-only", cfg.DevelopBranch+"..."+cfg.MainBranch)
			issues = append(issues, fmt.Sprintf("%s is %d commit(s) ahead of %s (%d file(s))",
				cfg.MainBranch, n, cfg.DevelopBranch, len(files)))
		} else {
			okItems = append(okItems, fmt.Sprintf("%s contains all of %s", cfg.DevelopBranch, cfg.MainBranch))
		}
	}

	for _, b := range allLocal {
		if strings.HasPrefix(b, "feature/") || strings.HasPrefix(b, "bugfix/") ||
			strings.HasPrefix(b, "release/") || strings.HasPrefix(b, "hotfix/") {
			ts := git.ExecQuiet("log", "-1", "--format=%ct", b)
			if epoch, err := strconv.ParseInt(ts, 10, 64); err == nil {
				ageDays := int(time.Since(time.Unix(epoch, 0)).Hours() / 24)
				if ageDays > 30 {
					warnings = append(warnings, fmt.Sprintf("stale branch: %s (inactive %d days)", b, ageDays))
				}
			}
		}
	}

	dirtyCount := len(git.ExecLines("status", "--porcelain"))
	if dirtyCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d uncommitted file(s)", dirtyCount))
	}

	okItems = append(okItems, fmt.Sprintf("IDE: %s", gf.IDEDisplay()))

	return map[string]any{
		"action":   "health",
		"issues":   issues,
		"warnings": warnings,
		"ok":       okItems,
		"healthy":  len(issues) == 0,
		"ide":      gf.IDE,
	}
}

// Doctor validates prerequisites and returns structured results.
func (gf *Logic) Doctor() map[string]any {
	cfg := gf.Config
	type check struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		OK    bool   `json:"ok"`
	}
	var checks []check

	checks = append(checks, check{"Go runtime", runtime.Version(), true})

	gitVer := git.ExecQuiet("--version")
	checks = append(checks, check{"git", strings.Replace(gitVer, "git version ", "", 1), gitVer != ""})

	gfInit := git.IsGitFlowInitialized()
	gfVal := "yes"
	if !gfInit {
		gfVal = "NOT INITIALIZED"
	}
	checks = append(checks, check{"gitflow structure", gfVal, gfInit})
	checks = append(checks, check{"IDE", gf.IDEDisplay(), true})
	checks = append(checks, check{"project_root", cfg.ProjectRoot, true})

	allOK := true
	for _, c := range checks {
		if !c.OK {
			allOK = false
			break
		}
	}

	return map[string]any{
		"action": "doctor",
		"checks": checks,
		"all_ok": allOK,
		"ide":    gf.IDE,
	}
}

// Log returns structured gitflow-aware commit log entries.
func (gf *Logic) Log(count int) map[string]any {
	logFmt := "%H|%h|%s|%an|%ar|%D"
	entries := git.ExecLines("log", "--all", fmt.Sprintf("--format=%s", logFmt), "-n", fmt.Sprintf("%d", count))

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

	tagRe := regexp.MustCompile(`tag:\s*([^\s,)]+)`)
	var parsed []logEntry
	for _, entry := range entries {
		parts := strings.SplitN(entry, "|", 6)
		if len(parts) < 6 {
			continue
		}
		e := logEntry{
			FullSHA: parts[0], SHA: parts[1], Subject: parts[2],
			Author: parts[3], Date: parts[4], Refs: strings.TrimSpace(parts[5]),
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

	return map[string]any{"action": "log", "entries": parsed}
}

// Undo analyzes reflog for undoable operations and returns candidates.
func (gf *Logic) Undo() map[string]any {
	cur := git.CurrentBranch()
	reflog := git.ExecLines("reflog", "--format=%H %gs", "-n", "20")

	if len(reflog) == 0 {
		return map[string]any{"action": "undo", "result": "no_reflog"}
	}

	keywords := []string{"merge", "checkout: moving", "commit", "finish", "start"}
	type candidate struct {
		SHA  string `json:"sha"`
		Desc string `json:"description"`
	}
	var candidates []candidate
	for _, entry := range reflog {
		parts := strings.SplitN(entry, " ", 2)
		if len(parts) < 2 {
			continue
		}
		sha, desc := parts[0], parts[1]
		descLower := strings.ToLower(desc)
		for _, kw := range keywords {
			if strings.Contains(descLower, kw) {
				if len(sha) > 12 {
					sha = sha[:12]
				}
				candidates = append(candidates, candidate{sha, desc})
				break
			}
		}
		if len(candidates) >= 10 {
			break
		}
	}

	if len(candidates) == 0 {
		return map[string]any{"action": "undo", "result": "nothing_to_undo"}
	}

	return map[string]any{
		"action":         "undo",
		"result":         "candidates",
		"current_branch": cur,
		"entries":        candidates,
	}
}
