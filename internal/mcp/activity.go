package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// ReadActivityLog reads the last N entries from the shared MCP activity log.
func ReadActivityLog(projectRoot string, maxEntries int) []ActivityEntry {
	path := ActivityLogPath(projectRoot)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []ActivityEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e ActivityEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			entries = append(entries, e)
		}
	}

	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	return entries
}

// AppendActivityLog appends an activity entry to the shared activity log.
// It is intentionally generic so CLI and TUI actions can be tracked too.
func AppendActivityLog(projectRoot string, entry ActivityEntry) error {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	path := ActivityLogPath(projectRoot)
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(line)
	return err
}
