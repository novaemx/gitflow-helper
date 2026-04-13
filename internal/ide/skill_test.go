package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureEmbeddedSkill_ProjectScopedIDE(t *testing.T) {
	dir := t.TempDir()

	path, err := ensureEmbeddedSkill(dir, IDECursor)
	if err != nil {
		t.Fatalf("ensureEmbeddedSkill: %v", err)
	}

	expected := filepath.Join(dir, ".agents", "skills", "gitflow", "SKILL.md")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(data), "name: gitflow") {
		t.Fatal("expected embedded skill frontmatter")
	}

	path2, err := ensureEmbeddedSkill(dir, IDECursor)
	if err != nil {
		t.Fatalf("second ensureEmbeddedSkill: %v", err)
	}
	if path2 != "" {
		t.Fatalf("expected no update on second call, got %s", path2)
	}
}

func TestEnsureEmbeddedSkill_UserFallback(t *testing.T) {
	tmpHome := t.TempDir()
	prev := UserHomeDirFunc
	UserHomeDirFunc = func() (string, error) { return tmpHome, nil }
	defer func() { UserHomeDirFunc = prev }()

	path, err := ensureEmbeddedSkill(t.TempDir(), IDEUnknown)
	if err != nil {
		t.Fatalf("ensureEmbeddedSkill: %v", err)
	}

	expected := filepath.Join(tmpHome, ".agents", "skills", "gitflow", "SKILL.md")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}
}
