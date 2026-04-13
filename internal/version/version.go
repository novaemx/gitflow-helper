package version

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/luis-lozano/gitflow-helper/internal/config"
	"github.com/luis-lozano/gitflow-helper/internal/git"
)

func ReadVersion(cfg config.FlowConfig) string {
	if cfg.VersionFile == "" {
		tag := git.LatestTag()
		if tag != "none" {
			return strings.TrimLeft(tag, "v")
		}
		return "0.0.0"
	}
	path := filepath.Join(cfg.ProjectRoot, cfg.VersionFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "0.0.0"
	}
	if cfg.VersionFile == "VERSION" {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			return strings.TrimPrefix(line, "v")
		}
		return "0.0.0"
	}
	re, err := regexp.Compile(cfg.VersionPattern)
	if err != nil {
		return "0.0.0"
	}
	m := re.FindStringSubmatch(string(data))
	if len(m) > 1 {
		return m[1]
	}
	return "0.0.0"
}

func SuggestVersion(cfg config.FlowConfig, bumpType string) string {
	ver := ReadVersion(cfg)
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(ver)
	if len(m) < 4 {
		return "0.0.1"
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])

	switch bumpType {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	default:
		patch++
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

func WriteVersionFile(cfg config.FlowConfig, ver string) {
	if cfg.VersionFile == "" {
		return
	}
	ver = strings.TrimPrefix(ver, "v")
	path := filepath.Join(cfg.ProjectRoot, cfg.VersionFile)

	if cfg.VersionFile == "VERSION" {
		_ = os.WriteFile(path, []byte(ver+"\n"), 0644)
		_ = git.Exec("add", cfg.VersionFile)
		_ = git.Exec("commit", "-m", fmt.Sprintf("chore: bump version to %s", ver))
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	re, err := regexp.Compile(cfg.VersionPattern)
	if err != nil {
		return
	}
	loc := re.FindSubmatchIndex(data)
	if len(loc) >= 4 {
		updated := string(data[:loc[2]]) + ver + string(data[loc[3]:])
		_ = os.WriteFile(path, []byte(updated), 0644)
		_ = git.Exec("add", cfg.VersionFile)
		_ = git.Exec("commit", "-m", fmt.Sprintf("chore: bump version to %s", ver))
	}
}

func RunBumpCommand(cfg config.FlowConfig, part string) {
	if cfg.BumpCommand == "" {
		return
	}
	cmd := cfg.BumpCommand
	if strings.Contains(cmd, "{part}") {
		cmd = strings.ReplaceAll(cmd, "{part}", part)
	} else if part != "patch" {
		cmd = fmt.Sprintf("%s --%s", cmd, part)
	}
	_ = git.Run(cmd)
}

func RunBuildBumpCommand(cfg config.FlowConfig) {
	if cfg.BuildBumpCommand == "" {
		return
	}
	plat := "mac"
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		plat = "win"
	}
	cmd := strings.ReplaceAll(cfg.BuildBumpCommand, "{platform}", plat)
	_ = git.Run(cmd)
}
