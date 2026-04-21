package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func resetDebugStateForTest(t *testing.T) {
	t.Helper()

	mu.Lock()
	defer mu.Unlock()

	level = LevelOff
	timings = nil
	markers = make(map[string]time.Time)
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	logFilePath = ""
	configuredRoot = ""
}

func TestConfigure_LogCreatesFileAndAppendsLogEntries(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, true, false)
	Logf("hello %s", "world")

	path := filepath.Join(root, ".gitflow", "logs", "gitflow.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "[LOG] hello world") {
		t.Fatalf("expected log file to contain log line, got %q", content)
	}
	if IsDebugEnabled() {
		t.Fatal("expected debug mode to remain disabled")
	}
	if !IsLogEnabled() {
		t.Fatal("expected log mode to be enabled")
	}
}

func TestConfigure_DebugWritesDebugEntriesToLogFile(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, false, true)
	Logf("workflow step")
	Printf("git exit=%d", 0)

	path := filepath.Join(root, ".gitflow", "logs", "gitflow.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "[LOG] workflow step") {
		t.Fatalf("expected log file to contain log line, got %q", content)
	}
	if !strings.Contains(content, "[DEBUG] git exit=0") {
		t.Fatalf("expected log file to contain debug line, got %q", content)
	}
	if !IsDebugEnabled() {
		t.Fatal("expected debug mode to be enabled")
	}
}
