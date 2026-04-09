package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMCPToolName(t *testing.T) {
	tool := NewMCPTool(nil, "myserver", ToolInfo{Name: "echo", Description: "Echoes input"})
	// Name should follow mcp__<server>__<tool> convention
	if got := tool.Name(); got != "mcp__myserver__echo" {
		t.Errorf("Name() = %q, want %q", got, "mcp__myserver__echo")
	}
}

func TestMCPToolNameNormalization(t *testing.T) {
	// Server name with spaces gets normalized
	tool := NewMCPTool(nil, "my server", ToolInfo{Name: "my-tool"})
	if got := tool.Name(); got != "mcp__my_server__my-tool" {
		t.Errorf("Name() = %q, want %q", got, "mcp__my_server__my-tool")
	}
}

func TestMCPToolMetadata(t *testing.T) {
	tool := NewMCPTool(nil, "srv", ToolInfo{
		Name:        "query",
		Description: "Run a query",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	})
	if tool.ServerName() != "srv" {
		t.Errorf("ServerName() = %q", tool.ServerName())
	}
	if tool.ToolName() != "query" {
		t.Errorf("ToolName() = %q", tool.ToolName())
	}
	if tool.Description() != "Run a query" {
		t.Errorf("Description() = %q", tool.Description())
	}
	if string(tool.InputSchema()) != `{"type":"object"}` {
		t.Errorf("InputSchema() = %s", tool.InputSchema())
	}
	if !tool.IsReadOnly() {
		t.Error("should be read-only by default")
	}
	if !strings.Contains(tool.UserFacingName(), "query") {
		t.Error("UserFacingName should contain tool name")
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
	tool := NewMCPTool(client, "test", ToolInfo{Name: "greet"})

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
	tool := NewMCPTool(client, "test", ToolInfo{Name: "fail"})

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
	tool := NewMCPTool(client, "test", ToolInfo{Name: "multi"})

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

func TestMCPToolExecuteTruncation(t *testing.T) {
	// Build a response with text exceeding MaxResultSizeChars
	bigText := strings.Repeat("x", MaxResultSizeChars+100)
	handler := func(method string, id int64, params json.RawMessage) json.RawMessage {
		switch method {
		case "tools/call":
			return json.RawMessage(`{"content":[{"type":"text","text":"` + bigText + `"}],"isError":false}`)
		default:
			return json.RawMessage(`{}`)
		}
	}

	client := setupMockClient(t, handler)
	tool := NewMCPTool(client, "test", ToolInfo{Name: "big"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := tool.Execute(ctx, nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasSuffix(output.Content, "[truncated]") {
		t.Error("large output should be truncated")
	}
	if len(output.Content) > MaxResultSizeChars+50 {
		t.Errorf("output too large: %d chars", len(output.Content))
	}
}

func TestClassifyMCPToolForCollapse(t *testing.T) {
	tests := []struct {
		name     string
		isSearch bool
		isRead   bool
	}{
		{"search_code", true, false},        // GitHub search
		{"searchCode", true, false},         // camelCase normalized
		{"search-code", true, false},        // kebab-case normalized
		{"get_file_contents", false, true},  // GitHub read
		{"getFileContents", false, true},    // camelCase
		{"send_message", false, false},      // not classified
		{"slack_search_public", true, false}, // Slack search
		{"git_status", false, true},         // Git read
		{"kubectl_get", false, true},        // k8s read
		{"unknown_tool", false, false},      // unknown → conservative
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ClassifyMCPToolForCollapse(tt.name)
			if c.IsSearch != tt.isSearch {
				t.Errorf("IsSearch = %v, want %v", c.IsSearch, tt.isSearch)
			}
			if c.IsRead != tt.isRead {
				t.Errorf("IsRead = %v, want %v", c.IsRead, tt.isRead)
			}
		})
	}
}

func TestNormalizeMCPToolName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"search_code", "search_code"},
		{"searchCode", "search_code"},
		{"search-code", "search_code"},
		{"SearchCode", "search_code"},
		{"git_status", "git_status"},
		{"gitStatus", "git_status"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeMCPToolName(tt.input); got != tt.want {
				t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
