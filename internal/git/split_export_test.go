package git

import "testing"

func TestSplitCommand_ExportedSimple(t *testing.T) {
	args := SplitCommand("git checkout main")
	if len(args) != 3 || args[0] != "git" || args[1] != "checkout" || args[2] != "main" {
		t.Fatalf("unexpected: %v", args)
	}
}

func TestSplitCommand_ExportedQuoted(t *testing.T) {
	args := SplitCommand(`git commit -m "chore: bump version"`)
	if len(args) != 4 || args[3] != "chore: bump version" {
		t.Fatalf("unexpected: %v", args)
	}
}
