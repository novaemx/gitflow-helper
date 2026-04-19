package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"time"
)

func activityTimestamp(entry ActivityEntry) (time.Time, bool) {
	if entry.Timestamp == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, entry.Timestamp)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

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

	sort.SliceStable(entries, func(i, j int) bool {
		left, leftOK := activityTimestamp(entries[i])
		right, rightOK := activityTimestamp(entries[j])
		switch {
		case leftOK && rightOK && !left.Equal(right):
			return left.After(right)
		case leftOK != rightOK:
			return leftOK
		default:
			return i > j
		}
	})

	if maxEntries > 0 && len(entries) > maxEntries {
		entries = entries[:maxEntries]
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
