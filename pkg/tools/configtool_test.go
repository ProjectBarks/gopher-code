package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestConfigTool(t *testing.T) {
	tool := tools.NewConfigTool()

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "Config" {
			t.Errorf("expected 'Config', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("ConfigTool should be read-only")
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
		if _, ok := props["key"]; !ok {
			t.Error("schema missing 'key' property")
		}
		if _, ok := props["value"]; !ok {
			t.Error("schema missing 'value' property")
		}
		// Verify enum on action
		actionProp := props["action"].(map[string]interface{})
		enumRaw, ok := actionProp["enum"].([]interface{})
		if !ok {
			t.Fatal("action property missing enum")
		}
		if len(enumRaw) != 3 {
			t.Errorf("expected 3 enum values, got %d", len(enumRaw))
		}
	})

	t.Run("list_empty", func(t *testing.T) {
		ct := tools.NewConfigTool()
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "list"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if out.Content != "No configuration settings" {
			t.Errorf("expected empty config message, got %q", out.Content)
		}
	})

	t.Run("set_and_get", func(t *testing.T) {
		ct := tools.NewConfigTool()
		// Set
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "set", "key": "theme", "value": "dark"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error on set: %s", out.Content)
		}
		if !strings.Contains(out.Content, "theme") || !strings.Contains(out.Content, "dark") {
			t.Errorf("expected confirmation with key/value, got %q", out.Content)
		}

		// Get
		out, err = ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "get", "key": "theme"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error on get: %s", out.Content)
		}
		if !strings.Contains(out.Content, "dark") {
			t.Errorf("expected value 'dark', got %q", out.Content)
		}
	})

	t.Run("get_unset_key", func(t *testing.T) {
		ct := tools.NewConfigTool()
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "get", "key": "missing"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "not set") {
			t.Errorf("expected 'not set' message, got %q", out.Content)
		}
	})

	t.Run("list_with_entries", func(t *testing.T) {
		ct := tools.NewConfigTool()
		ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "set", "key": "b_key", "value": "val_b"}`))
		ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "set", "key": "a_key", "value": "val_a"}`))

		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "list"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Should be sorted alphabetically
		aIdx := strings.Index(out.Content, "a_key")
		bIdx := strings.Index(out.Content, "b_key")
		if aIdx < 0 || bIdx < 0 {
			t.Errorf("expected both keys in output, got %q", out.Content)
		}
		if aIdx > bIdx {
			t.Errorf("expected a_key before b_key, got %q", out.Content)
		}
	})

	t.Run("get_missing_key_param", func(t *testing.T) {
		ct := tools.NewConfigTool()
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "get"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing key")
		}
	})

	t.Run("set_missing_key_param", func(t *testing.T) {
		ct := tools.NewConfigTool()
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "set"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing key on set")
		}
	})

	t.Run("unknown_action", func(t *testing.T) {
		ct := tools.NewConfigTool()
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"action": "delete"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for unknown action")
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
