package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type FlowConfig struct {
	Remote           string `json:"remote"`
	MainBranch       string `json:"main_branch"`
	DevelopBranch    string `json:"develop_branch"`
	VersionFile      string `json:"version_file"`
	VersionPattern   string `json:"version_pattern"`
	BumpCommand      string `json:"bump_command"`
	BuildBumpCommand string `json:"build_bump_command"`
	TagPrefix        string `json:"tag_prefix"`
	ProjectRoot      string `json:"-"`
}

func DefaultConfig() FlowConfig {
	return FlowConfig{
		Remote:         "origin",
		MainBranch:     "main",
		DevelopBranch:  "develop",
		VersionPattern: `(?:__version__|"version")\s*[:=]\s*"([^"]+)"`,
		TagPrefix:      "v",
	}
}

func FindProjectRoot() string {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(filepath.Dir(exe)))
	}
	for _, start := range candidates {
		p := start
		for {
			if _, err := os.Stat(filepath.Join(p, ".git")); err == nil {
				return p
			}
			parent := filepath.Dir(p)
			if parent == p {
				break
			}
			p = parent
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

func LoadConfig(root string) FlowConfig {
	cfg := DefaultConfig()
	cfg.ProjectRoot = root

	data, err := os.ReadFile(filepath.Join(root, ".gitflow.json"))
	if err != nil {
		cfg.VersionFile = detectVersionFile(root)
		return cfg
	}

	var override map[string]string
	if err := json.Unmarshal(data, &override); err != nil {
		cfg.VersionFile = detectVersionFile(root)
		return cfg
	}

	if v, ok := override["remote"]; ok {
		cfg.Remote = v
	}
	if v, ok := override["main_branch"]; ok {
		cfg.MainBranch = v
	}
	if v, ok := override["develop_branch"]; ok {
		cfg.DevelopBranch = v
	}
	if v, ok := override["version_file"]; ok {
		cfg.VersionFile = v
	}
	if v, ok := override["version_pattern"]; ok {
		cfg.VersionPattern = v
	}
	if v, ok := override["bump_command"]; ok {
		cfg.BumpCommand = v
	}
	if v, ok := override["build_bump_command"]; ok {
		cfg.BuildBumpCommand = v
	}
	if v, ok := override["tag_prefix"]; ok {
		cfg.TagPrefix = v
	}

	if cfg.VersionFile == "" {
		cfg.VersionFile = detectVersionFile(root)
	}
	return cfg
}

var versionFileCandidates = []string{
	"core/version.py", "version.py", "src/version.py",
	"package.json", "pyproject.toml", "setup.cfg",
	"Cargo.toml", "VERSION", "internal/version/version.go",
}

func detectVersionFile(root string) string {
	for _, c := range versionFileCandidates {
		if _, err := os.Stat(filepath.Join(root, c)); err == nil {
			return c
		}
	}
	return ""
}
