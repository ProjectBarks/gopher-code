package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestSendMessageTool(t *testing.T) {
	tool := &tools.SendMessageTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "SendMessage" {
			t.Errorf("expected 'SendMessage', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("SendMessageTool should be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["recipient"]; !ok {
			t.Error("schema missing 'recipient' property")
		}
		if _, ok := props["message"]; !ok {
			t.Error("schema missing 'message' property")
		}
	})

	t.Run("returns_error_about_multi_agent", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"recipient": "agent1", "message": "hello"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error output for unsupported feature")
		}
		if !strings.Contains(out.Content, "multi-agent") {
			t.Errorf("expected 'multi-agent' in error message, got %q", out.Content)
		}
	})
}
