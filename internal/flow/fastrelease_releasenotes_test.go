package flow

import (
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/git"
)

func setupFastRepo(t *testing.T) (string, config.FlowConfig) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := osexec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %v\n%s", args, err, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "initial commit")
	run("git", "branch", "develop")
	run("git", "checkout", "develop")
	run("git", "checkout", "-b", "feature/fast-one")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run("git", "add", "f.txt")
	run("git", "commit", "-m", "feat: add fast feature")

	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.0\n"), 0644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}

	cfg := config.FlowConfig{ProjectRoot: dir, MainBranch: "main", DevelopBranch: "develop", Remote: "", TagPrefix: "v", VersionFile: "VERSION"}
	git.ProjectRoot = dir
	return dir, cfg
}

func TestFastRelease_SuccessFeature(t *testing.T) {
	_, cfg := setupFastRepo(t)
	code, result := FastRelease(cfg, "fast-one")
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %v", code, result)
	}
	if result["result"] != "ok" {
		t.Fatalf("expected result ok, got %v", result["result"])
	}
	if result["tag"] != "v1.0.0" {
		t.Fatalf("expected tag v1.0.0, got %v", result["tag"])
	}
}

func TestWriteReleaseNotes_WritesFileAndContent(t *testing.T) {
	dir, cfg := setupFastRepo(t)
	meta := WriteReleaseNotes(cfg, "")
	if meta == nil {
		t.Fatal("expected metadata from WriteReleaseNotes")
	}
	path, _ := meta["file"].(string)
	if path == "" {
		t.Fatal("expected notes file path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read release notes: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Release") {
		t.Fatalf("expected Release heading, got %q", content)
	}
	if !strings.Contains(content, "What's New") && !strings.Contains(content, "Changes") {
		t.Fatalf("expected notes sections, got %q", content)
	}
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("expected notes under repo root, got %q", path)
	}
}
