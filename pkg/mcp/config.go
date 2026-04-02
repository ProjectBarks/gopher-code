package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// MCPConfig holds the configuration for MCP servers.
type MCPConfig struct {
	Servers map[string]ServerConfig `json:"mcpServers"`
}

// LoadConfig reads the MCP configuration from ~/.claude/mcp.json.
// If the file doesn't exist, it returns an empty config (not an error).
func LoadConfig() (*MCPConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &MCPConfig{Servers: map[string]ServerConfig{}}, nil
	}
	path := filepath.Join(home, ".claude", "mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		// File not found is not an error - just no MCP servers configured
		return &MCPConfig{Servers: map[string]ServerConfig{}}, nil
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Servers == nil {
		cfg.Servers = map[string]ServerConfig{}
	}
	return &cfg, nil
}
