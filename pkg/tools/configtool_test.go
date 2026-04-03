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
		// Source: ConfigTool.ts:36-48 — setting/value schema
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["setting"]; !ok {
			t.Error("schema missing 'setting' property")
		}
		if _, ok := props["value"]; !ok {
			t.Error("schema missing 'value' property")
		}
		// setting should be required
		req, _ := parsed["required"].([]interface{})
		found := false
		for _, r := range req {
			if r == "setting" {
				found = true
			}
		}
		if !found {
			t.Error("setting should be required")
		}
	})

	t.Run("get_unset_setting", func(t *testing.T) {
		ct := tools.NewConfigTool()
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"setting": "theme"}`))
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

	t.Run("set_and_get", func(t *testing.T) {
		ct := tools.NewConfigTool()
		// Set
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"setting": "theme", "value": "dark"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error on set: %s", out.Content)
		}
		if !strings.Contains(out.Content, "theme") || !strings.Contains(out.Content, "dark") {
			t.Errorf("expected confirmation with setting/value, got %q", out.Content)
		}

		// Get
		out, err = ct.Execute(context.Background(), nil, json.RawMessage(`{"setting": "theme"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "dark") {
			t.Errorf("expected value 'dark', got %q", out.Content)
		}
	})

	t.Run("set_boolean_value", func(t *testing.T) {
		// Source: ConfigTool.ts:37 — value can be string, boolean, or number
		ct := tools.NewConfigTool()
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{"setting": "verbose", "value": true}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "verbose") {
			t.Errorf("expected verbose in output, got %q", out.Content)
		}
	})

	t.Run("missing_setting_param", func(t *testing.T) {
		ct := tools.NewConfigTool()
		out, err := ct.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing setting")
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
