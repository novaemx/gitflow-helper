package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
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
