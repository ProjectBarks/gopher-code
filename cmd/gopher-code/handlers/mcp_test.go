package handlers

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/mcp"
)

// newTestHandler creates an MCPHandler rooted at a temp dir with captured output.
func newTestHandler(t *testing.T) (*MCPHandler, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	// Ensure .claude subdir exists for local/project config writes
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	var stdout, stderr bytes.Buffer
	h := &MCPHandler{
		CWD:    dir,
		Stdout: &stdout,
		Stderr: &stderr,
	}
	return h, &stdout, &stderr
}

// writeMCPJSON writes an mcp.json file into the handler's CWD.
func writeMCPJSON(t *testing.T, dir string, servers map[string]mcp.ServerConfig) {
	t.Helper()
	cfg := mcp.MCPConfig{Servers: servers}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

// writeSettingsJSON writes a settings.json file with mcpServers block.
func writeSettingsJSON(t *testing.T, path string, servers map[string]mcp.ServerConfig) {
	t.Helper()
	raw := map[string]interface{}{
		"mcpServers": servers,
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestList_NoServers(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	got := stdout.String()
	want := "No MCP servers configured. Use `claude mcp add` to add a server."
	if !strings.Contains(got, want) {
		t.Errorf("List() output = %q, want it to contain %q", got, want)
	}
}

func TestList_WithServers(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	writeMCPJSON(t, h.CWD, map[string]mcp.ServerConfig{
		"my-server": {
			Type:    mcp.TransportStdio,
			Command: "node",
			Args:    []string{"server.js"},
		},
	})

	err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Checking MCP server health...") {
		t.Errorf("expected health check message, got: %q", got)
	}
	if !strings.Contains(got, "my-server: node server.js") {
		t.Errorf("expected server line, got: %q", got)
	}
}

func TestList_SSEServer(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	writeMCPJSON(t, h.CWD, map[string]mcp.ServerConfig{
		"remote": {
			Type: mcp.TransportSSE,
			URL:  "https://example.com/mcp",
		},
	})

	err := h.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "remote: https://example.com/mcp (SSE)") {
		t.Errorf("expected SSE server line, got: %q", got)
	}
}

func TestAddJSON_GetRoundTrip(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	jsonStr := `{"type":"stdio","command":"npx","args":["my-mcp-server"],"env":{"API_KEY":"test123"}}`

	// Add server
	err := h.AddJSON("test-srv", jsonStr, "local")
	if err != nil {
		t.Fatalf("AddJSON() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Added stdio MCP server test-srv to local config") {
		t.Errorf("AddJSON() output = %q, want add confirmation", got)
	}

	// Now get it back
	stdout.Reset()
	err = h.Get("test-srv")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	got = stdout.String()
	if !strings.Contains(got, "test-srv:") {
		t.Errorf("Get() missing server name header, got: %q", got)
	}
	if !strings.Contains(got, "Type: stdio") {
		t.Errorf("Get() missing type, got: %q", got)
	}
	if !strings.Contains(got, "Command: npx") {
		t.Errorf("Get() missing command, got: %q", got)
	}
	if !strings.Contains(got, "Args: my-mcp-server") {
		t.Errorf("Get() missing args, got: %q", got)
	}
	if !strings.Contains(got, "API_KEY=test123") {
		t.Errorf("Get() missing env, got: %q", got)
	}
	if !strings.Contains(got, "Scope: Local config") {
		t.Errorf("Get() missing scope label, got: %q", got)
	}
	if !strings.Contains(got, `claude mcp remove "test-srv" -s local`) {
		t.Errorf("Get() missing remove hint, got: %q", got)
	}
}

func TestAddJSON_HTTPServer(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	jsonStr := `{"type":"http","url":"https://api.example.com/mcp","headers":{"Authorization":"Bearer tok"}}`

	err := h.AddJSON("http-srv", jsonStr, "local")
	if err != nil {
		t.Fatalf("AddJSON() error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Added http MCP server http-srv to local config") {
		t.Errorf("unexpected output: %q", stdout.String())
	}

	// Get it
	stdout.Reset()
	err = h.Get("http-srv")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Type: http") {
		t.Errorf("Get() missing type, got: %q", got)
	}
	if !strings.Contains(got, "URL: https://api.example.com/mcp") {
		t.Errorf("Get() missing url, got: %q", got)
	}
	if !strings.Contains(got, "Authorization: Bearer tok") {
		t.Errorf("Get() missing headers, got: %q", got)
	}
}

func TestAddJSON_InvalidName(t *testing.T) {
	h, _, _ := newTestHandler(t)

	err := h.AddJSON("bad name!", `{"command":"foo"}`, "local")
	if err == nil {
		t.Fatal("AddJSON() expected error for invalid name")
	}
	if !strings.Contains(err.Error(), "invalid name") {
		t.Errorf("error = %q, want 'invalid name'", err.Error())
	}
}

func TestAddJSON_InvalidJSON(t *testing.T) {
	h, _, _ := newTestHandler(t)

	err := h.AddJSON("good-name", "not json", "local")
	if err == nil {
		t.Fatal("AddJSON() expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("error = %q, want 'invalid JSON'", err.Error())
	}
}

func TestAddJSON_InvalidScope(t *testing.T) {
	h, _, _ := newTestHandler(t)

	err := h.AddJSON("srv", `{"command":"foo"}`, "bogus")
	if err == nil {
		t.Fatal("AddJSON() expected error for invalid scope")
	}
	if !strings.Contains(err.Error(), "invalid scope") {
		t.Errorf("error = %q, want 'invalid scope'", err.Error())
	}
}

func TestAddJSON_DefaultScopeIsLocal(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	jsonStr := `{"command":"echo","args":["hello"]}`
	err := h.AddJSON("default-scope", jsonStr, "")
	if err != nil {
		t.Fatalf("AddJSON() error: %v", err)
	}

	if !strings.Contains(stdout.String(), "local config") {
		t.Errorf("expected default scope 'local', got: %q", stdout.String())
	}
}

func TestRemove_SingleScope(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	// Add first
	jsonStr := `{"command":"node","args":["srv.js"]}`
	if err := h.AddJSON("rm-me", jsonStr, "local"); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()

	// Remove without specifying scope
	err := h.Remove("rm-me", "")
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, `Removed MCP server "rm-me" from local config`) {
		t.Errorf("Remove() output = %q, want removal confirmation", got)
	}

	// Verify it's gone
	stdout.Reset()
	err = h.Get("rm-me")
	if err == nil {
		t.Fatal("Get() should error after remove")
	}
}

func TestRemove_WithExplicitScope(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	// Add to local scope
	jsonStr := `{"command":"echo"}`
	if err := h.AddJSON("scoped", jsonStr, "local"); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()

	// Remove with explicit scope
	err := h.Remove("scoped", "local")
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Removed MCP server scoped from local config") {
		t.Errorf("Remove() output = %q, want removal confirmation", got)
	}
}

func TestRemove_NotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)

	err := h.Remove("nonexistent", "")
	if err == nil {
		t.Fatal("Remove() expected error for nonexistent server")
	}
	if !strings.Contains(err.Error(), "no MCP server found") {
		t.Errorf("error = %q, want 'no MCP server found'", err.Error())
	}
}

func TestRemove_MultipleScopes(t *testing.T) {
	h, _, stderr := newTestHandler(t)

	// Add to both local and project scopes
	jsonStr := `{"command":"echo"}`
	if err := h.AddJSON("multi", jsonStr, "local"); err != nil {
		t.Fatal(err)
	}

	// Also add to project scope via .mcp.json
	writeMCPJSON(t, h.CWD, map[string]mcp.ServerConfig{
		"multi": {Command: "echo"},
	})

	err := h.Remove("multi", "")
	if err == nil {
		t.Fatal("Remove() expected error for multi-scope server")
	}

	got := stderr.String()
	if !strings.Contains(got, `MCP server "multi" exists in multiple scopes:`) {
		t.Errorf("stderr = %q, want multi-scope message", got)
	}
	if !strings.Contains(got, "claude mcp remove") {
		t.Errorf("stderr = %q, want remove hint", got)
	}
}

func TestGet_NotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)

	err := h.Get("ghost")
	if err == nil {
		t.Fatal("Get() expected error for nonexistent server")
	}
	if !strings.Contains(err.Error(), "no MCP server found with name: ghost") {
		t.Errorf("error = %q, want 'no MCP server found'", err.Error())
	}
}

func TestGet_SSEWithOAuth(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	jsonStr := `{"type":"sse","url":"https://mcp.example.com","oauth":{"clientId":"abc","callbackPort":8080}}`
	if err := h.AddJSON("oauth-srv", jsonStr, "local"); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()

	err := h.Get("oauth-srv")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "Type: sse") {
		t.Errorf("missing Type: sse, got: %q", got)
	}
	if !strings.Contains(got, "OAuth: client_id configured, callback_port 8080") {
		t.Errorf("missing OAuth info, got: %q", got)
	}
}

func TestScopeFlag_UserScope(t *testing.T) {
	// This test verifies the -s flag for user scope.
	// We can't easily test user scope (writes to ~/.claude/) in unit tests,
	// so we just verify scope validation works.
	h, _, _ := newTestHandler(t)

	// Verify valid scopes are accepted
	for _, s := range []string{"user", "project", "local"} {
		scope, err := mcp.EnsureConfigScope(s)
		if err != nil {
			t.Errorf("EnsureConfigScope(%q) error: %v", s, err)
		}
		if string(scope) != s {
			t.Errorf("EnsureConfigScope(%q) = %q, want %q", s, scope, s)
		}
	}

	// Verify invalid scope is rejected
	_, err := mcp.EnsureConfigScope("invalid")
	if err == nil {
		t.Error("EnsureConfigScope('invalid') should error")
	}

	_ = h // silence unused
}

func TestResetChoices(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	err := h.ResetChoices()
	if err != nil {
		t.Fatalf("ResetChoices() error: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "server approvals and rejections have been reset") {
		t.Errorf("ResetChoices() output = %q, want reset confirmation", got)
	}
	if !strings.Contains(got, "prompted for approval") {
		t.Errorf("ResetChoices() output = %q, want approval prompt message", got)
	}
}

func TestAddJSON_ProjectScope(t *testing.T) {
	h, stdout, _ := newTestHandler(t)

	jsonStr := `{"command":"npx","args":["server"]}`
	err := h.AddJSON("proj-srv", jsonStr, "project")
	if err != nil {
		t.Fatalf("AddJSON() error: %v", err)
	}

	if !strings.Contains(stdout.String(), "project config") {
		t.Errorf("expected project scope, got: %q", stdout.String())
	}

	// Verify .mcp.json was created
	data, err := os.ReadFile(filepath.Join(h.CWD, ".mcp.json"))
	if err != nil {
		t.Fatalf("expected .mcp.json, error: %v", err)
	}

	var cfg mcp.MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid .mcp.json: %v", err)
	}

	srv, ok := cfg.Servers["proj-srv"]
	if !ok {
		t.Fatal("server 'proj-srv' not found in .mcp.json")
	}
	if srv.Command != "npx" {
		t.Errorf("command = %q, want 'npx'", srv.Command)
	}
}
