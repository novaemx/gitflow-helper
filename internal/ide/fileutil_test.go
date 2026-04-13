package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureFileWithGitflow_CreateNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST.md")

	result, err := ensureFileWithGitflow(path, "# Test Header\n\n", "full")
	if err != nil {
		t.Fatal(err)
	}
	if result != path {
		t.Errorf("expected %s, got %s", path, result)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Test Header") {
		t.Error("missing header in new file")
	}
	if !strings.Contains(content, "Gitflow Pre-flight Check") {
		t.Error("missing gitflow instructions in new file")
	}
}

func TestEnsureFileWithGitflow_AppendExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST.md")
	if err := os.WriteFile(path, []byte("# Existing content\n\nSome text.\n"), 0644); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}

	_, err := ensureFileWithGitflow(path, "# Header\n\n", "compact")
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read appended file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Existing content") {
		t.Error("original content should be preserved")
	}
	if !strings.Contains(content, "Gitflow Enforcement") {
		t.Error("compact template should be appended")
	}
}

func TestEnsureFileWithGitflow_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "TEST.md")

	_, _ = ensureFileWithGitflow(path, "# Header\n\n", "full")
	data1, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read first render: %v", err)
	}

	_, _ = ensureFileWithGitflow(path, "# Header\n\n", "full")
	data2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read second render: %v", err)
	}

	if string(data1) != string(data2) {
		t.Error("second call should not modify file (idempotent)")
	}
}

func TestEnsureFileWithGitflow_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "TEST.md")

	_, err := ensureFileWithGitflow(path, "# Test\n\n", "compact")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("file should exist after creation")
	}
}

func TestFileContainsGitflow(t *testing.T) {
	dir := t.TempDir()

	path1 := filepath.Join(dir, "has.md")
	if err := os.WriteFile(path1, []byte("# Gitflow Enforcement\nRules here."), 0644); err != nil {
		t.Fatalf("write marker file: %v", err)
	}
	if !fileContainsGitflow(path1) {
		t.Error("expected true for file with marker")
	}

	path2 := filepath.Join(dir, "nope.md")
	if err := os.WriteFile(path2, []byte("# Just a file\nNo gitflow."), 0644); err != nil {
		t.Fatalf("write control file: %v", err)
	}
	if fileContainsGitflow(path2) {
		t.Error("expected false for file without marker")
	}

	if fileContainsGitflow(filepath.Join(dir, "nonexistent.md")) {
		t.Error("expected false for nonexistent file")
	}
}
