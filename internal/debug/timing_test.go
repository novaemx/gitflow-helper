package debug

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var logFileNamePattern = regexp.MustCompile(`^log-\d{8}-\d{6}\.txt$`)

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
	buildVersion = "unknown"
	buildDate = "unknown"
	buildCommit = "unknown"
}

func currentLogPathForTest(t *testing.T, root string) string {
	t.Helper()

	mu.Lock()
	path := logFilePath
	mu.Unlock()

	if path == "" {
		t.Fatal("expected log file path to be set")
	}

	prefix := filepath.Join(root, ".gitflow") + string(os.PathSeparator)
	if !strings.HasPrefix(path, prefix) {
		t.Fatalf("expected log file under %q, got %q", prefix, path)
	}

	base := filepath.Base(path)
	if !logFileNamePattern.MatchString(base) {
		t.Fatalf("expected timestamped log filename, got %q", base)
	}

	return path
}

func TestConfigure_LogCreatesFileAndAppendsLogEntries(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	Configure(root, true, false)
	Logf("hello %s", "world")

	path := currentLogPathForTest(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Log capture started at") {
		t.Fatalf("expected log file to contain capture start header, got %q", content)
	}
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

	path := currentLogPathForTest(t, root)
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

func TestConfigure_HeaderIncludesRuntimeMetadata(t *testing.T) {
	resetDebugStateForTest(t)
	defer resetDebugStateForTest(t)

	root := t.TempDir()
	SetBuildInfo("1.2.3", "2026-05-17T00:00:00Z", "abc1234")
	Configure(root, true, false)

	path := currentLogPathForTest(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "gitflow_version=1.2.3") {
		t.Fatalf("expected version metadata in header, got %q", content)
	}
	if !strings.Contains(content, "gitflow_build_date=2026-05-17T00:00:00Z") {
		t.Fatalf("expected build date metadata in header, got %q", content)
	}
	if !strings.Contains(content, "gitflow_build_commit=abc1234") {
		t.Fatalf("expected build commit metadata in header, got %q", content)
	}
	if !strings.Contains(content, "os=") {
		t.Fatalf("expected os metadata in header, got %q", content)
	}
	if !strings.Contains(content, "cpu_cores=") {
		t.Fatalf("expected cpu metadata in header, got %q", content)
	}
	if !strings.Contains(content, "ram_total=") {
		t.Fatalf("expected ram metadata in header, got %q", content)
	}
}
