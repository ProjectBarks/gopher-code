package mcp

import (
	"testing"
)

func TestFilterToolsByServer(t *testing.T) {
	tools := []string{
		"mcp__github__list_repos",
		"mcp__github__create_pr",
		"mcp__slack__send_message",
		"Bash",
		"Read",
	}
	got := FilterToolsByServer(tools, "github")
	if len(got) != 2 {
		t.Fatalf("expected 2 github tools, got %d: %v", len(got), got)
	}
}

func TestCommandBelongsToServer(t *testing.T) {
	// MCP prompt format
	if !CommandBelongsToServer("mcp__github__list_repos", "github") {
		t.Error("mcp__ prefix should match")
	}
	// MCP skill format
	if !CommandBelongsToServer("github:pr-review", "github") {
		t.Error("server:skill format should match")
	}
	// Non-matching
	if CommandBelongsToServer("Bash", "github") {
		t.Error("Bash should not belong to github")
	}
}

func TestMCPInfoFromString_NPX(t *testing.T) {
	info := MCPInfoFromString("npx:@modelcontextprotocol/server-github")
	if info.Transport != "stdio" {
		t.Errorf("transport = %q, want stdio", info.Transport)
	}
	if info.ServerName != "@modelcontextprotocol/server-github" {
		t.Errorf("serverName = %q", info.ServerName)
	}
}

func TestMCPInfoFromString_URL(t *testing.T) {
	info := MCPInfoFromString("https://mcp.example.com/api")
	if info.Transport != "sse" {
		t.Errorf("transport = %q, want sse", info.Transport)
	}
	if info.URL != "https://mcp.example.com/api" {
		t.Errorf("url = %q", info.URL)
	}
	if info.ServerName != "mcp_example_com" {
		t.Errorf("serverName = %q, want mcp_example_com", info.ServerName)
	}
}

func TestMCPInfoFromString_PlainName(t *testing.T) {
	info := MCPInfoFromString("my-server")
	if info.ServerName != "my-server" {
		t.Errorf("serverName = %q, want my-server", info.ServerName)
	}
	if info.Transport != "" {
		t.Errorf("transport should be empty for plain name, got %q", info.Transport)
	}
}

func TestMergeServerConfigs(t *testing.T) {
	enterprise := map[string]ServerConfig{
		"locked": {Command: "enterprise-cmd"},
	}
	user := map[string]ServerConfig{
		"github": {Command: "github-cmd"},
		"locked": {Command: "user-override"}, // overrides enterprise
	}

	merged := MergeServerConfigs(enterprise, user)
	if len(merged) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(merged))
	}
	// "locked" should be overridden by user scope
	if merged["locked"].Command != "user-override" {
		t.Errorf("locked command = %q, want user-override", merged["locked"].Command)
	}
	if merged["locked"].Scope != ScopeUser {
		t.Errorf("locked scope = %q, want user", merged["locked"].Scope)
	}
}
