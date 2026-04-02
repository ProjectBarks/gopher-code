package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestRemoteTriggerTool(t *testing.T) {
	tool := &tools.RemoteTriggerTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "RemoteTrigger" {
			t.Errorf("expected 'RemoteTrigger', got %q", tool.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("RemoteTriggerTool should not be read-only")
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
		if _, ok := props["agent"]; !ok {
			t.Error("schema missing 'agent' property")
		}
		if _, ok := props["prompt"]; !ok {
			t.Error("schema missing 'prompt' property")
		}
		// Verify additionalProperties is false
		if ap, ok := parsed["additionalProperties"]; !ok || ap != false {
			t.Error("additionalProperties should be false")
		}
	})

	t.Run("returns_not_configured", func(t *testing.T) {
		input := json.RawMessage(`{"agent": "my-agent", "prompt": "do something"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error output (not configured)")
		}
		if !strings.Contains(out.Content, "not configured") {
			t.Errorf("expected 'not configured' in output, got %q", out.Content)
		}
	})

	t.Run("missing_agent", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"agent": "", "prompt": "x"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing agent")
		}
	})

	t.Run("missing_prompt", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"agent": "a", "prompt": ""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing prompt")
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
