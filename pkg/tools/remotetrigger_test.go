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
		if _, ok := props["action"]; !ok {
			t.Error("schema missing 'action' property")
		}
	})

	t.Run("list_requires_no_id", func(t *testing.T) {
		// list without OAuth token → auth error (expected in test)
		input := json.RawMessage(`{"action": "list"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should fail with auth error since no OAuth configured in test
		if !out.IsError {
			// If it succeeds, that's fine too (means auth is configured)
			return
		}
		if !strings.Contains(out.Content, "Not authenticated") && !strings.Contains(out.Content, "failed") {
			t.Errorf("expected auth or connection error, got: %s", out.Content)
		}
	})

	t.Run("get_without_auth", func(t *testing.T) {
		// Without OAuth, returns auth error (param validation happens after auth)
		input := json.RawMessage(`{"action": "get"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("should fail without auth")
		}
	})

	t.Run("create_without_auth", func(t *testing.T) {
		input := json.RawMessage(`{"action": "create", "body": {"name": "test"}}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("should fail without auth")
		}
	})

	t.Run("unknown_action", func(t *testing.T) {
		input := json.RawMessage(`{"action": "delete"}`)
		out, err := tool.Execute(context.Background(), nil, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("should fail for unknown action")
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
