package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestSendMessageTool(t *testing.T) {
	// Source: tools/SendMessageTool/SendMessageTool.ts

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
		// Source: SendMessageTool.ts:60-80
		if _, ok := props["to"]; !ok {
			t.Error("schema missing 'to' property")
		}
		if _, ok := props["message"]; !ok {
			t.Error("schema missing 'message' property")
		}
	})

	t.Run("no_mailbox_returns_team_error", func(t *testing.T) {
		// Source: SendMessageTool.ts — not in team context
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"to": "alice", "message": "hello"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error when no mailbox configured")
		}
		if !strings.Contains(out.Content, "not in a team context") {
			t.Errorf("expected team context error, got %q", out.Content)
		}
	})

	t.Run("direct_message_with_mailbox", func(t *testing.T) {
		// Source: SendMessageTool.ts:140-190
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		toolWithMb := &tools.SendMessageTool{
			Mailbox:    mb,
			TeamName:   "test-team",
			SenderName: "bob",
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "alice", "message": "hello alice!", "summary": "greeting"}`)
		out, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "alice") {
			t.Errorf("expected confirmation mentioning alice, got %q", out.Content)
		}

		// Verify message was written to mailbox
		messages, _ := mb.ReadMailbox("alice", "test-team")
		if len(messages) != 1 {
			t.Fatalf("expected 1 message in alice's inbox, got %d", len(messages))
		}
		if messages[0].From != "bob" {
			t.Errorf("from = %q, want 'bob'", messages[0].From)
		}
	})
}
