package commands

import "testing"

func TestStartFailureLines_IncludeHintAndDiagnostics(t *testing.T) {
	lines := startFailureLines(map[string]any{
		"error": "could not auto-detect release version",
		"hint":  "pass an explicit semantic version or configure VERSION",
		"diagnostics": []string{
			"version_file=VERSION value=not-a-semver",
			"latest_semver_tag=none",
		},
	})

	if len(lines) < 4 {
		t.Fatalf("expected multiple failure lines, got %v", lines)
	}
	if lines[0] != "could not auto-detect release version" {
		t.Fatalf("expected first line to be the error, got %q", lines[0])
	}
	if lines[1] != "Hint: pass an explicit semantic version or configure VERSION" {
		t.Fatalf("unexpected hint line: %q", lines[1])
	}
	if lines[2] != "Diagnostics:" {
		t.Fatalf("expected diagnostics header, got %q", lines[2])
	}
	if lines[3] != "- version_file=VERSION value=not-a-semver" {
		t.Fatalf("unexpected diagnostic line: %q", lines[3])
	}
}

func TestNewRootCmd_HasTroubleshootingFlags(t *testing.T) {
	root := NewRootCmd("test")
	if root.PersistentFlags().Lookup("log") == nil {
		t.Fatal("expected persistent --log flag")
	}
	if root.PersistentFlags().Lookup("debug") == nil {
		t.Fatal("expected persistent --debug flag")
	}
}
