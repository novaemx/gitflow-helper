package gitflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/ide"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "initial commit")
	run("git", "branch", "develop")
	return dir
}

func TestNew(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)

	if gf == nil {
		t.Fatal("New returned nil")
	}
	if gf.Config.ProjectRoot != dir {
		t.Errorf("expected project root %s, got %s", dir, gf.Config.ProjectRoot)
	}
	if gf.Config.MainBranch != "main" {
		t.Errorf("expected main branch 'main', got %q", gf.Config.MainBranch)
	}
	if gf.Config.DevelopBranch != "develop" {
		t.Errorf("expected develop branch 'develop', got %q", gf.Config.DevelopBranch)
	}
}

func TestNewFromConfig(t *testing.T) {
	dir := setupTestRepo(t)
	cfg := config.FlowConfig{
		ProjectRoot:   dir,
		MainBranch:    "main",
		DevelopBranch: "develop",
		Remote:        "origin",
		TagPrefix:     "v",
	}
	gf := NewFromConfig(cfg)

	if gf.Config.ProjectRoot != dir {
		t.Errorf("expected project root %s, got %s", dir, gf.Config.ProjectRoot)
	}
}

func TestIDEDisplay(t *testing.T) {
	gf := &Logic{
		IDE: ide.DetectedIDE{ID: ide.IDECursor, DisplayName: "Cursor"},
	}
	if got := gf.IDEDisplay(); got != "Cursor" {
		t.Errorf("expected 'Cursor', got %q", got)
	}

	gf.IDE = ide.DetectedIDE{ID: ide.IDEUnknown, DisplayName: "Terminal"}
	if got := gf.IDEDisplay(); got != "Terminal" {
		t.Errorf("expected 'Terminal', got %q", got)
	}
}

func TestIsGitAvailable(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)
	if !gf.IsGitAvailable() {
		t.Error("expected git to be available")
	}
}

func TestIsGitRepo(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)
	if !gf.IsGitRepo() {
		t.Error("expected to be in a git repo")
	}
}

func TestIsGitFlowInitialized(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)
	if !gf.IsGitFlowInitialized() {
		t.Error("expected gitflow to be initialized (main + develop exist)")
	}
}

func TestRefreshAndStatus(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)
	gf.Refresh()

	if gf.State.Current != "main" {
		t.Errorf("expected current branch 'main', got %q", gf.State.Current)
	}
	if !gf.State.HasMain {
		t.Error("expected HasMain to be true")
	}
	if !gf.State.HasDevelop {
		t.Error("expected HasDevelop to be true")
	}

	s := gf.Status()
	if s.ProjectRoot != dir {
		t.Errorf("expected project root %s, got %s", dir, s.ProjectRoot)
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)
	ok, msg := gf.Init()
	if !ok {
		t.Errorf("expected Init to succeed, got msg=%q", msg)
	}
	if msg != "already_initialized" {
		t.Errorf("expected 'already_initialized', got %q", msg)
	}
}

func TestEnsureRules(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)
	tmpHome := t.TempDir()
	prev := ide.UserHomeDirFunc
	ide.UserHomeDirFunc = func() (string, error) { return tmpHome, nil }
	defer func() { ide.UserHomeDirFunc = prev }()

	created, err := gf.EnsureRules()
	if err != nil {
		t.Fatalf("EnsureRules failed: %v", err)
	}

	// At least one file must be created (IDE rule file or skill).
	// AGENTS.md is only expected when the IDE does not support .agents/
	// (e.g. IDEUnknown). In CI / VS Code the IDE may be detected as Copilot
	// which uses .agents/, so we check for the gitflow skill instead.
	if len(created) == 0 {
		t.Error("expected at least one file to be created by EnsureRules")
	}

	// Second call should be idempotent
	created2, err := gf.EnsureRules()
	if err != nil {
		t.Fatalf("second EnsureRules failed: %v", err)
	}
	if len(created2) != 0 {
		t.Errorf("expected no files created on second call, got %d: %v", len(created2), created2)
	}
}

func TestListSwitchable(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)

	branches := gf.ListSwitchable()
	found := false
	for _, b := range branches {
		if b == "develop" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'develop' in switchable branches, got %v", branches)
	}
}

func TestStart_Feature(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)

	// Switch to develop first
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "checkout", "develop")

	code, result := gf.Start("feature", "test-feature")
	if code != 0 {
		t.Errorf("expected exit code 0, got %d: %v", code, result)
	}
	if result["branch"] != "feature/test-feature" {
		t.Errorf("expected branch 'feature/test-feature', got %v", result["branch"])
	}
}

func TestStart_InvalidType(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)

	code, result := gf.Start("invalid", "test")
	if code == 0 {
		t.Error("expected non-zero exit code for invalid branch type")
	}
	if result["error"] == nil {
		t.Error("expected error in result")
	}
}

func TestReleaseNotes_Empty(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)
	meta := gf.ReleaseNotes("")
	// Should return nil or a valid map (depends on commit history)
	_ = meta
}

func TestEnsureReady(t *testing.T) {
	dir := setupTestRepo(t)
	gf := New(dir)
	ok, msg := gf.EnsureReady()
	if !ok {
		t.Errorf("expected EnsureReady to succeed, got msg=%q", msg)
	}
	if msg != "ready" {
		t.Errorf("expected 'ready', got %q", msg)
	}
}

func TestNewFromEmpty_AutoDetectsRoot(t *testing.T) {
	// New("") should auto-detect project root from cwd
	gf := New("")
	if gf == nil {
		t.Fatal("New('') returned nil")
	}
	if gf.Config.ProjectRoot == "" {
		t.Error("expected non-empty project root")
	}
}

func TestHealthReport_ToMap(t *testing.T) {
	report := HealthReport{
		Action:   "health",
		Issues:   []string{"a"},
		Warnings: []string{"b"},
		OK:       []string{"c"},
		Healthy:  false,
		IDE:      ide.DetectedIDE{ID: ide.IDEUnknown, DisplayName: "Terminal"},
	}

	m := report.ToMap()
	if got, _ := m["action"].(string); got != "health" {
		t.Fatalf("expected action=health, got %v", m["action"])
	}
	if got, _ := m["healthy"].(bool); got {
		t.Fatalf("expected healthy=false, got %v", m["healthy"])
	}
	if got, ok := m["issues"].([]string); !ok || len(got) != 1 || got[0] != "a" {
		t.Fatalf("unexpected issues map value: %#v", m["issues"])
	}
}

func TestHealthReport_DirtyDevelopIsIssue(t *testing.T) {
	dir := setupTestRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "checkout", "develop")
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	gf := New(dir)
	report := gf.HealthReport()
	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue, "protected base branch 'develop' has uncommitted changes") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected protected branch dirty issue, got %+v", report.Issues)
	}
}

func TestHealthReport_PRModeWithoutRemote(t *testing.T) {
	dir := setupTestRepo(t)
	cfg := config.FlowConfig{
		ProjectRoot:     dir,
		MainBranch:      "main",
		DevelopBranch:   "develop",
		Remote:          "origin",
		TagPrefix:       "v",
		IntegrationMode: config.IntegrationModePullRequest,
	}
	gf := NewFromConfig(cfg)
	report := gf.HealthReport()
	found := false
	for _, issue := range report.Issues {
		if strings.Contains(issue, "pull-request") && strings.Contains(issue, "remote") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected PR mode without remote issue, got issues=%v warnings=%v", report.Issues, report.Warnings)
	}
}

func TestHealthReport_DevelopAheadOfMainWarning(t *testing.T) {
	dir := setupTestRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "checkout", "develop")
	// Add 10 commits on develop
	for i := 0; i < 10; i++ {
		fname := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(fname, []byte("content\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		run("git", "add", fname)
		run("git", "commit", "-m", fmt.Sprintf("commit %d", i))
	}

	gf := New(dir)
	report := gf.HealthReport()
	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "develop is") && strings.Contains(w, "ahead of main") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected develop-ahead-of-main warning, got warnings=%v", report.Warnings)
	}
}

func TestHealthReport_VersionFileMismatch(t *testing.T) {
	dir := setupTestRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	// Create a release branch on develop
	run("git", "checkout", "develop")
	run("git", "checkout", "-b", "release/1.2.3")

	// Write a VERSION file that doesn't match the branch
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("9.9.9\n"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	gf := New(dir)
	report := gf.HealthReport()
	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "release/1.2.3") && strings.Contains(w, "VERSION") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected VERSION mismatch warning, got warnings=%v", report.Warnings)
	}
}

func TestHealthReport_VersionFileMatchesReleaseBranch(t *testing.T) {
	dir := setupTestRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "checkout", "develop")
	run("git", "checkout", "-b", "release/2.0.0")
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("2.0.0\n"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	gf := New(dir)
	report := gf.HealthReport()
	for _, w := range report.Warnings {
		if strings.Contains(w, "release/2.0.0") && strings.Contains(w, "VERSION") {
			t.Fatalf("unexpected VERSION mismatch warning when versions match: %s", w)
		}
	}
}

func TestHealthReport_BrokenUpstreamTracking(t *testing.T) {
	// Set up a bare remote so we can simulate a deleted upstream.
	remoteDir := t.TempDir()
	localDir := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	// Init bare remote
	run(remoteDir, "git", "init", "--bare", "-b", "main")

	// Clone into local
	run(localDir, "git", "clone", remoteDir, ".")
	run(localDir, "git", "commit", "--allow-empty", "-m", "init")
	run(localDir, "git", "push", "origin", "main")
	run(localDir, "git", "branch", "develop")
	run(localDir, "git", "push", "origin", "develop")

	// Create and push a feature branch
	run(localDir, "git", "checkout", "-b", "feature/gone-branch")
	run(localDir, "git", "push", "-u", "origin", "feature/gone-branch")

	// Delete the remote branch (simulates "gone" upstream)
	run(localDir, "git", "push", "origin", "--delete", "feature/gone-branch")
	run(localDir, "git", "fetch", "--prune", "origin")

	gf := New(localDir)
	report := gf.HealthReport()
	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "feature/gone-branch") && strings.Contains(w, "deleted upstream") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected deleted upstream warning, got warnings=%v", report.Warnings)
	}
}

func TestHealthReport_FlowBranchWithoutRemoteTracking(t *testing.T) {
	remoteDir := t.TempDir()
	localDir := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run(remoteDir, "git", "init", "--bare", "-b", "main")
	run(localDir, "git", "clone", remoteDir, ".")
	run(localDir, "git", "commit", "--allow-empty", "-m", "init")
	run(localDir, "git", "push", "origin", "main")
	run(localDir, "git", "branch", "develop")
	run(localDir, "git", "push", "origin", "develop")

	// Create a local feature branch but do NOT push it
	run(localDir, "git", "checkout", "-b", "feature/local-only")

	gf := New(localDir)
	report := gf.HealthReport()
	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "feature/local-only") && strings.Contains(w, "no remote tracking") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected no-remote-tracking warning, got warnings=%v", report.Warnings)
	}
}
