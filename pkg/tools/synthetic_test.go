package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestSyntheticOutputTool(t *testing.T) {
	tool := &tools.SyntheticOutputTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "SyntheticOutput" {
			t.Errorf("expected 'SyntheticOutput', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("SyntheticOutputTool should be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		if parsed["type"] != "object" {
			t.Error("schema type should be object")
		}
	})

	t.Run("returns_structured_output", func(t *testing.T) {
		input := json.RawMessage(`{"name": "test", "value": 42}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if out.Content == "" {
			t.Error("output should not be empty")
		}
	})

	t.Run("empty_object", func(t *testing.T) {
		input := json.RawMessage(`{}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Error("empty object should succeed (no schema to violate)")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})
}
