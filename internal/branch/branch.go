package branch

import "strings"

// TypeOf classifies a git branch name into a gitflow type.
// Returns: "feature", "bugfix", "release", "hotfix", "base", or "other".
// This package has zero dependencies -- safe to import from any internal package.
func TypeOf(name string) string {
	for _, p := range []struct{ prefix, btype string }{
		{"feature/", "feature"},
		{"bugfix/", "bugfix"},
		{"release/", "release"},
		{"hotfix/", "hotfix"},
	} {
		if strings.HasPrefix(name, p.prefix) {
			return p.btype
		}
	}
	if name == "develop" || name == "main" || name == "master" {
		return "base"
	}
	return "other"
}
