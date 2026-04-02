package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestToolSearchTool(t *testing.T) {
	// Set up a registry with a few tools for testing
	registry := tools.NewRegistry()
	registry.Register(&tools.FileReadTool{})
	registry.Register(&tools.FileWriteTool{})
	registry.Register(&tools.FileEditTool{})
	registry.Register(&tools.GlobTool{})
	registry.Register(&tools.BashTool{})

	tool := tools.NewToolSearchTool(registry)

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "ToolSearch" {
			t.Errorf("expected 'ToolSearch', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("ToolSearchTool should be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
	})

	t.Run("search_by_name", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"query": "Bash"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Bash") {
			t.Errorf("expected Bash in results, got %q", out.Content)
		}
	})

	t.Run("search_by_description_keyword", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"query": "file"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Should match file-related tools
		if !strings.Contains(out.Content, "Read") && !strings.Contains(out.Content, "Write") && !strings.Contains(out.Content, "Edit") {
			t.Errorf("expected file tools in results, got %q", out.Content)
		}
	})

	t.Run("search_no_match", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"query": "xyznonexistent"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "No tools found") {
			t.Errorf("expected 'No tools found' message, got %q", out.Content)
		}
	})

	t.Run("max_results_limits_output", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		// "file" matches multiple tools; limit to 1
		input := json.RawMessage(`{"query": "file", "max_results": 1}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Found 1 tool") {
			t.Errorf("expected exactly 1 result, got %q", out.Content)
		}
	})

	t.Run("empty_query_returns_error", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"query": ""}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty query")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{bad}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("case_insensitive_search", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"query": "bash"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Bash") {
			t.Errorf("case-insensitive search should find Bash, got %q", out.Content)
		}
	})
}
