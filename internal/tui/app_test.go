package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveGitDir_DirectoryDotGit(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	got := resolveGitDir(root)
	if got != gitDir {
		t.Fatalf("expected %q, got %q", gitDir, got)
	}
}

func TestResolveGitDir_GitdirFile(t *testing.T) {
	root := t.TempDir()
	realGitDir := filepath.Join(root, ".worktrees", "wt1")
	if err := os.MkdirAll(realGitDir, 0755); err != nil {
		t.Fatalf("mkdir gitdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: .worktrees/wt1\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := resolveGitDir(root)
	if got != realGitDir {
		t.Fatalf("expected %q, got %q", realGitDir, got)
	}
}

func TestResolveGitDir_RejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	traversal := filepath.Join("..", filepath.Base(outside))
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+traversal+"\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := resolveGitDir(root)
	if got != "" {
		t.Fatalf("expected traversal path rejected, got %q", got)
	}
}

func TestResolveGitDir_AbsoluteGitdirPath(t *testing.T) {
	root := t.TempDir()
	absGitDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+absGitDir+"\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := resolveGitDir(root)
	if got != absGitDir {
		t.Fatalf("expected absolute gitdir %q, got %q", absGitDir, got)
	}
}

func TestResolveGitDir_InvalidGitFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not-a-gitdir-line\n"), 0644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := resolveGitDir(root)
	if got != "" {
		t.Fatalf("expected empty result for invalid git file, got %q", got)
	}
}

func TestRepoFingerprint_ChangesWhenHeadChanges(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0755); err != nil {
		t.Fatalf("mkdir refs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/develop\n"), 0644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "develop"), []byte("abc123\n"), 0644); err != nil {
		t.Fatalf("write develop ref: %v", err)
	}

	before := repoFingerprint(root)
	if before == "" {
		t.Fatal("expected non-empty fingerprint")
	}

	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatalf("rewrite HEAD: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte("def456\n"), 0644); err != nil {
		t.Fatalf("write main ref: %v", err)
	}

	after := repoFingerprint(root)
	if after == before {
		t.Fatal("expected fingerprint to change after branch head change")
	}
}

func TestSelectionIndexForRefresh_PreservesExactAction(t *testing.T) {
	actions := []action{
		{Tag: "pull", Label: "Pull latest"},
		{Tag: "finish", Label: "Finish bugfix"},
	}
	prev := action{Tag: "finish", Label: "Finish bugfix"}

	got := selectionIndexForRefresh(actions, &prev)
	if got != 1 {
		t.Fatalf("expected index 1, got %d", got)
	}
}

func TestSelectionIndexForRefresh_FallbackByTag(t *testing.T) {
	actions := []action{
		{Tag: "pull", Label: "Pull latest"},
		{Tag: "finish", Label: "Finish feature alpha"},
	}
	prev := action{Tag: "finish", Label: "Finish feature beta"}

	got := selectionIndexForRefresh(actions, &prev)
	if got != 1 {
		t.Fatalf("expected tag fallback index 1, got %d", got)
	}
}

func TestSelectionIndexForRefresh_DefaultRecommended(t *testing.T) {
	actions := []action{
		{Tag: "pull", Label: "Pull latest"},
		{Tag: "start", Label: "Start feature", Recommended: true},
		{Tag: "finish", Label: "Finish bugfix"},
	}
	prev := action{Tag: "unknown", Label: "Unknown"}

	got := selectionIndexForRefresh(actions, &prev)
	if got != 1 {
		t.Fatalf("expected recommended index 1, got %d", got)
	}
}
