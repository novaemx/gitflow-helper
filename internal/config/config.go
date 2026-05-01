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
	projectConfigDirName       = ".gitflow"
	projectConfigFileName      = "config.json"
	legacyConfigFileName       = ".gitflow.json"
	legacyAIConfigFileName     = "ai-integration.json"
)

type AIIntegrationChoice struct {
	Enabled bool   `json:"enabled"`
	Version string `json:"version,omitempty"`
}

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
	TestCommand      string `json:"test_command"`
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

func ProjectConfigPath(root string) string {
	return filepath.Join(root, projectConfigDirName, projectConfigFileName)
}

func legacyConfigPath(root string) string {
	return filepath.Join(root, legacyConfigFileName)
}

func legacyAIIntegrationPath(root string) string {
	return filepath.Join(root, projectConfigDirName, legacyAIConfigFileName)
}

func readJSONMap(path string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, true, err
	}
	if decoded == nil {
		decoded = map[string]any{}
	}
	return decoded, true, nil
}

func loadProjectConfigMap(root string) (map[string]any, bool, error) {
	if decoded, ok, err := readJSONMap(ProjectConfigPath(root)); err != nil || ok {
		return decoded, ok, err
	}
	return readJSONMap(legacyConfigPath(root))
}

func writeProjectConfigMap(root string, decoded map[string]any) error {
	path := ProjectConfigPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	_ = os.Remove(legacyConfigPath(root))
	_ = os.Remove(legacyAIIntegrationPath(root))
	return nil
}

func applyFlowOverrides(cfg *FlowConfig, override map[string]any) {
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
	if v, ok := readString(override, "test_command"); ok {
		cfg.TestCommand = v
	}
}

func normalizeAIChoice(value any) (*AIIntegrationChoice, bool) {
	choiceMap, ok := value.(map[string]any)
	if !ok || choiceMap == nil {
		return nil, false
	}
	choice := &AIIntegrationChoice{}
	if enabled, ok := choiceMap["enabled"].(bool); ok {
		choice.Enabled = enabled
	}
	if version, ok := readString(choiceMap, "version"); ok {
		choice.Version = version
	}
	return choice, true
}

func loadProjectAIIntegrationChoice(decoded map[string]any) (*AIIntegrationChoice, bool) {
	if decoded == nil {
		return nil, false
	}
	if choice, ok := normalizeAIChoice(decoded["ai_integration"]); ok {
		return choice, true
	}
	// Older project configs could store the consent payload directly at the
	// root of .gitflow/config.json before the unified ai_integration block.
	if choice, ok := normalizeAIChoice(decoded); ok {
		_, hasEnabled := decoded["enabled"]
		_, hasVersion := decoded["version"]
		if hasEnabled || hasVersion {
			return choice, true
		}
	}
	return nil, false
}

func LoadAIIntegrationChoice(root string) (AIIntegrationChoice, bool, error) {
	if decoded, _, err := loadProjectConfigMap(root); err != nil {
		return AIIntegrationChoice{}, false, err
	} else if choice, ok := loadProjectAIIntegrationChoice(decoded); ok {
			return *choice, true, nil
	}

	data, err := os.ReadFile(legacyAIIntegrationPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return AIIntegrationChoice{}, false, nil
		}
		return AIIntegrationChoice{}, false, err
	}

	var choice AIIntegrationChoice
	if err := json.Unmarshal(data, &choice); err != nil {
		return AIIntegrationChoice{}, false, err
	}
	return choice, true, nil
}

func SaveAIIntegrationChoice(root string, choice AIIntegrationChoice) error {
	decoded, _, err := loadProjectConfigMap(root)
	if err != nil {
		return err
	}
	if decoded == nil {
		decoded = map[string]any{}
	}
	aiSection := map[string]any{
		"enabled": choice.Enabled,
	}
	if strings.TrimSpace(choice.Version) != "" {
		aiSection["version"] = choice.Version
	}
	decoded["ai_integration"] = aiSection
	return writeProjectConfigMap(root, decoded)
}

// findProjectRootFrom walks up from start looking for a .git directory.
// Returns the directory containing .git, or "" if not found.
func findProjectRootFrom(start string) string {
	p := start
	for {
		if _, err := os.Stat(filepath.Join(p, ".git")); err == nil {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return ""
		}
		p = parent
	}
}

// FindProjectRoot returns the nearest ancestor directory (inclusive of CWD)
// that contains a .git directory. Falls back to the current working directory
// when no git repository is found. The binary's own location is intentionally
// NOT considered — it would produce wrong results when the tool is installed
// inside a foreign git repository (e.g. Homebrew at /opt/homebrew).
func FindProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if root := findProjectRootFrom(cwd); root != "" {
		return root
	}
	return cwd
}

func LoadConfig(root string) FlowConfig {
	cfg := DefaultConfig()
	cfg.ProjectRoot = root

	override, _, err := loadProjectConfigMap(root)
	if err != nil {
		cfg.VersionFile = detectVersionFile(root)
		return cfg
	}
	if override == nil {
		cfg.VersionFile = detectVersionFile(root)
		return cfg
	}

	applyFlowOverrides(&cfg, override)

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

	override, _, err := loadProjectConfigMap(root)
	if err != nil {
		return err
	}
	if override == nil {
		override = map[string]any{}
	}
	override["integration_mode"] = normalized
	return writeProjectConfigMap(root, override)
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
