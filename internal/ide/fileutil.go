package ide

import (
	"os"
	"path/filepath"
	"strings"
)

var gitflowMarkers = []string{"Gitflow Pre-flight Check", "Gitflow Enforcement"}

// fileContainsGitflow checks if a file contains any gitflow instruction marker.
func fileContainsGitflow(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	for _, marker := range gitflowMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

// ensureFileWithGitflow is the DRY helper for all IDE generators that follow
// the "create-or-append" pattern:
//   - If the file doesn't exist, create it with headerIfNew + template
//   - If the file exists but has no gitflow section, append template
//   - If the file already has gitflow section, return path (idempotent)
//
// templateChoice: "full" uses GitflowInstructionsFull, anything else uses Compact.
func ensureFileWithGitflow(path, headerIfNew, templateChoice string) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	fullTemplate := GitflowInstructionsFull
	if templateChoice != "full" {
		fullTemplate = GitflowInstructionsCompact
	}

	existing, err := os.ReadFile(path)
	if err == nil {
		content := string(existing)
		for _, marker := range gitflowMarkers {
			if strings.Contains(content, marker) {
				if !hasCurrentVersionHeader(content) {
					updated := withVersionHeader(content)
					if err := os.WriteFile(path, []byte(updated), 0600); err != nil {
						return "", err
					}
				}
				return path, nil
			}
		}
		// Always use compact when appending to keep existing files concise
		content = withVersionHeader(content)
		content += "\n" + GitflowInstructionsCompact
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return "", err
		}
		return path, nil
	}

	content := withVersionHeader(headerIfNew + fullTemplate)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", err
	}
	return path, nil
}
