package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestListMcpResourcesTool(t *testing.T) {
	tool := &tools.ListMcpResourcesTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "ListMcpResources" {
			t.Errorf("expected 'ListMcpResources', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("ListMcpResources should be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
	})

	t.Run("returns_no_resources", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "No MCP resources configured") {
			t.Errorf("expected 'No MCP resources configured', got %q", out.Content)
		}
	})
}

func TestReadMcpResourceTool(t *testing.T) {
	tool := &tools.ReadMcpResourceTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "ReadMcpResource" {
			t.Errorf("expected 'ReadMcpResource', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("ReadMcpResource should be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["uri"]; !ok {
			t.Error("schema missing 'uri' property")
		}
	})

	t.Run("returns_error_for_any_uri", func(t *testing.T) {
		input := json.RawMessage(`{"uri": "mcp://example/resource"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for any URI (no resources configured)")
		}
		if !strings.Contains(out.Content, "not found") {
			t.Errorf("expected 'not found' in error, got %q", out.Content)
		}
	})

	t.Run("empty_uri_returns_error", func(t *testing.T) {
		input := json.RawMessage(`{"uri": ""}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty URI")
		}
	})

	t.Run("invalid_json_returns_error", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})
}
