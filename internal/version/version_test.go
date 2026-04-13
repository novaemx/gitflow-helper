package version

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
)

func TestReadVersionFromVERSIONFile(t *testing.T) {
	dir := t.TempDir()
	cfg := config.FlowConfig{
		ProjectRoot:    dir,
		VersionFile:    "VERSION",
		VersionPattern: `(?:__version__|"version")\s*[:=]\s*"([^"]+)"`,
	}

	writeFile(t, filepath.Join(dir, "VERSION"), "1.2.3\n")

	if got := ReadVersion(cfg); got != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", got)
	}
}

func TestReadVersionFromVERSIONFileWithVPrefix(t *testing.T) {
	dir := t.TempDir()
	cfg := config.FlowConfig{
		ProjectRoot:    dir,
		VersionFile:    "VERSION",
		VersionPattern: `(?:__version__|"version")\s*[:=]\s*"([^"]+)"`,
	}

	writeFile(t, filepath.Join(dir, "VERSION"), "v2.0.1\n")

	if got := ReadVersion(cfg); got != "2.0.1" {
		t.Fatalf("expected 2.0.1, got %q", got)
	}
}

func TestSuggestVersionBumps(t *testing.T) {
	dir := t.TempDir()
	cfg := config.FlowConfig{ProjectRoot: dir, VersionFile: "VERSION", VersionPattern: `(?:__version__|"version")\s*[:=]\s*"([^"]+)"`}
	writeFile(t, filepath.Join(dir, "VERSION"), "1.2.3\n")

	if got := SuggestVersion(cfg, "patch"); got != "1.2.4" {
		t.Fatalf("patch bump expected 1.2.4, got %q", got)
	}
	if got := SuggestVersion(cfg, "minor"); got != "1.3.0" {
		t.Fatalf("minor bump expected 1.3.0, got %q", got)
	}
	if got := SuggestVersion(cfg, "major"); got != "2.0.0" {
		t.Fatalf("major bump expected 2.0.0, got %q", got)
	}
}

func TestReadVersionMissingFileReturnsDefault(t *testing.T) {
	cfg := config.FlowConfig{
		ProjectRoot:    t.TempDir(),
		VersionFile:    "VERSION",
		VersionPattern: `(?:__version__|"version")\s*[:=]\s*"([^"]+)"`,
	}
	if got := ReadVersion(cfg); got != "0.0.0" {
		t.Fatalf("expected 0.0.0 for missing file, got %q", got)
	}
}

func TestWriteVersionFileForVERSION(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	oldRoot := git.ProjectRoot
	t.Cleanup(func() { git.ProjectRoot = oldRoot })
	git.ProjectRoot = dir

	writeFile(t, filepath.Join(dir, "VERSION"), "1.0.0\n")
	runGit(t, dir, "add", "VERSION")
	runGit(t, dir, "commit", "-m", "init")

	cfg := config.FlowConfig{ProjectRoot: dir, VersionFile: "VERSION", VersionPattern: `(?:__version__|"version")\s*[:=]\s*"([^"]+)"`}
	WriteVersionFile(cfg, "1.1.0")

	data, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	if strings.TrimSpace(string(data)) != "1.1.0" {
		t.Fatalf("expected VERSION 1.1.0, got %q", strings.TrimSpace(string(data)))
	}
}

func TestWriteVersionFileWithPattern(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	oldRoot := git.ProjectRoot
	t.Cleanup(func() { git.ProjectRoot = oldRoot })
	git.ProjectRoot = dir

	file := filepath.Join(dir, "package.json")
	writeFile(t, file, "{\"version\":\"0.9.0\"}\n")
	runGit(t, dir, "add", "package.json")
	runGit(t, dir, "commit", "-m", "init")

	cfg := config.FlowConfig{
		ProjectRoot:    dir,
		VersionFile:    "package.json",
		VersionPattern: `(?:__version__|"version")\s*[:=]\s*"([^"]+)"`,
	}
	WriteVersionFile(cfg, "1.0.0")

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !strings.Contains(string(data), "1.0.0") {
		t.Fatalf("expected updated version in package.json, got %s", string(data))
	}
}

func TestRunBumpCommandsDoNotError(t *testing.T) {
	dir := t.TempDir()
	oldRoot := git.ProjectRoot
	t.Cleanup(func() { git.ProjectRoot = oldRoot })
	git.ProjectRoot = dir

	RunBumpCommand(config.FlowConfig{BumpCommand: "echo bump-{part}"}, "minor")
	RunBuildBumpCommand(config.FlowConfig{BuildBumpCommand: "echo build-{platform}"})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
