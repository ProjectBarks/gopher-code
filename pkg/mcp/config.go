package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Source: services/mcp/types.ts, services/mcp/config.ts

// ConfigScope identifies where an MCP server config comes from.
// Source: services/mcp/types.ts:10-21
type ConfigScope string

const (
	ScopeLocal      ConfigScope = "local"
	ScopeUser       ConfigScope = "user"
	ScopeProject    ConfigScope = "project"
	ScopeDynamic    ConfigScope = "dynamic"
	ScopeEnterprise ConfigScope = "enterprise"
	ScopeManaged    ConfigScope = "managed"
)

// Transport identifies the MCP transport type.
// Source: services/mcp/types.ts:23-25
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportSSE   Transport = "sse"
	TransportHTTP  Transport = "http"
	TransportWS    Transport = "ws"
	TransportSDK   Transport = "sdk"
)

// ServerConfig describes how to start/connect to an MCP server.
// This is the union type covering all transport variants.
// Source: services/mcp/types.ts:28-134
type ServerConfig struct {
	// Common
	Type Transport `json:"type,omitempty"` // optional for stdio backward compat

	// Stdio — Source: types.ts:28-35
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// Remote (SSE, HTTP, WS) — Source: types.ts:58-106
	URL           string            `json:"url,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	HeadersHelper string            `json:"headersHelper,omitempty"` // path to script

	// OAuth — Source: types.ts:43-56
	OAuth *OAuthConfig `json:"oauth,omitempty"`
}

// OAuthConfig holds OAuth configuration for remote MCP servers.
// Source: services/mcp/types.ts:43-56
type OAuthConfig struct {
	ClientID              string `json:"clientId,omitempty"`
	CallbackPort          int    `json:"callbackPort,omitempty"`
	AuthServerMetadataURL string `json:"authServerMetadataUrl,omitempty"`
}

// ScopedServerConfig adds scope metadata to a server config.
// Source: services/mcp/types.ts:163-165
type ScopedServerConfig struct {
	ServerConfig
	Scope        ConfigScope `json:"scope"`
	PluginSource string      `json:"pluginSource,omitempty"`
}

// IsStdio returns true if this is a stdio-based server.
func (c *ServerConfig) IsStdio() bool {
	return c.Type == TransportStdio || (c.Type == "" && c.Command != "")
}

// IsRemote returns true if this is a remote (HTTP/SSE/WS) server.
func (c *ServerConfig) IsRemote() bool {
	return c.Type == TransportSSE || c.Type == TransportHTTP || c.Type == TransportWS
}

// MCPConfig holds the configuration for MCP servers from a single source.
type MCPConfig struct {
	Servers map[string]ServerConfig `json:"mcpServers"`
}

// MergedMCPConfig holds scoped configs from all sources.
type MergedMCPConfig struct {
	Servers map[string]ScopedServerConfig
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

// LoadMergedConfig loads MCP server configurations from all sources and merges them.
// Source: services/mcp/config.ts — getClaudeCodeMcpConfigs()
// Precedence (highest→lowest): local > project > user > dynamic
func LoadMergedConfig(cwd string) *MergedMCPConfig {
	merged := &MergedMCPConfig{
		Servers: make(map[string]ScopedServerConfig),
	}

	home, _ := os.UserHomeDir()

	// 1. User scope: ~/.claude/mcp.json
	// Source: config.ts — user servers
	if home != "" {
		userCfg := loadMCPFile(filepath.Join(home, ".claude", "mcp.json"))
		for name, cfg := range userCfg {
			merged.Servers[name] = ScopedServerConfig{
				ServerConfig: cfg,
				Scope:        ScopeUser,
			}
		}
	}

	// 2. User settings.json mcpServers block
	if home != "" {
		settingsCfg := loadSettingsMCPServers(filepath.Join(home, ".claude", "settings.json"))
		for name, cfg := range settingsCfg {
			merged.Servers[name] = ScopedServerConfig{
				ServerConfig: cfg,
				Scope:        ScopeUser,
			}
		}
	}

	// 3. Project scope: .mcp.json in CWD
	// Source: config.ts — project servers
	if cwd != "" {
		projectCfg := loadMCPFile(filepath.Join(cwd, ".mcp.json"))
		for name, cfg := range projectCfg {
			merged.Servers[name] = ScopedServerConfig{
				ServerConfig: cfg,
				Scope:        ScopeProject,
			}
		}
	}

	// 4. Project settings.json mcpServers
	if cwd != "" {
		projectSettings := loadSettingsMCPServers(filepath.Join(cwd, ".claude", "settings.json"))
		for name, cfg := range projectSettings {
			merged.Servers[name] = ScopedServerConfig{
				ServerConfig: cfg,
				Scope:        ScopeProject,
			}
		}
	}

	// 5. Local scope: project config mcpServers
	// Source: config.ts — local servers (highest precedence)
	if cwd != "" {
		localCfg := loadSettingsMCPServers(filepath.Join(cwd, ".claude", "settings.local.json"))
		for name, cfg := range localCfg {
			merged.Servers[name] = ScopedServerConfig{
				ServerConfig: cfg,
				Scope:        ScopeLocal,
			}
		}
	}

	return merged
}

// StdioServers returns only the stdio-based servers from the merged config.
func (m *MergedMCPConfig) StdioServers() map[string]ScopedServerConfig {
	result := make(map[string]ScopedServerConfig)
	for name, cfg := range m.Servers {
		if cfg.IsStdio() {
			result[name] = cfg
		}
	}
	return result
}

// RemoteServers returns only the remote (SSE/HTTP/WS) servers from the merged config.
func (m *MergedMCPConfig) RemoteServers() map[string]ScopedServerConfig {
	result := make(map[string]ScopedServerConfig)
	for name, cfg := range m.Servers {
		if cfg.IsRemote() {
			result[name] = cfg
		}
	}
	return result
}

// ServerNames returns the names of all configured servers.
func (m *MergedMCPConfig) ServerNames() []string {
	names := make([]string, 0, len(m.Servers))
	for name := range m.Servers {
		names = append(names, name)
	}
	return names
}

// loadMCPFile reads servers from a standalone mcp.json file.
func loadMCPFile(path string) map[string]ServerConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return cfg.Servers
}

// loadSettingsMCPServers reads the mcpServers block from a settings.json file.
func loadSettingsMCPServers(path string) map[string]ServerConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	serversData, ok := raw["mcpServers"]
	if !ok {
		return nil
	}
	var servers map[string]ServerConfig
	if err := json.Unmarshal(serversData, &servers); err != nil {
		return nil
	}
	return servers
}

// IsMCPServerDisabled checks if a server is disabled in the project config.
// Source: services/mcp/config.ts:1528-1536
func IsMCPServerDisabled(name string, disabledServers []string) bool {
	for _, disabled := range disabledServers {
		if disabled == name {
			return true
		}
	}
	return false
}
