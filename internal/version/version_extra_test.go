package version

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	igit "github.com/novaemx/gitflow-helper/internal/git"
)

func setupGitRepo(t *testing.T) string {
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
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "initial commit")
	return dir
}

func TestReadVersion_FromTag(t *testing.T) {
	dir := setupGitRepo(t)

	// create annotated tag v1.2.3
	cmd := exec.Command("git", "tag", "-a", "v1.2.3", "-m", "v1.2.3")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tag failed: %v\n%s", err, out)
	}

	oldRoot := igit.ProjectRoot
	t.Cleanup(func() { igit.ProjectRoot = oldRoot })
	igit.ProjectRoot = dir

	cfg := config.FlowConfig{ProjectRoot: dir, VersionFile: ""}
	got := ReadVersion(cfg)
	if got != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", got)
	}
}

func TestReadVersion_InvalidRegexReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "meta.txt")
	if err := os.WriteFile(f, []byte("version = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg := config.FlowConfig{ProjectRoot: dir, VersionFile: "meta.txt", VersionPattern: "("}
	got := ReadVersion(cfg)
	if got != "0.0.0" {
		t.Fatalf("expected default 0.0.0 for invalid regex, got %q", got)
	}
}

type mockGitClientCalls struct {
	calls [][]string
}

func (m *mockGitClientCalls) Exec(args ...string) error {
	m.calls = append(m.calls, append([]string(nil), args...))
	return nil
}
func (m *mockGitClientCalls) ExecResult(args ...string) (int, string, string) {
	m.calls = append(m.calls, append([]string(nil), args...))
	return 0, "", ""
}
func (m *mockGitClientCalls) ExecQuiet(args ...string) string   { m.ExecResult(args...); return "" }
func (m *mockGitClientCalls) ExecLines(args ...string) []string { return nil }

func TestRunBumpCommand_Various(t *testing.T) {
	mock := &mockGitClientCalls{}
	old := igit.DefaultClient()
	igit.SetDefaultClient(mock)
	defer igit.SetDefaultClient(old)

	// substitution path
	RunBumpCommand(config.FlowConfig{BumpCommand: "echo bump-{part}"}, "minor")
	found := false
	for _, c := range mock.calls {
		if len(c) >= 2 && c[0] == "echo" && strings.Contains(c[1], "bump-minor") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected substitution bump call, got %v", mock.calls)
	}

	mock.calls = nil
	// append flag path (no {part})
	RunBumpCommand(config.FlowConfig{BumpCommand: "echo bump"}, "minor")
	foundFlag := false
	for _, c := range mock.calls {
		for _, a := range c {
			if a == "--minor" {
				foundFlag = true
			}
		}
	}
	if !foundFlag {
		t.Fatalf("expected --minor flag in calls, got %v", mock.calls)
	}
}

func TestRunBuildBumpCommand_PlatformReplacement(t *testing.T) {
	mock := &mockGitClientCalls{}
	old := igit.DefaultClient()
	igit.SetDefaultClient(mock)
	defer igit.SetDefaultClient(old)

	// Force Windows-like platform
	oldOS := os.Getenv("OS")
	t.Cleanup(func() { _ = os.Setenv("OS", oldOS) })
	_ = os.Setenv("OS", "Windows_NT")

	RunBuildBumpCommand(config.FlowConfig{BuildBumpCommand: "echo build-{platform}"})
	found := false
	for _, c := range mock.calls {
		for _, a := range c {
			if strings.Contains(a, "build-win") || strings.Contains(a, "build-wi") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected build-win in calls, got %v", mock.calls)
	}
}
