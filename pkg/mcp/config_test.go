package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Source: services/mcp/types.ts, services/mcp/config.ts

func TestConfigScopeConstants(t *testing.T) {
	// Source: services/mcp/types.ts:10-21
	if ScopeLocal != "local" { t.Error("wrong") }
	if ScopeUser != "user" { t.Error("wrong") }
	if ScopeProject != "project" { t.Error("wrong") }
	if ScopeDynamic != "dynamic" { t.Error("wrong") }
	if ScopeEnterprise != "enterprise" { t.Error("wrong") }
	if ScopeManaged != "managed" { t.Error("wrong") }
}

func TestTransportConstants(t *testing.T) {
	// Source: services/mcp/types.ts:23-25
	if TransportStdio != "stdio" { t.Error("wrong") }
	if TransportSSE != "sse" { t.Error("wrong") }
	if TransportHTTP != "http" { t.Error("wrong") }
	if TransportWS != "ws" { t.Error("wrong") }
	if TransportSDK != "sdk" { t.Error("wrong") }
}

func TestServerConfig_IsStdio(t *testing.T) {
	// Source: types.ts:28-35 — stdio has command field
	t.Run("explicit_stdio", func(t *testing.T) {
		cfg := ServerConfig{Type: TransportStdio, Command: "node"}
		if !cfg.IsStdio() {
			t.Error("should be stdio")
		}
	})

	t.Run("implicit_stdio", func(t *testing.T) {
		// Source: types.ts:30 — type is optional for backwards compat
		cfg := ServerConfig{Command: "python3"}
		if !cfg.IsStdio() {
			t.Error("command without type should be stdio")
		}
	})

	t.Run("not_stdio", func(t *testing.T) {
		cfg := ServerConfig{Type: TransportSSE, URL: "https://example.com"}
		if cfg.IsStdio() {
			t.Error("SSE should not be stdio")
		}
	})
}

func TestServerConfig_IsRemote(t *testing.T) {
	// Source: types.ts:58-106 — remote types have URL
	for _, tp := range []Transport{TransportSSE, TransportHTTP, TransportWS} {
		cfg := ServerConfig{Type: tp, URL: "https://example.com"}
		if !cfg.IsRemote() {
			t.Errorf("%s should be remote", tp)
		}
	}
	cfg := ServerConfig{Type: TransportStdio, Command: "node"}
	if cfg.IsRemote() {
		t.Error("stdio should not be remote")
	}
}

func TestServerConfigJSON_Stdio(t *testing.T) {
	// Source: types.ts:28-35
	data := []byte(`{"command":"node","args":["server.js"],"env":{"KEY":"val"}}`)
	var cfg ServerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Command != "node" {
		t.Errorf("command = %q", cfg.Command)
	}
	if len(cfg.Args) != 1 || cfg.Args[0] != "server.js" {
		t.Errorf("args = %v", cfg.Args)
	}
	if cfg.Env["KEY"] != "val" {
		t.Error("env missing")
	}
	if !cfg.IsStdio() {
		t.Error("should be stdio")
	}
}

func TestServerConfigJSON_SSE(t *testing.T) {
	// Source: types.ts:58-65
	data := []byte(`{"type":"sse","url":"https://mcp.example.com","headers":{"Authorization":"Bearer token"}}`)
	var cfg ServerConfig
	json.Unmarshal(data, &cfg)
	if cfg.Type != TransportSSE {
		t.Errorf("type = %q", cfg.Type)
	}
	if cfg.URL != "https://mcp.example.com" {
		t.Errorf("url = %q", cfg.URL)
	}
	if cfg.Headers["Authorization"] != "Bearer token" {
		t.Error("headers missing")
	}
}

func TestServerConfigJSON_HTTP(t *testing.T) {
	// Source: types.ts:89-96
	data := []byte(`{"type":"http","url":"https://api.example.com/mcp","oauth":{"clientId":"abc","callbackPort":3000}}`)
	var cfg ServerConfig
	json.Unmarshal(data, &cfg)
	if cfg.Type != TransportHTTP {
		t.Errorf("type = %q", cfg.Type)
	}
	if cfg.OAuth == nil {
		t.Fatal("oauth should be present")
	}
	if cfg.OAuth.ClientID != "abc" {
		t.Errorf("clientId = %q", cfg.OAuth.ClientID)
	}
	if cfg.OAuth.CallbackPort != 3000 {
		t.Errorf("callbackPort = %d", cfg.OAuth.CallbackPort)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Servers == nil {
		t.Error("Servers map should not be nil")
	}
}

func TestLoadMergedConfig_UserScope(t *testing.T) {
	// Source: config.ts — user scope from mcp.json
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	// Write mcp.json
	os.WriteFile(filepath.Join(claudeDir, "mcp.json"), []byte(`{
		"mcpServers": {
			"user-server": {"command": "node", "args": ["server.js"]}
		}
	}`), 0644)

	// Use tmpDir as fake home
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// We can't easily override UserHomeDir, so test loadMCPFile directly
	servers := loadMCPFile(filepath.Join(claudeDir, "mcp.json"))
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers["user-server"].Command != "node" {
		t.Errorf("command = %q", servers["user-server"].Command)
	}
}

func TestLoadMergedConfig_ProjectScope(t *testing.T) {
	// Source: config.ts — project scope from .mcp.json
	cwd := t.TempDir()

	os.WriteFile(filepath.Join(cwd, ".mcp.json"), []byte(`{
		"mcpServers": {
			"project-server": {"command": "python3", "args": ["-m", "server"]}
		}
	}`), 0644)

	servers := loadMCPFile(filepath.Join(cwd, ".mcp.json"))
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers["project-server"].Command != "python3" {
		t.Errorf("command = %q", servers["project-server"].Command)
	}
}

func TestLoadMergedConfig_SettingsMCPServers(t *testing.T) {
	// Source: config.ts — mcpServers block in settings.json
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	os.WriteFile(settingsPath, []byte(`{
		"model": "opus",
		"mcpServers": {
			"settings-server": {"type": "sse", "url": "https://mcp.example.com"}
		}
	}`), 0644)

	servers := loadSettingsMCPServers(settingsPath)
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	cfg := servers["settings-server"]
	if cfg.Type != TransportSSE {
		t.Errorf("type = %q", cfg.Type)
	}
	if cfg.URL != "https://mcp.example.com" {
		t.Errorf("url = %q", cfg.URL)
	}
}

func TestLoadMergedConfig_Precedence(t *testing.T) {
	// Source: config.ts — local > project > user (later overrides earlier)
	cwd := t.TempDir()

	// Project .mcp.json
	os.WriteFile(filepath.Join(cwd, ".mcp.json"), []byte(`{
		"mcpServers": {
			"shared": {"command": "project-cmd"},
			"project-only": {"command": "proj"}
		}
	}`), 0644)

	// Local settings.local.json
	claudeDir := filepath.Join(cwd, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(`{
		"mcpServers": {
			"shared": {"command": "local-cmd"}
		}
	}`), 0644)

	merged := LoadMergedConfig(cwd)

	// "shared" should be local scope (overrides project)
	shared, ok := merged.Servers["shared"]
	if !ok {
		t.Fatal("missing 'shared'")
	}
	if shared.Command != "local-cmd" {
		t.Errorf("shared.Command = %q, want 'local-cmd' (local overrides project)", shared.Command)
	}
	if shared.Scope != ScopeLocal {
		t.Errorf("shared.Scope = %q, want local", shared.Scope)
	}

	// "project-only" should still be present
	_, ok = merged.Servers["project-only"]
	if !ok {
		t.Error("missing 'project-only'")
	}
}

func TestMergedMCPConfig_StdioServers(t *testing.T) {
	merged := &MergedMCPConfig{
		Servers: map[string]ScopedServerConfig{
			"stdio-1": {ServerConfig: ServerConfig{Command: "node"}, Scope: ScopeUser},
			"sse-1":   {ServerConfig: ServerConfig{Type: TransportSSE, URL: "https://x"}, Scope: ScopeUser},
			"stdio-2": {ServerConfig: ServerConfig{Type: TransportStdio, Command: "python3"}, Scope: ScopeProject},
		},
	}

	stdio := merged.StdioServers()
	if len(stdio) != 2 {
		t.Errorf("expected 2 stdio servers, got %d", len(stdio))
	}
}

func TestMergedMCPConfig_RemoteServers(t *testing.T) {
	merged := &MergedMCPConfig{
		Servers: map[string]ScopedServerConfig{
			"stdio": {ServerConfig: ServerConfig{Command: "node"}, Scope: ScopeUser},
			"sse":   {ServerConfig: ServerConfig{Type: TransportSSE, URL: "https://x"}, Scope: ScopeUser},
			"http":  {ServerConfig: ServerConfig{Type: TransportHTTP, URL: "https://y"}, Scope: ScopeProject},
		},
	}

	remote := merged.RemoteServers()
	if len(remote) != 2 {
		t.Errorf("expected 2 remote servers, got %d", len(remote))
	}
}

func TestMergedMCPConfig_ServerNames(t *testing.T) {
	merged := &MergedMCPConfig{
		Servers: map[string]ScopedServerConfig{
			"a": {Scope: ScopeUser},
			"b": {Scope: ScopeProject},
		},
	}

	names := merged.ServerNames()
	sort.Strings(names)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("names = %v", names)
	}
}

func TestIsMCPServerDisabled(t *testing.T) {
	// Source: services/mcp/config.ts:1528-1536
	disabled := []string{"server-a", "server-b"}

	if !IsMCPServerDisabled("server-a", disabled) {
		t.Error("server-a should be disabled")
	}
	if IsMCPServerDisabled("server-c", disabled) {
		t.Error("server-c should not be disabled")
	}
	if IsMCPServerDisabled("anything", nil) {
		t.Error("nil list should not disable anything")
	}
}

func TestParseConfigEmptyServers(t *testing.T) {
	data := []byte(`{"mcpServers":{}}`)
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(cfg.Servers))
	}
}

func TestParseConfigInvalidJSON(t *testing.T) {
	data := []byte(`{invalid}`)
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadSettingsMCPServers_NoFile(t *testing.T) {
	servers := loadSettingsMCPServers("/nonexistent/settings.json")
	if servers != nil {
		t.Error("should return nil for missing file")
	}
}

func TestLoadSettingsMCPServers_NoMCPBlock(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")
	os.WriteFile(path, []byte(`{"model":"opus"}`), 0644)

	servers := loadSettingsMCPServers(path)
	if servers != nil {
		t.Error("should return nil when no mcpServers block")
	}
}

func TestOAuthConfig_JSON(t *testing.T) {
	// Source: types.ts:43-56
	data := []byte(`{"clientId":"my-client","callbackPort":8080,"authServerMetadataUrl":"https://auth.example.com/.well-known/oauth-authorization-server"}`)
	var oauth OAuthConfig
	json.Unmarshal(data, &oauth)
	if oauth.ClientID != "my-client" {
		t.Errorf("clientId = %q", oauth.ClientID)
	}
	if oauth.CallbackPort != 8080 {
		t.Errorf("callbackPort = %d", oauth.CallbackPort)
	}
	if oauth.AuthServerMetadataURL != "https://auth.example.com/.well-known/oauth-authorization-server" {
		t.Errorf("authServerMetadataUrl = %q", oauth.AuthServerMetadataURL)
	}
}
