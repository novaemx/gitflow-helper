package gitflow

import (
	"fmt"
	"os"
	"path/filepath"
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

// HealthReport is the canonical typed result for repository health checks.
// Use this model internally and convert to map only at integration boundaries.
type HealthReport struct {
	Action   string          `json:"action"`
	Issues   []string        `json:"issues"`
	Warnings []string        `json:"warnings"`
	OK       []string        `json:"ok"`
	Healthy  bool            `json:"healthy"`
	IDE      ide.DetectedIDE `json:"ide"`
}

func (r HealthReport) ToMap() map[string]any {
	return map[string]any{
		"action":   r.Action,
		"issues":   r.Issues,
		"warnings": r.Warnings,
		"ok":       r.OK,
		"healthy":  r.Healthy,
		"ide":      r.IDE,
	}
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
// For a fresh repository (not already initialized) it also provisions IDE
// agent-rule files and commits them on the develop branch so the working tree
// is clean and all generated files are version-controlled from the start.
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

// FastRelease merges a feature/bugfix branch directly to main, bypassing the
// release/ staging phase. See flow.FastRelease for full semantics.
func (gf *Logic) FastRelease(featureName string) (int, map[string]any) {
	code, result := flow.FastRelease(gf.Config, featureName)
	gf.Refresh()
	return code, result
}

// Finish completes the current (or named) flow branch.
func (gf *Logic) Finish(name string, opts ...flow.FinishOptions) (int, map[string]any) {
	code, result := flow.FinishCurrent(gf.Config, name, opts...)
	gf.Refresh()
	return code, result
}

// Pull performs a safe fetch + fast-forward merge.
func (gf *Logic) Pull() (int, map[string]any) {
	code, result := flow.Pull(gf.Config)
	gf.Refresh()
	return code, result
}

// Push pushes the current local branch to a selected remote target branch.
func (gf *Logic) Push(target string) (int, map[string]any) {
	code, result := flow.Push(gf.Config, target)
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

func (gf *Logic) IntegrationMode() string {
	mode := config.NormalizeIntegrationMode(gf.Config.IntegrationMode)
	if mode == "" {
		mode = config.IntegrationModeLocalMerge
	}
	return mode
}

// ResetChecks clears cached git checks (availability, repo detection, init)
// so subsequent calls will re-evaluate the git environment. Useful when the
// working directory mutates during execution (for example, after running
// `git init` in-place).
func (gf *Logic) ResetChecks() {
	gf.gitAvailCache = nil
	gf.isgitRepoCache = nil
	gf.gfInitCache = nil
}

func (gf *Logic) SetIntegrationMode(mode string) error {
	normalized := config.NormalizeIntegrationMode(mode)
	if normalized == "" {
		normalized = config.IntegrationModeLocalMerge
	}
	if err := config.SetIntegrationMode(gf.Config.ProjectRoot, normalized); err != nil {
		return err
	}
	gf.Config.IntegrationMode = normalized
	gf.Config.ModeConfigured = true
	return nil
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

// HealthReport evaluates repository health and returns a typed report.
func (gf *Logic) HealthReport() HealthReport {
	cfg := gf.Config
	var issues, warnings, okItems []string
	s := gf.Status()

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

	if s.Merge.InMerge {
		if len(s.Merge.ConflictedFiles) > 0 {
			issues = append(issues, fmt.Sprintf("merge conflict in progress (%d file(s) conflicted)", len(s.Merge.ConflictedFiles)))
		} else {
			warnings = append(warnings, "merge operation in progress without listed conflicted files")
		}
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

	if len(s.Releases) > 1 {
		issues = append(issues, fmt.Sprintf("multiple open release branches (%d)", len(s.Releases)))
	}
	if len(s.Hotfixes) > 1 {
		warnings = append(warnings, fmt.Sprintf("multiple open hotfix branches (%d)", len(s.Hotfixes)))
	}

	branchType := git.BranchTypeOf(s.Current)
	if branchType == "other" {
		warnings = append(warnings, fmt.Sprintf("current branch '%s' is not a gitflow branch", s.Current))
	}
	if s.Dirty && (s.Current == cfg.DevelopBranch || s.Current == cfg.MainBranch) {
		issues = append(issues, fmt.Sprintf("protected base branch '%s' has uncommitted changes", s.Current))
	}

	if remoteExists {
		for _, branch := range []string{cfg.MainBranch, cfg.DevelopBranch} {
			if !localSet[branch] {
				continue
			}
			unpushed := git.ExecQuiet("rev-list", "--count", cfg.Remote+"/"+branch+".."+branch)
			n, _ := strconv.Atoi(unpushed)
			if n > 0 {
				warnings = append(warnings, fmt.Sprintf("'%s' has %d unpushed commit(s) — fix: gitflow push %s", branch, n, branch))
			} else {
				okItems = append(okItems, fmt.Sprintf("'%s' up to date with remote", branch))
			}
		}

		if branchType == "feature" || branchType == "bugfix" || branchType == "release" || branchType == "hotfix" {
			if !strings.Contains(s.Current, "/") {
				warnings = append(warnings, "flow branch name format looks invalid")
			} else {
				localAhead := git.ExecQuiet("rev-list", "--count", cfg.Remote+"/"+s.Current+".."+s.Current)
				if n, _ := strconv.Atoi(localAhead); n > 0 {
					warnings = append(warnings, fmt.Sprintf("current branch '%s' has %d unpushed commit(s)", s.Current, n))
				}
			}
		}

		// Broken upstream tracking: branches whose tracked remote was deleted.
		refLines := git.ExecLines("for-each-ref", "--format=%(refname:short) %(upstream:track)", "refs/heads/")
		for _, line := range refLines {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			branch := parts[0]
			tracking := strings.Join(parts[1:], " ")
			if strings.Contains(tracking, "[gone]") {
				warnings = append(warnings, fmt.Sprintf("branch '%s' has a deleted upstream — fix: git branch --unset-upstream %s", branch, branch))
			}
		}

		// Flow branches with no remote tracking at all.
		for _, b := range allLocal {
			if !strings.HasPrefix(b, "feature/") && !strings.HasPrefix(b, "bugfix/") &&
				!strings.HasPrefix(b, "release/") && !strings.HasPrefix(b, "hotfix/") {
				continue
			}
			code, _, _ := git.ExecResult("rev-parse", "--verify", "refs/remotes/"+cfg.Remote+"/"+b)
			if code != 0 {
				warnings = append(warnings, fmt.Sprintf("flow branch '%s' has no remote tracking — consider: git push -u %s %s", b, cfg.Remote, b))
			}
		}
	}

	// PR mode sanity: pull-request mode requires a reachable remote.
	if cfg.IntegrationMode == config.IntegrationModePullRequest && !remoteExists {
		issues = append(issues, fmt.Sprintf("integration mode is 'pull-request' but remote '%s' is not configured — fix: git remote add %s <url>", cfg.Remote, cfg.Remote))
	}

	// Develop release-readiness: many unreleased commits on develop.
	if localSet[cfg.DevelopBranch] && localSet[cfg.MainBranch] {
		devAhead := git.ExecQuiet("rev-list", "--count", cfg.MainBranch+".."+cfg.DevelopBranch)
		if n, _ := strconv.Atoi(devAhead); n >= 10 {
			warnings = append(warnings, fmt.Sprintf("develop is %d commit(s) ahead of main — consider preparing a release", n))
		}
	}

	// VERSION file consistency: open release branch name must match VERSION file.
	versionFile := filepath.Join(cfg.ProjectRoot, "VERSION")
	if data, err := os.ReadFile(versionFile); err == nil {
		fileVer := strings.TrimSpace(string(data))
		for _, b := range allLocal {
			if strings.HasPrefix(b, "release/") {
				branchVer := strings.TrimPrefix(b, "release/")
				if branchVer != fileVer && strings.TrimPrefix(branchVer, "v") != strings.TrimPrefix(fileVer, "v") {
					warnings = append(warnings, fmt.Sprintf("release branch '%s' version doesn't match VERSION file (%s)", b, fileVer))
				}
			}
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
				if ageDays > 14 {
					parent := cfg.DevelopBranch
					if strings.HasPrefix(b, "hotfix/") {
						parent = cfg.MainBranch
					}
					behind := git.ExecQuiet("rev-list", "--count", b+".."+parent)
					if bn, _ := strconv.Atoi(behind); bn > 20 {
						warnings = append(warnings, fmt.Sprintf("merge-hell risk: %s is %d commit(s) behind %s (inactive %d days)", b, bn, parent, ageDays))
					}
				}
			}
		}
	}

	dirtyCount := len(git.ExecLines("status", "--porcelain"))
	if dirtyCount > 0 {
		if !(s.Current == cfg.DevelopBranch || s.Current == cfg.MainBranch) {
			warnings = append(warnings, fmt.Sprintf("%d uncommitted file(s)", dirtyCount))
		}
	}

	okItems = append(okItems, fmt.Sprintf("IDE: %s", gf.IDEDisplay()))

	return HealthReport{
		Action:   "health",
		Issues:   issues,
		Warnings: warnings,
		OK:       okItems,
		Healthy:  len(issues) == 0,
		IDE:      gf.IDE,
	}
}

// Health returns a map representation for compatibility with existing integrations.
func (gf *Logic) Health() map[string]any {
	return gf.HealthReport().ToMap()
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
