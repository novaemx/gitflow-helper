package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
	"github.com/luis-lozano/gitflow-helper/internal/output"
	"github.com/luis-lozano/gitflow-helper/internal/version"
)

func getPreviousTag(currentTag string) string {
	tags := git.RunLines("git tag --sort=-version:refname")
	found := false
	for _, t := range tags {
		if found {
			return t
		}
		if t == currentTag {
			found = true
		}
	}
	return ""
}

func classifyCommit(subject string) string {
	s := strings.ToLower(subject)
	chorePrefixes := []string{"chore:", "chore(", "ci:", "ci(", "build:", "build(", "deps:"}
	for _, p := range chorePrefixes {
		if strings.HasPrefix(s, p) {
			return "maintenance"
		}
	}
	if strings.Contains(s, "bump version") || strings.Contains(s, "bump to") {
		return "maintenance"
	}
	featKW := []string{"feat", "add ", "new ", "implement", "introduce"}
	for _, kw := range featKW {
		if strings.Contains(s, kw) {
			return "features"
		}
	}
	fixKW := []string{"fix", "bug", "patch", "hotfix", "resolve", "correct"}
	for _, kw := range fixKW {
		if strings.Contains(s, kw) {
			return "fixes"
		}
	}
	improveKW := []string{"improve", "enhance", "optim", "refactor", "perf", "update", "upgrade"}
	for _, kw := range improveKW {
		if strings.Contains(s, kw) {
			return "improvements"
		}
	}
	return "other"
}

func generateReleaseNotes(cfg config.FlowConfig, fromRef, toRef, ver string) string {
	entries := git.RunLines(fmt.Sprintf("git log --format='%%s' %s..%s", fromRef, toRef))

	groups := map[string][]string{
		"features":     {},
		"fixes":        {},
		"improvements": {},
		"maintenance":  {},
		"other":        {},
	}

	cleanRe := regexp.MustCompile(`(?i)^(feat|fix|chore|refactor|perf|docs|style|test|ci|build|improvement)(\(.*?\))?:\s*`)

	for _, subj := range entries {
		subj = strings.TrimSpace(subj)
		if subj == "" {
			continue
		}
		cat := classifyCommit(subj)
		clean := cleanRe.ReplaceAllString(subj, "")
		clean = strings.TrimSpace(clean)
		if len(clean) > 0 {
			clean = strings.ToUpper(clean[:1]) + clean[1:]
		}
		if clean != "" {
			exists := false
			for _, existing := range groups[cat] {
				if existing == clean {
					exists = true
					break
				}
			}
			if !exists {
				groups[cat] = append(groups[cat], clean)
			}
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("# Release %s\n", ver))
	lines = append(lines, fmt.Sprintf("**Date:** %s\n", time.Now().Format("2006-01-02")))

	hasContent := false
	if len(groups["features"]) > 0 {
		hasContent = true
		lines = append(lines, "## What's New\n")
		for _, item := range groups["features"] {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}
	if len(groups["fixes"]) > 0 {
		hasContent = true
		lines = append(lines, "## Bug Fixes\n")
		for _, item := range groups["fixes"] {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}
	if len(groups["improvements"]) > 0 {
		hasContent = true
		lines = append(lines, "## Improvements\n")
		for _, item := range groups["improvements"] {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}
	if !hasContent {
		other := append(groups["maintenance"], groups["other"]...)
		if len(other) > 0 {
			lines = append(lines, "## Changes\n")
			for _, item := range other {
				lines = append(lines, "- "+item)
			}
			lines = append(lines, "")
		} else {
			lines = append(lines, "_No user-facing changes in this release._\n")
		}
	}
	return strings.Join(lines, "\n")
}

func WriteReleaseNotes(cfg config.FlowConfig, fromTag string) map[string]any {
	currentVer := version.ReadVersion(cfg)
	tag := git.LatestTag()

	var fromRef string
	if fromTag != "" {
		fromRef = fromTag
	} else if tag != "none" {
		prev := getPreviousTag(tag)
		if prev != "" {
			fromRef = prev
		} else {
			fromRef = tag
		}
	}

	if fromRef == "" {
		entries := git.RunLines("git log --format='%s' -n 50")
		if len(entries) == 0 {
			return nil
		}
		fromRef = git.RunQuiet("git rev-list --max-parents=0 HEAD")
	}

	toRef := "HEAD"
	ver := currentVer
	if ver == "0.0.0" {
		if tag != "none" {
			ver = strings.TrimLeft(tag, "v")
		} else {
			ver = "unreleased"
		}
	}

	content := generateReleaseNotes(cfg, fromRef, toRef, ver)
	notesPath := filepath.Join(cfg.ProjectRoot, "RELEASE_NOTES.md")
	_ = os.WriteFile(notesPath, []byte(content+"\n"), 0644)

	return map[string]any{
		"version":  ver,
		"from_ref": fromRef,
		"to_ref":   toRef,
		"file":     notesPath,
		"content":  content,
	}
}

func PrintReleaseNotes(meta map[string]any) {
	output.Infof("\n  %sRelease Notes for %s%s", output.Bold, meta["version"], output.Reset)
	output.Infof("  %sRange: %s..%s%s\n", output.Dim, meta["from_ref"], meta["to_ref"], output.Reset)

	content, _ := meta["content"].(string)
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# ") {
			output.Infof("  %s%s%s%s", output.Bold, output.Green, line, output.Reset)
		} else if strings.HasPrefix(line, "## ") {
			output.Infof("  %s%s%s%s", output.Bold, output.Cyan, line, output.Reset)
		} else if strings.HasPrefix(line, "- ") {
			output.Infof("    %s", line)
		} else {
			output.Infof("  %s", line)
		}
	}
	output.Infof("\n  %sWritten to:%s %s", output.Green, output.Reset, meta["file"])
}
