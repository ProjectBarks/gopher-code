package lsp

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/project")
	if m.Status() != InitNotStarted {
		t.Errorf("status = %q, want not-started", m.Status())
	}
	if m.IsConnected() {
		t.Error("should not be connected initially")
	}
	if m.ServerCount() != 0 {
		t.Error("should have no servers")
	}
}

func TestManager_RegisterServer(t *testing.T) {
	m := NewManager("/project")
	m.RegisterServer(".go", "gopls", []string{"serve"})
	m.RegisterServer(".ts", "typescript-language-server", []string{"--stdio"})

	if m.ServerCount() != 2 {
		t.Errorf("server count = %d, want 2", m.ServerCount())
	}

	exts := m.Extensions()
	sort.Strings(exts)
	if len(exts) != 2 || exts[0] != ".go" || exts[1] != ".ts" {
		t.Errorf("extensions = %v", exts)
	}
}

func TestManager_IsConnected(t *testing.T) {
	m := NewManager("/project")
	if m.IsConnected() {
		t.Error("should not be connected with no servers")
	}

	m.RegisterServer(".go", "gopls", nil)
	// After registration, server is in "ready" state (lazy start)
	if !m.IsConnected() {
		t.Error("should be connected after registering a ready server")
	}
}

func TestManager_IsConnectedAllError(t *testing.T) {
	m := NewManager("/project")
	m.RegisterServer(".go", "gopls", nil)

	// Manually set to error state
	m.mu.Lock()
	m.servers[".go"].State = ServerError
	m.mu.Unlock()

	if m.IsConnected() {
		t.Error("should not be connected when all servers are in error state")
	}
}

func TestManager_SendRequest_NoServer(t *testing.T) {
	m := NewManager("/project")
	result, err := m.SendRequest(nil, "/test.rs", "textDocument/hover", nil)
	if err != nil {
		t.Fatalf("no server should return nil, not error: %v", err)
	}
	if result != nil {
		t.Error("should return nil result for unregistered extension")
	}
}

func TestManager_Shutdown(t *testing.T) {
	m := NewManager("/project")
	m.RegisterServer(".go", "gopls", nil)
	m.Shutdown() // should not panic even without active clients
}

func TestInitStatus_Constants(t *testing.T) {
	if InitNotStarted != "not-started" {
		t.Error("wrong")
	}
	if InitPending != "pending" {
		t.Error("wrong")
	}
	if InitSuccess != "success" {
		t.Error("wrong")
	}
	if InitFailed != "failed" {
		t.Error("wrong")
	}
}

func TestServerState_Constants(t *testing.T) {
	if ServerReady != "ready" {
		t.Error("wrong")
	}
	if ServerError != "error" {
		t.Error("wrong")
	}
}

func TestServerEntry(t *testing.T) {
	entry := ServerEntry{
		Command:   "gopls",
		Args:      []string{"serve"},
		State:     ServerReady,
		Extension: ".go",
	}
	if entry.Command != "gopls" {
		t.Error("wrong command")
	}
	if entry.Extension != ".go" {
		t.Error("wrong extension")
	}
}

func TestJsonRPCError(t *testing.T) {
	err := &jsonRPCError{Code: -32600, Message: "Invalid Request"}
	s := err.Error()
	if s != "LSP error -32600: Invalid Request" {
		t.Errorf("error string = %q", s)
	}
}

func TestJsonRPCRequest_Marshal(t *testing.T) {
	id := int64(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params:  map[string]string{"rootUri": "file:///project"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("should produce non-empty JSON")
	}
}
