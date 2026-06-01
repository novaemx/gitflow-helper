package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
	"github.com/novaemx/gitflow-helper/internal/ide"
	"github.com/novaemx/gitflow-helper/internal/output"
)

func setupSetupTest(t *testing.T) string {
	t.Helper()
	dir := setupCommandsRepo(t)
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v"})

	// Mock user-home for fallback skill install.
	home := t.TempDir()
	prevHome := ide.UserHomeDirFunc
	ide.UserHomeDirFunc = func() (string, error) { return home, nil }
	t.Cleanup(func() { ide.UserHomeDirFunc = prevHome })

	// Default: pretend the user accepts the consent dialog. Individual tests
	// can override this to assert that --yes bypasses the dialog entirely.
	prevAsk := ide.AskAIIntegrationFunc
	ide.AskAIIntegrationFunc = func(_ ide.DetectedIDE) (bool, error) { return true, nil }
	t.Cleanup(func() { ide.AskAIIntegrationFunc = prevAsk })

	// Default to JSON mode so the tests don't write to stdout.
	prevJSON := output.IsJSONMode()
	output.SetJSONMode(true)
	t.Cleanup(func() { output.SetJSONMode(prevJSON) })

	return dir
}

// setupSetupTestInteractive is the same as setupSetupTest but leaves
// interactive mode on (JSON mode off) so the consent dialog IS reachable.
func setupSetupTestInteractive(t *testing.T) string {
	t.Helper()
	dir := setupCommandsRepo(t)
	GF = gitflow.NewFromConfig(config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v"})

	home := t.TempDir()
	prevHome := ide.UserHomeDirFunc
	ide.UserHomeDirFunc = func() (string, error) { return home, nil }
	t.Cleanup(func() { ide.UserHomeDirFunc = prevHome })

	prevJSON := output.IsJSONMode()
	output.SetJSONMode(false) // interactive
	t.Cleanup(func() { output.SetJSONMode(prevJSON) })

	return dir
}

func runSetupWithFlags(t *testing.T, dir string, flags ...string) error {
	t.Helper()
	cmd := newSetupCmd()
	cmd.SetArgs(append([]string{}, flags...))
	cmd.SetOut(os.NewFile(0, os.DevNull))
	cmd.SetErr(os.NewFile(0, os.DevNull))
	GF.Config.ProjectRoot = dir
	return cmd.Execute()
}

func TestSetupCmd_YesFlag_SkipsConsentDialog(t *testing.T) {
	dir := setupSetupTestInteractive(t)

	called := false
	prev := ide.AskAIIntegrationFunc
	ide.AskAIIntegrationFunc = func(_ ide.DetectedIDE) (bool, error) {
		called = true
		return true, nil
	}
	t.Cleanup(func() { ide.AskAIIntegrationFunc = prev })

	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--yes"); err != nil {
		t.Fatalf("setup --yes: %v", err)
	}
	if called {
		t.Error("consent dialog must NOT be called when --yes is set")
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules", "gitflow-preflight.mdc")); err != nil {
		t.Errorf("expected Cursor rule after --yes: %v", err)
	}
}

func TestSetupCmd_Default_Interactive_CallsConsentDialog(t *testing.T) {
	dir := setupSetupTestInteractive(t)

	called := false
	prev := ide.AskAIIntegrationFunc
	ide.AskAIIntegrationFunc = func(_ ide.DetectedIDE) (bool, error) {
		called = true
		return true, nil
	}
	t.Cleanup(func() { ide.AskAIIntegrationFunc = prev })

	_ = runSetupWithFlags(t, dir, "--ide", "cursor")
	if !called {
		t.Error("consent dialog must be called in interactive mode without --yes")
	}
}

func TestSetupCmd_ForceFlag_ReinstallsEvenIfContentMatches(t *testing.T) {
	dir := setupSetupTest(t)

	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--yes"); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	rulePath := filepath.Join(dir, ".cursor", "rules", "gitflow-preflight.mdc")
	firstMtime := fileMTime(t, rulePath)

	// Second install WITHOUT --force: should be a no-op (mtime unchanged).
	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--yes"); err != nil {
		t.Fatalf("second setup: %v", err)
	}
	if fileMTime(t, rulePath) != firstMtime {
		t.Error("expected mtime unchanged on idempotent re-run")
	}

	// Third install WITH --force: should rewrite the file (mtime changes).
	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--yes", "--force"); err != nil {
		t.Fatalf("third setup (force): %v", err)
	}
	if fileMTime(t, rulePath) == firstMtime {
		t.Error("expected mtime to change with --force")
	}
}

func TestSetupCmd_CheckFlag_DryRun_CreatesNoFiles(t *testing.T) {
	dir := setupSetupTest(t)

	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--check"); err != nil {
		t.Fatalf("setup --check: %v", err)
	}
	for _, rel := range []string{
		filepath.Join(".cursor", "rules", "gitflow-preflight.mdc"),
		filepath.Join(".cursor", "mcp.json"),
		filepath.Join(".agents", "skills", "gitflow", "SKILL.md"),
		filepath.Join("AGENTS.md"),
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
			t.Errorf("--check should not have created %s; stat err = %v", rel, err)
		}
	}
}

func TestSetupCmd_UninstallFlag_RemovesAllArtifacts(t *testing.T) {
	dir := setupSetupTest(t)

	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--yes"); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	rulePath := filepath.Join(dir, ".cursor", "rules", "gitflow-preflight.mdc")
	if _, err := os.Stat(rulePath); err != nil {
		t.Fatalf("setup did not create rule: %v", err)
	}

	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--uninstall"); err != nil {
		t.Fatalf("setup --uninstall: %v", err)
	}
	if _, err := os.Stat(rulePath); !os.IsNotExist(err) {
		t.Errorf("expected cursor rule removed; stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "mcp.json")); !os.IsNotExist(err) {
		t.Errorf("expected cursor MCP config removed; stat err = %v", err)
	}
	choice, exists, err := config.LoadAIIntegrationChoice(dir)
	if err != nil {
		t.Fatalf("load consent: %v", err)
	}
	if exists && choice.Enabled {
		t.Error("expected consent entry cleared by --uninstall")
	}
}

func TestSetupCmd_UninstallFlag_AlsoRemovesClaudeCompanionArtifacts(t *testing.T) {
	dir := setupSetupTest(t)
	t.Setenv("CLAUDE_SESSION", "1")

	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--yes"); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	if _, err := os.Stat(claudeMD); err != nil {
		t.Fatalf("companion CLAUDE.md not created: %v", err)
	}

	if err := runSetupWithFlags(t, dir, "--ide", "cursor", "--uninstall"); err != nil {
		t.Fatalf("setup --uninstall: %v", err)
	}
	if _, err := os.Stat(claudeMD); !os.IsNotExist(err) {
		t.Errorf("companion CLAUDE.md not removed by --uninstall; stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "mcp.json")); !os.IsNotExist(err) {
		t.Errorf("companion .claude/mcp.json not removed by --uninstall; stat err = %v", err)
	}
}

func fileMTime(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.ModTime().UnixNano()
}
