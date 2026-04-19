package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/novaemx/gitflow-helper/internal/config"
	gflogic "github.com/novaemx/gitflow-helper/internal/gitflow"
)

func TestRecord_ConcurrentWritesProduceValidJSONL(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	s := &Server{gf: &gflogic.Logic{Config: config.FlowConfig{ProjectRoot: root}}}

	const total = 200
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		i := i
		go func() {
			defer wg.Done()
			s.record("status", "{}", "ok", "")
			_ = i
		}()
	}
	wg.Wait()

	logPath := ActivityLogPath(root)
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open activity log: %v", err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
		var e ActivityEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("line %d invalid json: %v", count, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan log: %v", err)
	}
	if count != total {
		t.Fatalf("expected %d log lines, got %d", total, count)
	}

	entries := ReadActivityLog(root, total+10)
	if len(entries) != total {
		t.Fatalf("expected %d parsed entries, got %d", total, len(entries))
	}
}

func TestReadActivityLog_ReturnsNewestFirst(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	log := strings.Join([]string{
		`{"tool":"first","timestamp":"2026-04-18T10:00:00Z","result":"ok"}`,
		`{"tool":"second","timestamp":"2026-04-18T11:00:00Z","result":"ok"}`,
		`{"tool":"third","timestamp":"2026-04-18T12:00:00Z","result":"ok"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(ActivityLogPath(root), []byte(log), 0644); err != nil {
		t.Fatalf("write activity log: %v", err)
	}

	entries := ReadActivityLog(root, 2)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Tool != "third" || entries[1].Tool != "second" {
		t.Fatalf("expected newest-first order, got %+v", entries)
	}
}
