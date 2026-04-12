package mcp

import (
	"bufio"
	"encoding/json"
	"os"
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
