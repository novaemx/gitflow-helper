package tui

import (
	"runtime"
	"testing"
)

func TestBuildExecCmd_NoShellMetachars(t *testing.T) {
	cmd := BuildExecCmd("git status --porcelain", ".")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	if cmd.Path == "" {
		t.Fatalf("expected cmd.Path to be set, got empty")
	}
	// first arg should be the executable
	if len(cmd.Args) == 0 || cmd.Args[0] == "" {
		t.Fatalf("expected cmd.Args[0] to be executable, got %v", cmd.Args)
	}
}

func TestBuildExecCmd_ShellFallback(t *testing.T) {
	cmd := BuildExecCmd("echo foo | sed s/foo/bar/", ".")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	// On non-windows we expect sh -c fallback; on windows expect powershell -Command
	if runtime.GOOS == "windows" {
		if len(cmd.Args) < 2 || cmd.Args[0] != "powershell" {
			t.Fatalf("expected powershell fallback on windows, got %v", cmd.Args)
		}
	} else {
		if len(cmd.Args) < 2 || cmd.Args[0] != "sh" {
			t.Fatalf("expected sh fallback on unix, got %v", cmd.Args)
		}
	}
}

func TestNeedsShellCases(t *testing.T) {
	cases := map[string]bool{
		"echo foo":                  false,
		"echo \"$(date)\"":          true,
		"echo foo | sed s/foo/bar/": true,
		"echo foo > out.txt":        true,
		"git status":                false,
		"echo foo && echo bar":      true,
		"echo foo; echo bar":        true,
		"`rm -rf /`":                true,
	}
	for s, want := range cases {
		got := needsShell(s)
		if got != want {
			t.Fatalf("needsShell(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestBuildExecCmd_QuotedAndRedirects(t *testing.T) {
	cmd := BuildExecCmd("echo 'hello world' > out.txt", ".")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	if runtime.GOOS == "windows" {
		if len(cmd.Args) == 0 || cmd.Args[0] != "powershell" {
			t.Fatalf("expected powershell fallback on windows, got %v", cmd.Args)
		}
	} else {
		if len(cmd.Args) == 0 || cmd.Args[0] != "sh" {
			t.Fatalf("expected sh fallback on unix, got %v", cmd.Args)
		}
	}
}

func TestBuildExecCmd_EmptyString(t *testing.T) {
	cmd := BuildExecCmd("", ".")
	if cmd == nil {
		t.Fatal("expected non-nil cmd for empty input")
	}
}

func TestBuildExecCmd_CommandSubshell(t *testing.T) {
	cmd := BuildExecCmd("echo $(echo hi)", ".")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	if runtime.GOOS == "windows" {
		if len(cmd.Args) == 0 || cmd.Args[0] != "powershell" {
			t.Fatalf("expected powershell fallback on windows, got %v", cmd.Args)
		}
	} else {
		if len(cmd.Args) == 0 || cmd.Args[0] != "sh" {
			t.Fatalf("expected sh fallback on unix, got %v", cmd.Args)
		}
	}
}

func TestBuildExecCmd_SingleWord(t *testing.T) {
	cmd := BuildExecCmd("true", ".")
	if cmd == nil {
		t.Fatal("expected non-nil cmd for single word")
	}
}

func TestBuildExecCmd_QuotedArg(t *testing.T) {
	cmd := BuildExecCmd(`git commit -m "hello world"`, ".")
	if cmd == nil {
		t.Fatal("expected non-nil cmd for quoted arg")
	}
}
