package ide

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// mcpServerConfig matches the .cursor/mcp.json and similar IDE MCP formats.
type mcpServerConfig struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// MCPSupportedIDEs lists IDE IDs that support MCP server configuration.
var MCPSupportedIDEs = map[string]bool{
	IDECursor:     true,
	IDECopilot:    true,
	IDEVSCode:     true,
	IDEClaudeCode: true,
	IDEWindsurf:   true,
	IDECline:      true,
	IDEZed:        true,
	IDENeovim:     true,
	IDEJetBrains:  true,
}

func gitflowBinaryPath() string {
	p, err := exec.LookPath("gitflow")
	if err != nil {
		return "gitflow"
	}
	return p
}

// mcpConfigPath returns the MCP config file path for the given IDE.
func mcpConfigPath(projectRoot, ideID string) string {
	switch ideID {
	case IDECursor:
		return filepath.Join(projectRoot, ".cursor", "mcp.json")
	case IDEVSCode, IDECopilot:
		return filepath.Join(projectRoot, ".vscode", "mcp.json")
	case IDEClaudeCode:
		return filepath.Join(projectRoot, ".claude", "mcp.json")
	case IDEWindsurf:
		return filepath.Join(projectRoot, ".windsurf", "mcp.json")
	case IDECline:
		return filepath.Join(projectRoot, ".cline", "mcp.json")
	case IDEZed:
		return filepath.Join(projectRoot, ".zed", "mcp.json")
	case IDENeovim:
		return filepath.Join(projectRoot, ".nvim", "mcp.json")
	case IDEJetBrains:
		return filepath.Join(projectRoot, ".idea", "mcp.json")
	default:
		return ""
	}
}

// MCPConfigExists checks whether an MCP config with a gitflow entry already exists.
func MCPConfigExists(projectRoot, ideID string) bool {
	path := mcpConfigPath(projectRoot, ideID)
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), `"gitflow"`)
}

// EnsureMCPConfig creates or updates the MCP config file for the given IDE,
// adding a gitflow server entry if not already present. Returns the file path
// if created/updated, or empty string if already present or IDE unsupported.
func EnsureMCPConfig(projectRoot, ideID string) (string, error) {
	if !MCPSupportedIDEs[ideID] {
		return "", nil
	}

	path := mcpConfigPath(projectRoot, ideID)
	if path == "" {
		return "", nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	var cfg mcpServerConfig

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			cfg = mcpServerConfig{}
		}
	}

	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]mcpServerEntry)
	}

	if _, exists := cfg.MCPServers["gitflow"]; exists {
		return "", nil
	}

	cfg.MCPServers["gitflow"] = mcpServerEntry{
		Command: gitflowBinaryPath(),
		Args:    []string{"serve"},
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0644); err != nil {
		return "", err
	}
	return path, nil
}
