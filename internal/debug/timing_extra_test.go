package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── Level.String ──────────────────────────────────────────────────────────

func TestLevel_String(t *testing.T) {
	cases := []struct {
		level Level
		want  string
	}{
		{LevelOff, "off"},
		{LevelLog, "log"},
		{LevelDebug, "debug"},
	}
	for _, tc := range cases {
		got := tc.level.String()
		if got != tc.want {
			t.Fatalf("Level(%d).String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}

// ── Start / End timing ────────────────────────────────────────────────────

func TestStart_End_RecordTimingWhenDebugEnabled(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, false, true) // debug mode

	end := Start("test-operation")
	time.Sleep(time.Millisecond)
	end()

	entries := Timings()
	if len(entries) == 0 {
		t.Fatal("expected at least one timing entry")
	}
	found := false
	for _, e := range entries {
		if e.Name == "test-operation" {
			found = true
			if e.Duration <= 0 {
				t.Fatalf("expected positive duration, got %v", e.Duration)
			}
		}
	}
	if !found {
		t.Fatalf("timing entry 'test-operation' not found in %v", entries)
	}
}

func TestStart_ReturnsNoopWhenDebugDisabled(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	// LevelOff — start should return no-op func and record nothing.
	end := Start("no-op-operation")
	end()

	entries := Timings()
	for _, e := range entries {
		if e.Name == "no-op-operation" {
			t.Fatal("expected no timing entry when debug is disabled")
		}
	}
}

func TestEnd_NoopWhenNoMatchingStart(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, false, true) // debug mode

	// End without Start should not panic or record an entry.
	End("never-started")

	entries := Timings()
	for _, e := range entries {
		if e.Name == "never-started" {
			t.Fatal("expected no entry for End without Start")
		}
	}
}

// ── PrintTimings ──────────────────────────────────────────────────────────

func TestPrintTimings_WritesToStderrWhenDebugEnabled(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, false, true)

	end := Start("timed-section")
	end()

	// Redirect stderr to a temporary file to capture output.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	PrintTimings()
	w.Close()
	os.Stderr = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "TIMING REPORT") {
		t.Fatalf("expected TIMING REPORT in output, got: %q", output)
	}
	if !strings.Contains(output, "timed-section") {
		t.Fatalf("expected timed-section in output, got: %q", output)
	}
}

func TestPrintTimings_NoopWhenDisabled(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)
	// LevelOff — PrintTimings should not write anything.
	PrintTimings() // should not panic
}

func TestPrintTimings_EmptyTimings_NoOutput(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, false, true)
	// No Start/End calls — timings slice is empty.

	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	PrintTimings()
	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Fatalf("expected no output for empty timings, got: %q", string(buf[:n]))
	}
}

// ── Configure reopens same file ───────────────────────────────────────────

func TestConfigure_ReopensExistingLogFile(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, true, false)
	Logf("first line")

	// Call Configure again with the same path — should reuse existing file.
	Configure(root, true, false)
	Logf("second line")

	path := filepath.Join(root, ".gitflow", "logs", "gitflow.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "first line") || !strings.Contains(content, "second line") {
		t.Fatalf("expected both log lines, got: %q", content)
	}
}

// ── Printf (debug level) ──────────────────────────────────────────────────

func TestPrintf_WritesToStderrWhenDebugEnabled(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, false, true)

	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	Printf("debug value=%d", 42)
	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "debug value=42") {
		t.Fatalf("expected debug output, got: %q", output)
	}
}

func TestPrintf_NoopWhenLogOnly(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, true, false) // log only, not debug

	// Printf requires LevelDebug — should produce no output at LevelLog.
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	Printf("should-not-appear")
	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	if strings.Contains(string(buf[:n]), "should-not-appear") {
		t.Fatal("Printf should not output when only log level is enabled")
	}
}

// ── IsEnabled ────────────────────────────────────────────────────────────

func TestIsEnabled_ReturnsFalseByDefault(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	if IsEnabled() {
		t.Fatal("expected debug disabled by default")
	}
}

func TestIsEnabled_TrueWhenConfigured(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, false, true)

	if !IsEnabled() {
		t.Fatal("expected debug enabled after Configure(debug=true)")
	}
}
