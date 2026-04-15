package gitflow

import (
	"os"
	"os/exec"
	"path/filepath"
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

	// Should have created AGENTS.md at minimum (IDE is unknown in test)
	found := false
	for _, f := range created {
		if filepath.Base(f) == "AGENTS.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected AGENTS.md to be created")
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
