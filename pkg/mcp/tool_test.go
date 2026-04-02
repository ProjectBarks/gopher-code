package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMCPToolName(t *testing.T) {
	tool := &MCPTool{
		serverName: "myserver",
		info: ToolInfo{
			Name:        "echo",
			Description: "Echoes input",
		},
	}
	if got := tool.Name(); got != "myserver__echo" {
		t.Errorf("Name() = %q, want %q", got, "myserver__echo")
	}
}

func TestMCPToolDescription(t *testing.T) {
	tool := &MCPTool{
		info: ToolInfo{Description: "A tool"},
	}
	if got := tool.Description(); got != "A tool" {
		t.Errorf("Description() = %q, want %q", got, "A tool")
	}
}

func TestMCPToolInputSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	tool := &MCPTool{
		info: ToolInfo{InputSchema: schema},
	}
	if string(tool.InputSchema()) != `{"type":"object"}` {
		t.Errorf("InputSchema() = %s, want {\"type\":\"object\"}", tool.InputSchema())
	}
}

func TestMCPToolIsReadOnly(t *testing.T) {
	tool := &MCPTool{}
	if !tool.IsReadOnly() {
		t.Error("IsReadOnly() = false, want true")
	}
}

func TestMCPToolExecuteSuccess(t *testing.T) {
	handler := func(method string, id int64, params json.RawMessage) json.RawMessage {
		switch method {
		case "tools/call":
			return json.RawMessage(`{"content":[{"type":"text","text":"hello world"}],"isError":false}`)
		default:
			return json.RawMessage(`{}`)
		}
	}

	client := setupMockClient(t, handler)
	tool := &MCPTool{
		client:     client,
		serverName: "test",
		info:       ToolInfo{Name: "greet"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := tool.Execute(ctx, nil, json.RawMessage(`{"name":"world"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if output.IsError {
		t.Error("unexpected error output")
	}
	if output.Content != "hello world" {
		t.Errorf("content = %q, want %q", output.Content, "hello world")
	}
}

func TestMCPToolExecuteIsError(t *testing.T) {
	handler := func(method string, id int64, params json.RawMessage) json.RawMessage {
		switch method {
		case "tools/call":
			return json.RawMessage(`{"content":[{"type":"text","text":"something went wrong"}],"isError":true}`)
		default:
			return json.RawMessage(`{}`)
		}
	}

	client := setupMockClient(t, handler)
	tool := &MCPTool{
		client:     client,
		serverName: "test",
		info:       ToolInfo{Name: "fail"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := tool.Execute(ctx, nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !output.IsError {
		t.Error("expected error output")
	}
	if output.Content != "something went wrong" {
		t.Errorf("content = %q, want %q", output.Content, "something went wrong")
	}
}

func TestMCPToolExecuteMultipleTextContent(t *testing.T) {
	handler := func(method string, id int64, params json.RawMessage) json.RawMessage {
		switch method {
		case "tools/call":
			return json.RawMessage(`{"content":[{"type":"text","text":"line1"},{"type":"text","text":"line2"},{"type":"image","data":"abc"}],"isError":false}`)
		default:
			return json.RawMessage(`{}`)
		}
	}

	client := setupMockClient(t, handler)
	tool := &MCPTool{
		client:     client,
		serverName: "test",
		info:       ToolInfo{Name: "multi"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := tool.Execute(ctx, nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if output.Content != "line1line2" {
		t.Errorf("content = %q, want %q", output.Content, "line1line2")
	}
}
