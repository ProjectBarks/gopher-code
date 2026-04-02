package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestManagerRegisterTools(t *testing.T) {
	handler := func(method string, id int64, params json.RawMessage) json.RawMessage {
		switch method {
		case "tools/list":
			return json.RawMessage(`{"tools":[
				{"name":"read","description":"Read a file","inputSchema":{"type":"object"}},
				{"name":"write","description":"Write a file","inputSchema":{"type":"object"}}
			]}`)
		default:
			return json.RawMessage(`{}`)
		}
	}

	client := setupMockClient(t, handler)

	mgr := NewManager()
	mgr.mu.Lock()
	mgr.clients["files"] = client
	mgr.mu.Unlock()

	registry := tools.NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.RegisterTools(ctx, registry); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}

	// Check that tools were registered with prefixed names
	readTool := registry.Get("files__read")
	if readTool == nil {
		t.Error("expected files__read tool to be registered")
	}
	writeTool := registry.Get("files__write")
	if writeTool == nil {
		t.Error("expected files__write tool to be registered")
	}

	all := registry.All()
	if len(all) != 2 {
		t.Errorf("expected 2 tools, got %d", len(all))
	}
}

func TestManagerCloseAll(t *testing.T) {
	// Simply verify CloseAll doesn't panic with empty manager
	mgr := NewManager()
	mgr.CloseAll() // should not panic
}

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.clients == nil {
		t.Error("clients map is nil")
	}
	if len(mgr.clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(mgr.clients))
	}
}
