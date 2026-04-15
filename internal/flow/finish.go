package flow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
	"github.com/novaemx/gitflow-helper/internal/output"
	"github.com/novaemx/gitflow-helper/internal/version"
)

var execResultFinish = git.ExecResult
var remoteExistsFinish = git.RemoteExists
var remoteBranchExistsFinish = func(remote, branch string) bool {
	code, _, _ := execResultFinish("ls-remote", "--exit-code", "--heads", remote, branch)
	return code == 0
}

func mergedBranchDeleteWarning(branchName string, err error) string {
	return fmt.Sprintf("Warning: merged branch %s not deleted automatically (%v). You can remove manually with git branch -d %s.", branchName, err, branchName)
}

func addMergeAbortDiagnostics(result map[string]any) {
	if !output.IsJSONMode() {
		return
	}
	abortCode, _, abortErr := execResultFinish("merge", "--abort")
	if abortCode != 0 {
		result["abort_failed"] = true
		if strings.TrimSpace(abortErr) != "" {
			result["abort_error"] = strings.TrimSpace(abortErr)
		}
	}
}

func bumpPatchVersion(ver string) (string, error) {
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid version %q (expected x.y.z)", ver)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major version in %q", ver)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor version in %q", ver)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version in %q", ver)
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), nil
}

func nextAvailableTagVersion(cfg config.FlowConfig, start string) (string, error) {
	candidate := start
	for i := 0; i < 1000; i++ {
		tagName := cfg.TagPrefix + candidate
		if !git.TagExists(tagName) {
			return candidate, nil
		}
		next, err := bumpPatchVersion(candidate)
		if err != nil {
			return "", err
		}
		candidate = next
	}
	return "", fmt.Errorf("unable to find available version after %s", start)
}

func autoBumpFlowVersionIfTagExists(cfg config.FlowConfig, btype, name string, result map[string]any) (string, error) {
	tagName := cfg.TagPrefix + name
	if !git.TagExists(tagName) {
		return name, nil
	}

	next, err := nextAvailableTagVersion(cfg, name)
	if err != nil {
		return "", err
	}
	if next == name {
		return name, nil
	}

	if cfg.VersionFile == "" {
		return "", fmt.Errorf("cannot auto-bump %s: version_file is not configured", btype)
	}

	oldBranch := btype + "/" + name
	newBranch := btype + "/" + next
	if git.BranchExists(newBranch) {
		return "", fmt.Errorf("cannot auto-bump: target branch %s already exists", newBranch)
	}

	output.Infof("  %s⚠ Tag %s already exists; auto-bumping %s to %s.%s", output.Yellow, tagName, btype, next, output.Reset)
	version.WriteVersionFile(cfg, next)

	if err := git.Exec("branch", "-m", oldBranch, newBranch); err != nil {
		return "", fmt.Errorf("failed to rename %s to %s: %w", oldBranch, newBranch, err)
	}

	result["auto_bumped"] = true
	result["auto_bumped_from"] = name
	result["auto_bumped_to"] = next
	result["auto_bump_reason"] = fmt.Sprintf("tag %s already exists", tagName)
	return next, nil
}

// FinishOptions controls optional pre-merge transformations for feature/bugfix finishes.
type FinishOptions struct {
	Rebase       bool // rebase the branch onto develop before the final merge
	Squash       bool // squash all branch commits into a single commit on develop
	DeleteRemote bool // push-delete the remote tracking branch after a successful finish
}

// rebaseOnParent rebases branchName onto parent. Aborts automatically on failure.
func rebaseOnParent(branchName, parent string) error {
	if cur := git.CurrentBranch(); cur != branchName {
		if err := git.Exec("checkout", branchName); err != nil {
			return fmt.Errorf("failed to checkout %s: %w", branchName, err)
		}
	}
	output.Infof("  %sRebasing %s onto %s...%s", output.Dim, branchName, parent, output.Reset)
	if err := git.Exec("rebase", parent); err != nil {
		_ = git.Exec("rebase", "--abort")
		return fmt.Errorf("rebase of %s onto %s failed (conflicts?): %w", branchName, parent, err)
	}
	return nil
}

// squashFeatureBranch does a squash-merge of branchName into parent and commits
// with a descriptive message. Uses -D on the source branch since the commits
// are linearised, not merged via ancestry.
func squashFeatureBranch(cfg config.FlowConfig, btype, name, branchName string) error {
	if err := git.Exec("checkout", cfg.DevelopBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.DevelopBranch, err)
	}
	if err := git.Exec("merge", "--squash", branchName); err != nil {
		return fmt.Errorf("squash merge failed: %w", err)
	}
	squashMsg := fmt.Sprintf("squash(%s): %s", btype, name)
	if err := git.Exec("commit", "-m", squashMsg); err != nil {
		return fmt.Errorf("squash commit failed: %w", err)
	}
	return nil
}

func finishFeatureOrBugfix(cfg config.FlowConfig, btype, name string, opts FinishOptions) error {
	branchName := btype + "/" + name
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}

	// Squash path: stages all branch changes as a single commit directly on develop.
	// Uses -D because the squash commit is not a merge commit in git's eyes.
	if opts.Squash {
		if err := squashFeatureBranch(cfg, btype, name, branchName); err != nil {
			return err
		}
		tryDeleteRemote(cfg, branchName, opts.DeleteRemote)
		if err := git.Exec("branch", "-D", branchName); err != nil {
			output.Infof("  %s%s%s", output.Yellow, mergedBranchDeleteWarning(branchName, err), output.Reset)
		}
		output.Infof("  %s✓ %s/%s squashed into %s%s", output.Green, btype, name, cfg.DevelopBranch, output.Reset)
		return nil
	}

	// Rebase path: linearise branch history onto develop before the merge commit.
	if opts.Rebase {
		if err := rebaseOnParent(branchName, cfg.DevelopBranch); err != nil {
			return err
		}
	}

	if err := git.Exec("checkout", cfg.DevelopBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.DevelopBranch, err)
	}

	mergeMsg := fmt.Sprintf("Merge %s '%s' into %s", btype, name, cfg.DevelopBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", mergeMsg); err != nil {
		return fmt.Errorf("merge of %s failed (conflicts?): %w", branchName, err)
	}

	tryDeleteRemote(cfg, branchName, opts.DeleteRemote)
	if err := git.Exec("branch", "-d", branchName); err != nil {
		output.Infof("  %s%s%s", output.Yellow, mergedBranchDeleteWarning(branchName, err), output.Reset)
	}
	output.Infof("  %s✓ %s/%s → %s%s", output.Green, btype, name, cfg.DevelopBranch, output.Reset)
	return nil
}

// tryDeleteRemote pushes a remote branch deletion when opts.DeleteRemote is true
// and the remote is reachable. Errors are logged as warnings but never fatal.
func tryDeleteRemote(cfg config.FlowConfig, branchName string, deleteRemote bool) {
	if !deleteRemote || cfg.Remote == "" || !remoteExistsFinish(cfg.Remote) {
		return
	}
	if !remoteBranchExistsFinish(cfg.Remote, branchName) {
		return
	}
	code, _, _ := execResultFinish("push", cfg.Remote, "--delete", branchName)
	if code == 0 {
		output.Infof("  %s✓ Remote branch %s/%s deleted.%s", output.Green, cfg.Remote, branchName, output.Reset)
	} else {
		output.Infof("  %s⚠ Could not delete remote branch %s/%s (may not exist remotely).%s",
			output.Yellow, cfg.Remote, branchName, output.Reset)
	}
}

func finishRelease(cfg config.FlowConfig, ver string) error {
	branchName := "release/" + ver
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}
	tagName := cfg.TagPrefix + ver
	if git.TagExists(tagName) {
		return fmt.Errorf("tag %s already exists", tagName)
	}

	if err := git.Exec("checkout", cfg.MainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.MainBranch, err)
	}

	mergeMsg := fmt.Sprintf("Merge release '%s' into %s", ver, cfg.MainBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", mergeMsg); err != nil {
		return fmt.Errorf("merge of %s into %s failed: %w", branchName, cfg.MainBranch, err)
	}

	if err := git.Exec("tag", "-a", tagName, "-m", fmt.Sprintf("Release %s", ver)); err != nil {
		return fmt.Errorf("tag creation failed for %s: %w", tagName, err)
	}

	if err := git.Exec("checkout", cfg.DevelopBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.DevelopBranch, err)
	}

	// Merge the release branch (not the tag) so the genealogy is traceable via
	// branch ancestry, not via tag dereferencing — nvie canonical flow.
	backmergeMsg := fmt.Sprintf("Merge release '%s' into %s", ver, cfg.DevelopBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", backmergeMsg); err != nil {
		return fmt.Errorf("back-merge of %s into %s failed: %w", branchName, cfg.DevelopBranch, err)
	}

	if err := git.Exec("branch", "-d", branchName); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branchName, err)
	}
	output.Infof("  %s✓ release/%s → %s (tagged %s) → %s%s",
		output.Green, ver, cfg.MainBranch, tagName, cfg.DevelopBranch, output.Reset)
	return nil
}

func finishHotfix(cfg config.FlowConfig, ver string) error {
	branchName := "hotfix/" + ver
	if !git.BranchExists(branchName) {
		return fmt.Errorf("branch %s does not exist", branchName)
	}
	tagName := cfg.TagPrefix + ver
	if git.TagExists(tagName) {
		return fmt.Errorf("tag %s already exists", tagName)
	}

	if err := git.Exec("checkout", cfg.MainBranch); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", cfg.MainBranch, err)
	}

	mergeMsg := fmt.Sprintf("Merge hotfix '%s' into %s", ver, cfg.MainBranch)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", mergeMsg); err != nil {
		return fmt.Errorf("merge of %s into %s failed: %w", branchName, cfg.MainBranch, err)
	}

	if err := git.Exec("tag", "-a", tagName, "-m", fmt.Sprintf("Hotfix %s", ver)); err != nil {
		return fmt.Errorf("tag creation failed for %s: %w", tagName, err)
	}

	releases := git.ActiveReleaseBranches()
	backTarget := cfg.DevelopBranch
	if len(releases) > 0 {
		backTarget = releases[0]
	}

	if err := git.Exec("checkout", backTarget); err != nil {
		return fmt.Errorf("failed to checkout %s: %w", backTarget, err)
	}

	backmergeMsg := fmt.Sprintf("Merge hotfix '%s' into %s", ver, backTarget)
	if err := git.Exec("merge", "--no-ff", branchName, "-m", backmergeMsg); err != nil {
		return fmt.Errorf("back-merge of hotfix into %s failed: %w", backTarget, err)
	}

	if err := git.Exec("branch", "-d", branchName); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branchName, err)
	}
	output.Infof("  %s✓ hotfix/%s → %s (tagged %s) → %s%s",
		output.Green, ver, cfg.MainBranch, tagName, backTarget, output.Reset)
	return nil
}

// nonAtomicCommitWarnings returns subjects that appear to mix multiple concerns
// in a single commit (signals: " and " or "; " in the message body).
// The conventional commit type prefix is stripped before the check so that
// "feat(a-and-b): something clean" does not produce a spurious warning.
func nonAtomicCommitWarnings(subjects []string) []string {
	var warnings []string
	for _, s := range subjects {
		body := s
		if idx := strings.Index(s, ": "); idx >= 0 {
			body = s[idx+2:]
		}
		lower := strings.ToLower(body)
		if strings.Contains(lower, " and ") || strings.Contains(lower, "; ") {
			warnings = append(warnings, s)
		}
	}
	return warnings
}

// remoteParentAheadCount returns how many commits origin/parent has that local
// parent does not, using cached remote-tracking refs (no fetch).
// Returns 0 when the remote ref does not exist.
func remoteParentAheadCount(remote, parent string) int {
	ref := remote + "/" + parent
	code, _, _ := git.ExecResult("rev-parse", "--verify", ref)
	if code != 0 {
		return 0
	}
	raw := git.ExecQuiet("rev-list", "--count", parent+".."+ref)
	n, _ := strconv.Atoi(strings.TrimSpace(raw))
	return n
}

func parentForBranchType(cfg config.FlowConfig, btype string) string {
	switch btype {
	case "hotfix":
		return cfg.MainBranch
	default:
		return cfg.DevelopBranch
	}
}

func finishViaPullRequestMode(cfg config.FlowConfig, btype, name, branch string, result map[string]any) (int, map[string]any) {
	parent := parentForBranchType(cfg, btype)
	result["integration_mode"] = cfg.IntegrationMode
	result["parent"] = parent
	result["branch"] = branch

	if !git.RemoteExists(cfg.Remote) {
		result["result"] = "pr_required"
		result["warning"] = "remote not configured; branch was not pushed"
		result["needs_human"] = true
		result["next"] = []string{
			"Configure a remote (for example: git remote add origin <url>)",
			"Push your branch",
			"Open a pull request to the parent branch",
		}
		return 0, result
	}

	code, _, pushErr := git.ExecResult("push", "-u", cfg.Remote, branch)
	if code != 0 {
		result["result"] = "error"
		result["error"] = "failed to push branch before PR: " + pushErr
		return 1, result
	}

	result["result"] = "pr_ready"
	result["needs_human"] = true
	result["pr"] = map[string]any{
		"head":  branch,
		"base":  parent,
		"title": fmt.Sprintf("%s: %s", btype, name),
	}
	result["next"] = []string{
		fmt.Sprintf("Open a pull request from %s to %s", branch, parent),
		"Merge in origin using your repository policy",
		fmt.Sprintf("Then switch back to %s and pull latest", cfg.DevelopBranch),
	}

	output.Infof("  %s✓ Branch pushed for PR workflow.%s", output.Green, output.Reset)
	output.Infof("  %sOpen PR:%s %s -> %s", output.Dim, output.Reset, branch, parent)
	return 0, result
}

// FinishCurrent finishes the current (or named) flow branch.
// Pass an optional FinishOptions to enable rebase-first, squash, or remote deletion.
func FinishCurrent(cfg config.FlowConfig, name string, opts ...FinishOptions) (int, map[string]any) {
	var opt FinishOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	branch := git.CurrentBranch()
	btype := git.BranchTypeOf(branch)

	if btype != "feature" && btype != "bugfix" && btype != "release" && btype != "hotfix" {
		if name != "" {
			for _, prefix := range []string{"feature/", "bugfix/", "release/", "hotfix/"} {
				if strings.HasPrefix(name, prefix) {
					btype = strings.TrimSuffix(prefix, "/")
					name = strings.TrimPrefix(name, prefix)
					break
				}
			}
		}
		if btype != "feature" && btype != "bugfix" && btype != "release" && btype != "hotfix" {
			return 1, map[string]any{"action": "finish", "error": "not on flow branch"}
		}
	}

	if name == "" {
		prefixes := map[string]string{
			"feature": "feature/",
			"bugfix":  "bugfix/",
			"release": "release/",
			"hotfix":  "hotfix/",
		}
		name = strings.TrimLeft(strings.TrimPrefix(branch, prefixes[btype]), "v")
	}

	result := map[string]any{
		"action": "finish",
		"type":   btype,
		"name":   name,
	}

	// Dirty check must run before any side-effects (release notes commit, etc.)
	wt := git.WorkingTreeStatus()
	if wt.Staged > 0 || wt.Unstaged > 0 {
		var parts []string
		if wt.Staged > 0 {
			parts = append(parts, fmt.Sprintf("%d staged", wt.Staged))
		}
		if wt.Unstaged > 0 {
			parts = append(parts, fmt.Sprintf("%d modified", wt.Unstaged))
		}
		if wt.Untracked > 0 {
			parts = append(parts, fmt.Sprintf("%d untracked", wt.Untracked))
		}
		detail := strings.Join(parts, ", ")
		output.Infof("  %s✗ Uncommitted changes (%s) — commit or stash first.%s",
			output.Red, detail, output.Reset)
		result["result"] = "error"
		result["error"] = fmt.Sprintf("dirty working tree: %s", detail)
		result["dirty"] = map[string]int{
			"staged": wt.Staged, "modified": wt.Unstaged, "untracked": wt.Untracked,
		}
		return 1, result
	}

	if wt.Untracked > 0 {
		result["warning_untracked"] = wt.Untracked
	}

	// Advisory pre-flight checks — non-blocking; results are attached to the
	// JSON response and printed as warnings so the caller can decide.
	{
		parent := cfg.DevelopBranch
		if btype == "hotfix" {
			parent = cfg.MainBranch
		}

		// 1. Non-atomic commit detection
		subjects := git.BranchCommitSubjects(parent, branch)
		if warns := nonAtomicCommitWarnings(subjects); len(warns) > 0 {
			for _, w := range warns {
				output.Infof("  %s⚠ Non-atomic commit detected: %q — consider splitting before finish.%s",
					output.Yellow, w, output.Reset)
			}
			result["non_atomic_commits"] = warns
		}

		// 2. Remote parent drift — uses cached tracking ref, no fetch
		if cfg.Remote != "" && git.RemoteExists(cfg.Remote) {
			if n := remoteParentAheadCount(cfg.Remote, parent); n > 0 {
				output.Infof("  %s⚠ %s/%s has %d commit(s) ahead of local — run 'gitflow sync' before finish.%s",
					output.Yellow, cfg.Remote, parent, n, output.Reset)
				result["remote_parent_ahead"] = n
			}
		}

		// 3. Sync-merge count in feature/bugfix — nudge toward rebase for clean graph
		if btype == "feature" || btype == "bugfix" {
			if n := len(git.BranchMergeCommitSubjects(parent, branch)); n > 0 {
				output.Infof("  %sℹ %d sync merge(s) inside branch — rebase before finish for linear history.%s",
					output.Dim, n, output.Reset)
				result["sync_merges_in_branch"] = n
			}
		}
	}

	if cfg.IntegrationMode == config.IntegrationModePullRequest {
		headBranch := branch
		if !strings.HasPrefix(headBranch, btype+"/") {
			headBranch = btype + "/" + name
		}
		return finishViaPullRequestMode(cfg, btype, name, headBranch, result)
	}

	// Invariant guard: before finishing a release, main must not have commits that
	// are absent from develop. Otherwise the back-merge after the release finish
	// would leave develop permanently behind main (violates the nvie funnel).
	if btype == "release" {
		raw := git.ExecQuiet("rev-list", "--count", cfg.DevelopBranch+".."+cfg.MainBranch)
		if n, _ := strconv.Atoi(strings.TrimSpace(raw)); n > 0 {
			output.Infof("  %s✗ %s is %d commit(s) ahead of %s — run 'gitflow backmerge' before finishing a release.%s",
				output.Red, cfg.MainBranch, n, cfg.DevelopBranch, output.Reset)
			result["result"] = "error"
			result["error"] = fmt.Sprintf("%s is %d commit(s) ahead of %s — backmerge required before release finish", cfg.MainBranch, n, cfg.DevelopBranch)
			result["action_required"] = "backmerge"
			return 1, result
		}
	}

	if btype == "release" || btype == "hotfix" {
		fileVer := git.FlowVersion(version.ReadVersion(cfg))
		if fileVer != "" && fileVer != "0.0.0" && fileVer != name {
			result["result"] = "error"
			result["version_file"] = cfg.VersionFile
			result["version_from_file"] = fileVer
			result["error"] = fmt.Sprintf("version mismatch: branch %s/%s but %s=%s", btype, name, cfg.VersionFile, fileVer)
			return 1, result
		}

		updatedName, err := autoBumpFlowVersionIfTagExists(cfg, btype, name, result)
		if err != nil {
			result["result"] = "error"
			result["error"] = err.Error()
			return 1, result
		}
		if updatedName != name {
			name = updatedName
			result["name"] = name
		}
	}

	if btype == "release" || btype == "hotfix" {
		meta := WriteReleaseNotes(cfg, "")
		if meta != nil {
			result["release_notes"] = meta
			if err := git.Exec("add", "RELEASE_NOTES.md"); err != nil {
				result["result"] = "error"
				result["error"] = "failed to stage release notes: " + err.Error()
				return 1, result
			}
			if git.HasStagedChanges() {
				if err := git.Exec("commit", "-m", fmt.Sprintf("docs: release notes for %s %s", btype, name)); err != nil {
					result["result"] = "error"
					result["error"] = "failed to commit release notes: " + err.Error()
					return 1, result
				}
			}
		}
	}

	var err error
	switch btype {
	case "feature", "bugfix":
		err = finishFeatureOrBugfix(cfg, btype, name, opt)
	case "release":
		err = finishRelease(cfg, name)
	case "hotfix":
		err = finishHotfix(cfg, name)
	}

	if err != nil {
		result["result"] = "error"
		result["error"] = err.Error()
		conflicts := git.ExecLines("diff", "--name-only", "--diff-filter=U")
		if len(conflicts) > 0 {
			result["conflicts"] = conflicts
			result["needs_human"] = true
			addMergeAbortDiagnostics(result)
			return 2, result
		}
		return 1, result
	}

	cur := git.CurrentBranch()
	if cur != cfg.DevelopBranch {
		_ = git.Exec("checkout", cfg.DevelopBranch)
	}

	result["result"] = "ok"
	result["landed_on"] = cfg.DevelopBranch
	return 0, result
}
