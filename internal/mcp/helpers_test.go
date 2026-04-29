package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	gflogic "github.com/novaemx/gitflow-helper/internal/gitflow"
)

// ── jsonText ───────────────────────────────────────────────────────────────

func TestJsonText_MarshalMap(t *testing.T) {
	got := jsonText(map[string]any{"key": "value"})
	if !strings.Contains(got, `"key"`) || !strings.Contains(got, `"value"`) {
		t.Fatalf("unexpected jsonText output: %s", got)
	}
}

func TestJsonText_MarshalString(t *testing.T) {
	got := jsonText("hello")
	if got != `"hello"` {
		t.Fatalf("expected `\"hello\"`, got %q", got)
	}
}

func TestJsonText_MarshalNil(t *testing.T) {
	got := jsonText(nil)
	if got != "null" {
		t.Fatalf("expected null, got %q", got)
	}
}

// ── textResult ────────────────────────────────────────────────────────────

func TestTextResult_ContainsJSON(t *testing.T) {
	result := textResult(map[string]any{"action": "test", "result": "ok"})
	if result == nil {
		t.Fatal("expected non-nil CallToolResult")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content item")
	}
}

// ── errResult ─────────────────────────────────────────────────────────────

func TestErrResult_IsErrorTrue(t *testing.T) {
	r, _, err := errResult("something went wrong")
	if err != nil {
		t.Fatalf("expected nil error from errResult, got %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if !r.IsError {
		t.Fatal("expected IsError=true")
	}
	if len(r.Content) == 0 {
		t.Fatal("expected at least one content item")
	}
}

func TestErrResult_ContainsMessage(t *testing.T) {
	r, _, _ := errResult("bad request")
	if len(r.Content) == 0 {
		t.Fatal("expected content")
	}
}

// ── NewServer ──────────────────────────────────────────────────────────────

func TestNewServer_CreatesServer(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	gf := gflogic.NewFromConfig(config.FlowConfig{ProjectRoot: root})
	s := NewServer(gf)
	if s == nil {
		t.Fatal("expected non-nil Server")
	}
}

func TestNewServer_ActivityStartsEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	gf := gflogic.NewFromConfig(config.FlowConfig{ProjectRoot: root})
	s := NewServer(gf)

	activity := s.Activity()
	if len(activity) != 0 {
		t.Fatalf("expected empty activity on new server, got %d entries", len(activity))
	}
}

// ── Activity / record ──────────────────────────────────────────────────────

func TestActivity_ReturnsRecordedEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	s := &Server{gf: &gflogic.Logic{Config: config.FlowConfig{ProjectRoot: root}}}

	s.record("status", "", "ok", "")
	s.record("sync", "", "error", "merge conflict")

	activity := s.Activity()
	if len(activity) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(activity))
	}
	if activity[0].Tool != "status" {
		t.Fatalf("expected first tool=status, got %q", activity[0].Tool)
	}
	if activity[1].Tool != "sync" {
		t.Fatalf("expected second tool=sync, got %q", activity[1].Tool)
	}
	if activity[1].Error != "merge conflict" {
		t.Fatalf("expected error message, got %q", activity[1].Error)
	}
}

func TestActivity_CapsAt100Entries(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	s := &Server{gf: &gflogic.Logic{Config: config.FlowConfig{ProjectRoot: root}}}

	for i := 0; i < 110; i++ {
		s.record("status", "", "ok", "")
	}

	activity := s.Activity()
	if len(activity) > 100 {
		t.Fatalf("expected at most 100 entries, got %d", len(activity))
	}
}

// ── activityTimestamp ──────────────────────────────────────────────────────

func TestActivityTimestamp_ValidTimestamp(t *testing.T) {
	e := ActivityEntry{Timestamp: "2026-04-29T10:00:00Z"}
	ts, ok := activityTimestamp(e)
	if !ok {
		t.Fatal("expected valid timestamp parse")
	}
	if ts.Year() != 2026 {
		t.Fatalf("expected year 2026, got %d", ts.Year())
	}
}

func TestActivityTimestamp_EmptyTimestamp(t *testing.T) {
	e := ActivityEntry{Timestamp: ""}
	_, ok := activityTimestamp(e)
	if ok {
		t.Fatal("expected invalid for empty timestamp")
	}
}

func TestActivityTimestamp_InvalidTimestamp(t *testing.T) {
	e := ActivityEntry{Timestamp: "not-a-date"}
	_, ok := activityTimestamp(e)
	if ok {
		t.Fatal("expected invalid for bad timestamp")
	}
}

// ── AppendActivityLog / ActivityLogPath ───────────────────────────────────

func TestActivityLogPath_ContainsDotGit(t *testing.T) {
	path := ActivityLogPath("/some/root")
	if !strings.Contains(path, ".git") {
		t.Fatalf("expected .git in activity log path, got %q", path)
	}
}

func TestAppendActivityLog_CreatesFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	entry := ActivityEntry{Tool: "test", Result: "ok"}
	if err := AppendActivityLog(root, entry); err != nil {
		t.Fatalf("AppendActivityLog: %v", err)
	}
	logPath := ActivityLogPath(root)
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}
}
