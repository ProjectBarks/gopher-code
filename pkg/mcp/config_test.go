package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigMissingFile(t *testing.T) {
	// LoadConfig should return empty config when file doesn't exist
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// We can't guarantee the file doesn't exist on every machine,
	// but the function should always return a valid config
	if cfg.Servers == nil {
		t.Error("Servers map should not be nil")
	}
}

func TestParseConfigJSON(t *testing.T) {
	// Create a temp directory to test config parsing
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	configJSON := `{
		"mcpServers": {
			"test-server": {
				"command": "node",
				"args": ["server.js"],
				"env": {"KEY": "val"}
			},
			"another": {
				"command": "python3",
				"args": ["-m", "mcp_server"]
			}
		}
	}`
	configPath := filepath.Join(claudeDir, "mcp.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Test the parsing logic directly
	var cfg MCPConfig
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.Servers == nil {
		cfg.Servers = map[string]ServerConfig{}
	}

	if len(cfg.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(cfg.Servers))
	}

	ts, ok := cfg.Servers["test-server"]
	if !ok {
		t.Fatal("missing test-server")
	}
	if ts.Command != "node" {
		t.Errorf("command = %q, want %q", ts.Command, "node")
	}
	if len(ts.Args) != 1 || ts.Args[0] != "server.js" {
		t.Errorf("args = %v, want [server.js]", ts.Args)
	}
	if ts.Env["KEY"] != "val" {
		t.Errorf("env KEY = %q, want %q", ts.Env["KEY"], "val")
	}

	another, ok := cfg.Servers["another"]
	if !ok {
		t.Fatal("missing another")
	}
	if another.Command != "python3" {
		t.Errorf("command = %q, want %q", another.Command, "python3")
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
