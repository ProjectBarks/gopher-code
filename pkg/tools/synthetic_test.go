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
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["text"]; !ok {
			t.Error("schema missing 'text' property")
		}
		// Verify additionalProperties is false
		if ap, ok := parsed["additionalProperties"]; !ok || ap != false {
			t.Error("additionalProperties should be false")
		}
	})

	t.Run("returns_text_as_is", func(t *testing.T) {
		input := json.RawMessage(`{"text": "Hello, world!"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if out.Content != "Hello, world!" {
			t.Errorf("expected 'Hello, world!', got %q", out.Content)
		}
	})

	t.Run("returns_multiline_text", func(t *testing.T) {
		input := json.RawMessage(`{"text": "line1\nline2\nline3"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if out.Content != "line1\nline2\nline3" {
			t.Errorf("expected multiline text, got %q", out.Content)
		}
	})

	t.Run("empty_text", func(t *testing.T) {
		input := json.RawMessage(`{"text": ""}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty text")
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
