package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoGitInit_WhenNotRepo(t *testing.T) {
	// Create an empty directory and run the root PersistentPreRun; after
	// it returns the directory should contain a .git folder (interactive path).
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	defer os.Chdir(oldwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	root := NewRootCmd("test")
	// Call the PersistentPreRun directly to exercise initialization.
	// Use an empty args list which triggers the interactive path.
	root.PersistentPreRun(root, []string{})

	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		t.Fatalf("expected .git to exist after auto-init")
	}
}
