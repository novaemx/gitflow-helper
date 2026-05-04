package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureEmbeddedSkill_ProjectScopedIDE(t *testing.T) {
	SetGeneratorVersion("1.2.3")

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
	if firstLine(string(data)) != "<!-- gitflow-version: 1.2.3 -->" {
		t.Fatalf("expected version header first line, got %q", firstLine(string(data)))
	}
	if !strings.Contains(string(data), "name: gitflow") {
		t.Fatal("expected embedded skill frontmatter")
	}
	if !strings.Contains(string(data), "Post-validation commit flow") {
		t.Fatal("expected embedded skill post-validation commit guidance")
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

// TestEnsureEmbeddedSkill_SkipsInDevRepo verifies that ensureEmbeddedSkill
// does NOT overwrite .agents/skills/gitflow/SKILL.md when the project root
// contains the source asset (internal/ide/assets/gitflow_skill.md), meaning
// we are inside the gitflow-helper development repository.
func TestEnsureEmbeddedSkill_SkipsInDevRepo(t *testing.T) {
	dir := t.TempDir()

	// Simulate the dev repo by creating the source asset marker.
	marker := filepath.Join(dir, "internal", "ide", "assets", "gitflow_skill.md")
	if err := os.MkdirAll(filepath.Dir(marker), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("dev asset"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create a custom SKILL.md with different content.
	skillDir := filepath.Join(dir, ".agents", "skills", "gitflow")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	custom := "custom dev content\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := ensureEmbeddedSkill(dir, IDECursor)
	if err != nil {
		t.Fatalf("ensureEmbeddedSkill in dev repo: %v", err)
	}
	if path != "" {
		t.Fatalf("expected no write in dev repo, got %s", path)
	}

	// Verify the file was NOT overwritten.
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Fatalf("expected custom content preserved, got %q", string(data))
	}
}
