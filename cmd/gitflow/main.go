package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/novaemx/gitflow-helper/internal/commands"
	"github.com/novaemx/gitflow-helper/internal/debug"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

// commit is injected at build time via -ldflags "-X main.commit=..."
var commit = ""

// buildDate is injected at build time via -ldflags "-X main.buildDate=..."
var buildDate = ""

var commitHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

func detectCommitHash() string {
	out, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output()
	if err != nil {
		return ""
	}
	hash := strings.TrimSpace(string(out))
	if !commitHashPattern.MatchString(hash) {
		return ""
	}
	return strings.ToLower(hash)
}

func normalizeCommitHash(input string) string {
	hash := strings.TrimSpace(input)
	if hash == "" || strings.EqualFold(hash, "none") || strings.EqualFold(hash, "unknown") {
		return ""
	}
	if !commitHashPattern.MatchString(hash) {
		return ""
	}
	if len(hash) > 12 {
		hash = hash[:12]
	}
	return strings.ToLower(hash)
}

func buildDisplayVersion(baseVersion, buildHash string) string {
	hash := normalizeCommitHash(buildHash)
	if hash == "" {
		return baseVersion
	}
	return baseVersion + " (build " + hash + ")"
}

func detectBuildDate() string {
	if exe, err := os.Executable(); err == nil {
		if info, statErr := os.Stat(exe); statErr == nil {
			return info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	return "unknown"
}

func normalizeBuildDate(input string) string {
	v := strings.TrimSpace(input)
	if v == "" {
		return "unknown"
	}
	if strings.EqualFold(v, "none") || strings.EqualFold(v, "unknown") {
		return "unknown"
	}
	return v
}

func main() {
	if version == "dev" {
		if data, err := os.ReadFile("VERSION"); err == nil {
			if v := strings.TrimSpace(string(data)); v != "" {
				version = v
			}
		}
	}

	hash := normalizeCommitHash(commit)
	if hash == "" {
		hash = detectCommitHash()
	}
	compiledAt := normalizeBuildDate(buildDate)
	if compiledAt == "unknown" {
		compiledAt = detectBuildDate()
	}

	root := commands.NewRootCmd(version)
	root.Version = buildDisplayVersion(version, hash)
	debug.SetBuildInfo(root.Version, compiledAt, hash)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}

	// Print timing report if debug is enabled
	debug.PrintTimings()
}
