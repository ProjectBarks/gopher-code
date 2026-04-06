package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Source: services/mcp/utils.ts

func TestScopeLabel(t *testing.T) {
	// Source: services/mcp/utils.ts:282-299
	tests := []struct {
		scope ConfigScope
		want  string
	}{
		{ScopeLocal, "Local config (private to you in this project)"},
		{ScopeProject, "Project config (shared via .mcp.json)"},
		{ScopeUser, "User config (available in all your projects)"},
		{ScopeDynamic, "Dynamic config (from command line)"},
		{ScopeEnterprise, "Enterprise config (managed by your organization)"},
		{ScopeClaudeAI, "claude.ai config"},
		{ConfigScope("unknown"), "unknown"},
	}
	for _, tt := range tests {
		if got := ScopeLabel(tt.scope); got != tt.want {
			t.Errorf("ScopeLabel(%q) = %q, want %q", tt.scope, got, tt.want)
		}
	}
}

func TestEnsureConfigScope(t *testing.T) {
	// Source: services/mcp/utils.ts:301-311
	t.Run("empty defaults to local", func(t *testing.T) {
		scope, err := EnsureConfigScope("")
		if err != nil {
			t.Fatal(err)
		}
		if scope != ScopeLocal {
			t.Errorf("scope = %q, want local", scope)
		}
	})

	t.Run("valid scopes", func(t *testing.T) {
		for _, s := range []string{"local", "user", "project", "dynamic", "enterprise"} {
			scope, err := EnsureConfigScope(s)
			if err != nil {
				t.Errorf("EnsureConfigScope(%q) error: %v", s, err)
			}
			if string(scope) != s {
				t.Errorf("scope = %q, want %q", scope, s)
			}
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		_, err := EnsureConfigScope("bogus")
		if err == nil {
			t.Error("expected error for invalid scope")
		}
	})
}

func TestEnsureTransport(t *testing.T) {
	// Source: services/mcp/utils.ts:313-323
	t.Run("empty defaults to stdio", func(t *testing.T) {
		tp, err := EnsureTransport("")
		if err != nil {
			t.Fatal(err)
		}
		if tp != TransportStdio {
			t.Errorf("tp = %q, want stdio", tp)
		}
	})

	t.Run("valid types", func(t *testing.T) {
		for _, s := range []string{"stdio", "sse", "http"} {
			tp, err := EnsureTransport(s)
			if err != nil {
				t.Errorf("EnsureTransport(%q) error: %v", s, err)
			}
			if string(tp) != s {
				t.Errorf("tp = %q, want %q", tp, s)
			}
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		_, err := EnsureTransport("grpc")
		if err == nil {
			t.Error("expected error for invalid transport")
		}
	})
}

func TestParseHeaders(t *testing.T) {
	// Source: services/mcp/utils.ts:325-349
	t.Run("valid headers", func(t *testing.T) {
		headers, err := ParseHeaders([]string{
			"Authorization: Bearer token123",
			"Content-Type: application/json",
		})
		if err != nil {
			t.Fatal(err)
		}
		if headers["Authorization"] != "Bearer token123" {
			t.Errorf("Authorization = %q", headers["Authorization"])
		}
		if headers["Content-Type"] != "application/json" {
			t.Errorf("Content-Type = %q", headers["Content-Type"])
		}
	})

	t.Run("no colon", func(t *testing.T) {
		_, err := ParseHeaders([]string{"no-colon"})
		if err == nil {
			t.Error("expected error for missing colon")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := ParseHeaders([]string{": value"})
		if err == nil {
			t.Error("expected error for empty header name")
		}
	})

	t.Run("value with colons", func(t *testing.T) {
		headers, err := ParseHeaders([]string{"X-Custom: a:b:c"})
		if err != nil {
			t.Fatal(err)
		}
		if headers["X-Custom"] != "a:b:c" {
			t.Errorf("X-Custom = %q, want %q", headers["X-Custom"], "a:b:c")
		}
	})
}

func TestHashMCPConfig_Stable(t *testing.T) {
	// Source: services/mcp/utils.ts:157-169
	cfg := ScopedServerConfig{
		ServerConfig: ServerConfig{
			Command: "node",
			Args:    []string{"server.js"},
			Env:     map[string]string{"KEY": "val"},
		},
		Scope: ScopeUser,
	}

	hash1 := HashMCPConfig(cfg)
	hash2 := HashMCPConfig(cfg)
	if hash1 != hash2 {
		t.Errorf("hash not stable: %q != %q", hash1, hash2)
	}
	if len(hash1) != 16 {
		t.Errorf("hash length = %d, want 16", len(hash1))
	}
}

func TestHashMCPConfig_IgnoresScope(t *testing.T) {
	// Source: utils.ts:158 — scope excluded from hash
	cfg1 := ScopedServerConfig{
		ServerConfig: ServerConfig{Command: "node", Args: []string{"a"}},
		Scope:        ScopeUser,
	}
	cfg2 := ScopedServerConfig{
		ServerConfig: ServerConfig{Command: "node", Args: []string{"a"}},
		Scope:        ScopeProject,
	}
	if HashMCPConfig(cfg1) != HashMCPConfig(cfg2) {
		t.Error("different scopes should produce same hash")
	}
}

func TestHashMCPConfig_DifferentConfigs(t *testing.T) {
	cfg1 := ScopedServerConfig{
		ServerConfig: ServerConfig{Command: "node", Args: []string{"a"}},
	}
	cfg2 := ScopedServerConfig{
		ServerConfig: ServerConfig{Command: "node", Args: []string{"b"}},
	}
	if HashMCPConfig(cfg1) == HashMCPConfig(cfg2) {
		t.Error("different configs should produce different hashes")
	}
}

func TestGetLoggingSafeMCPBaseURL(t *testing.T) {
	// Source: services/mcp/utils.ts:561-575
	tests := []struct {
		name string
		cfg  ServerConfig
		want string
	}{
		{
			name: "strips query",
			cfg:  ServerConfig{URL: "https://mcp.example.com/api?token=secret"},
			want: "https://mcp.example.com/api",
		},
		{
			name: "strips trailing slash",
			cfg:  ServerConfig{URL: "https://mcp.example.com/"},
			want: "https://mcp.example.com",
		},
		{
			name: "strips both",
			cfg:  ServerConfig{URL: "https://mcp.example.com/?token=x"},
			want: "https://mcp.example.com",
		},
		{
			name: "no URL (stdio)",
			cfg:  ServerConfig{Command: "node"},
			want: "",
		},
		{
			name: "already clean",
			cfg:  ServerConfig{URL: "https://mcp.example.com/api"},
			want: "https://mcp.example.com/api",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetLoggingSafeMCPBaseURL(tt.cfg); got != tt.want {
				t.Errorf("GetLoggingSafeMCPBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsToolFromMCPServer(t *testing.T) {
	if !IsToolFromMCPServer("mcp__myserver__tool", "myserver") {
		t.Error("should match")
	}
	if IsToolFromMCPServer("mcp__myserver__tool", "other") {
		t.Error("should not match different server")
	}
	if IsToolFromMCPServer("BashTool", "myserver") {
		t.Error("non-MCP tool should not match")
	}
}

func TestLoadConfigFromTempSettingsFile(t *testing.T) {
	// Integration test: write settings.json with mcpServers to temp dir and load
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	settings := map[string]interface{}{
		"model": "opus",
		"mcpServers": map[string]interface{}{
			"stdio-server": map[string]interface{}{
				"command": "node",
				"args":    []string{"server.js", "--port", "3000"},
				"env":     map[string]string{"API_KEY": "test123"},
			},
			"sse-server": map[string]interface{}{
				"type": "sse",
				"url":  "https://mcp.example.com/sse",
			},
			"http-server": map[string]interface{}{
				"type": "http",
				"url":  "https://api.example.com/mcp",
				"headers": map[string]string{
					"Authorization": "Bearer token",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, data, 0644)

	servers := loadSettingsMCPServers(settingsPath)
	if len(servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(servers))
	}

	// Check stdio
	stdio := servers["stdio-server"]
	if stdio.Command != "node" {
		t.Errorf("stdio command = %q", stdio.Command)
	}
	if !stdio.IsStdio() {
		t.Error("should be stdio")
	}

	// Check SSE
	sse := servers["sse-server"]
	if sse.Type != TransportSSE {
		t.Errorf("sse type = %q", sse.Type)
	}
	if sse.URL != "https://mcp.example.com/sse" {
		t.Errorf("sse url = %q", sse.URL)
	}

	// Check HTTP
	http := servers["http-server"]
	if http.Type != TransportHTTP {
		t.Errorf("http type = %q", http.Type)
	}
	if http.Headers["Authorization"] != "Bearer token" {
		t.Errorf("http auth header = %q", http.Headers["Authorization"])
	}
}

func TestLoadMergedConfig_ThreeScopes(t *testing.T) {
	// Integration test: user + project + local scopes with precedence
	cwd := t.TempDir()

	// Project .mcp.json
	os.WriteFile(filepath.Join(cwd, ".mcp.json"), []byte(`{
		"mcpServers": {
			"project-server": {"command": "proj-cmd"},
			"shared": {"command": "proj-shared"}
		}
	}`), 0644)

	// Project settings.json
	claudeDir := filepath.Join(cwd, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{
		"mcpServers": {
			"project-settings": {"type": "sse", "url": "https://proj.example.com"}
		}
	}`), 0644)

	// Local settings.local.json (highest precedence)
	os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(`{
		"mcpServers": {
			"shared": {"command": "local-override"},
			"local-only": {"command": "local-cmd"}
		}
	}`), 0644)

	merged := LoadMergedConfig(cwd)

	// "shared" should be local scope (overrides project)
	shared := merged.Servers["shared"]
	if shared.Command != "local-override" {
		t.Errorf("shared.Command = %q, want local-override", shared.Command)
	}
	if shared.Scope != ScopeLocal {
		t.Errorf("shared.Scope = %q, want local", shared.Scope)
	}

	// project-server from .mcp.json
	if _, ok := merged.Servers["project-server"]; !ok {
		t.Error("missing project-server")
	}

	// project-settings from settings.json
	if _, ok := merged.Servers["project-settings"]; !ok {
		t.Error("missing project-settings")
	}

	// local-only from settings.local.json
	localOnly := merged.Servers["local-only"]
	if localOnly.Scope != ScopeLocal {
		t.Errorf("local-only.Scope = %q, want local", localOnly.Scope)
	}
}
