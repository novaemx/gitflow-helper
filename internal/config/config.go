package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	IntegrationModeLocalMerge  = "local-merge"
	IntegrationModePullRequest = "pull-request"
)

type FlowConfig struct {
	Remote           string `json:"remote"`
	MainBranch       string `json:"main_branch"`
	DevelopBranch    string `json:"develop_branch"`
	IntegrationMode  string `json:"integration_mode"`
	VersionFile      string `json:"version_file"`
	VersionPattern   string `json:"version_pattern"`
	BumpCommand      string `json:"bump_command"`
	BuildBumpCommand string `json:"build_bump_command"`
	TagPrefix        string `json:"tag_prefix"`
	ProjectRoot      string `json:"-"`
	ModeConfigured   bool   `json:"-"`
}

func DefaultConfig() FlowConfig {
	return FlowConfig{
		Remote:          "origin",
		MainBranch:      "main",
		DevelopBranch:   "develop",
		IntegrationMode: IntegrationModeLocalMerge,
		VersionPattern:  `(?:__version__|"version")\s*[:=]\s*"([^"]+)"`,
		TagPrefix:       "v",
	}
}

func NormalizeIntegrationMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "", "local", "local-merge", "merge-local", "local_merge":
		return IntegrationModeLocalMerge
	case "pr", "pull-request", "pull_request", "pullrequest", "remote-merge", "merge-remote":
		return IntegrationModePullRequest
	default:
		return ""
	}
}

func IntegrationModeDisplay(mode string) string {
	normalized := NormalizeIntegrationMode(mode)
	if normalized == "" {
		normalized = IntegrationModeLocalMerge
	}
	if normalized == IntegrationModePullRequest {
		return "Pull Requests"
	}
	return "Local Merge"
}

func readString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
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

	var override map[string]any
	if err := json.Unmarshal(data, &override); err != nil {
		cfg.VersionFile = detectVersionFile(root)
		return cfg
	}

	if v, ok := readString(override, "remote"); ok {
		cfg.Remote = v
	}
	if v, ok := readString(override, "main_branch"); ok {
		cfg.MainBranch = v
	}
	if v, ok := readString(override, "develop_branch"); ok {
		cfg.DevelopBranch = v
	}
	if v, ok := readString(override, "integration_mode"); ok {
		normalized := NormalizeIntegrationMode(v)
		if normalized != "" {
			cfg.IntegrationMode = normalized
			cfg.ModeConfigured = true
		}
	}
	if v, ok := readString(override, "version_file"); ok {
		cfg.VersionFile = v
	}
	if v, ok := readString(override, "version_pattern"); ok {
		cfg.VersionPattern = v
	}
	if v, ok := readString(override, "bump_command"); ok {
		cfg.BumpCommand = v
	}
	if v, ok := readString(override, "build_bump_command"); ok {
		cfg.BuildBumpCommand = v
	}
	if v, ok := readString(override, "tag_prefix"); ok {
		cfg.TagPrefix = v
	}

	if cfg.VersionFile == "" {
		cfg.VersionFile = detectVersionFile(root)
	}
	return cfg
}

func SetIntegrationMode(root, mode string) error {
	normalized := NormalizeIntegrationMode(mode)
	if normalized == "" {
		normalized = IntegrationModeLocalMerge
	}

	path := filepath.Join(root, ".gitflow.json")
	override := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &override)
	}
	override["integration_mode"] = normalized

	data, err := json.MarshalIndent(override, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
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
