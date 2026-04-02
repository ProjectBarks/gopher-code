package session_test

import (
	"encoding/json"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/session"
)

func TestNewSession(t *testing.T) {
	cfg := session.DefaultConfig()
	s := session.New(cfg, "/tmp")

	t.Run("has_id", func(t *testing.T) {
		if s.ID == "" {
			t.Error("session ID is empty")
		}
	})
	t.Run("id_is_uuid", func(t *testing.T) {
		if len(s.ID) < 32 {
			t.Errorf("session ID too short: %s", s.ID)
		}
	})
	t.Run("unique_ids", func(t *testing.T) {
		s2 := session.New(cfg, "/tmp")
		if s.ID == s2.ID {
			t.Error("two sessions have same ID")
		}
	})
	t.Run("empty_messages", func(t *testing.T) {
		if len(s.Messages) != 0 {
			t.Errorf("expected 0 messages, got %d", len(s.Messages))
		}
	})
	t.Run("cwd_set", func(t *testing.T) {
		if s.CWD != "/tmp" {
			t.Errorf("expected CWD=/tmp, got %s", s.CWD)
		}
	})
	t.Run("turn_count_zero", func(t *testing.T) {
		if s.TurnCount != 0 {
			t.Errorf("expected 0, got %d", s.TurnCount)
		}
	})
	t.Run("tokens_zero", func(t *testing.T) {
		if s.TotalInputTokens != 0 || s.TotalOutputTokens != 0 {
			t.Error("tokens should start at 0")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := session.DefaultConfig()

	t.Run("model_set", func(t *testing.T) {
		if cfg.Model == "" {
			t.Error("model is empty")
		}
	})
	t.Run("max_turns", func(t *testing.T) {
		if cfg.MaxTurns != 100 {
			t.Errorf("expected 100, got %d", cfg.MaxTurns)
		}
	})
	t.Run("permission_mode", func(t *testing.T) {
		if cfg.PermissionMode != permissions.AutoApprove {
			t.Errorf("expected AutoApprove, got %d", cfg.PermissionMode)
		}
	})
	t.Run("token_budget_context_window", func(t *testing.T) {
		if cfg.TokenBudget.ContextWindow != 200000 {
			t.Errorf("expected 200000, got %d", cfg.TokenBudget.ContextWindow)
		}
	})
}

func TestPushMessage(t *testing.T) {
	s := session.New(session.DefaultConfig(), "/tmp")

	s.PushMessage(message.UserMessage("hello"))
	t.Run("one_message", func(t *testing.T) {
		if len(s.Messages) != 1 {
			t.Errorf("expected 1, got %d", len(s.Messages))
		}
	})

	s.PushMessage(message.Message{Role: message.RoleAssistant, Content: []message.ContentBlock{{Type: message.ContentText, Text: "hi"}}})
	t.Run("two_messages", func(t *testing.T) {
		if len(s.Messages) != 2 {
			t.Errorf("expected 2, got %d", len(s.Messages))
		}
	})
	t.Run("first_is_user", func(t *testing.T) {
		if s.Messages[0].Role != message.RoleUser {
			t.Error("first should be user")
		}
	})
	t.Run("second_is_assistant", func(t *testing.T) {
		if s.Messages[1].Role != message.RoleAssistant {
			t.Error("second should be assistant")
		}
	})
}

func TestToRequestMessages(t *testing.T) {
	s := session.New(session.DefaultConfig(), "/tmp")
	s.PushMessage(message.UserMessage("hello"))
	s.PushMessage(message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "I'll help"},
			{Type: message.ContentToolUse, ID: "t1", Name: "Bash", Input: json.RawMessage(`{"command":"ls"}`)},
		},
	})
	s.PushMessage(message.Message{
		Role: message.RoleUser,
		Content: []message.ContentBlock{
			{Type: message.ContentToolResult, ToolUseID: "t1", Content: "file.txt", IsError: false},
		},
	})

	msgs := s.ToRequestMessages()

	t.Run("message_count", func(t *testing.T) {
		if len(msgs) != 3 {
			t.Fatalf("expected 3, got %d", len(msgs))
		}
	})
	t.Run("user_text_message", func(t *testing.T) {
		m := msgs[0]
		if m.Role != "user" {
			t.Errorf("expected user, got %s", m.Role)
		}
		if len(m.Content) != 1 {
			t.Fatalf("expected 1 block, got %d", len(m.Content))
		}
		if m.Content[0].Type != "text" {
			t.Error("expected text type")
		}
		if m.Content[0].Text != "hello" {
			t.Errorf("expected hello, got %s", m.Content[0].Text)
		}
	})
	t.Run("assistant_tool_use_message", func(t *testing.T) {
		m := msgs[1]
		if m.Role != "assistant" {
			t.Error("expected assistant")
		}
		if len(m.Content) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(m.Content))
		}
		t.Run("text_block", func(t *testing.T) {
			if m.Content[0].Type != "text" {
				t.Error("first block should be text")
			}
		})
		t.Run("tool_use_block", func(t *testing.T) {
			b := m.Content[1]
			if b.Type != "tool_use" {
				t.Error("second block should be tool_use")
			}
			if b.ID != "t1" {
				t.Errorf("expected id t1, got %s", b.ID)
			}
			if b.Name != "Bash" {
				t.Errorf("expected Bash, got %s", b.Name)
			}
		})
	})
	t.Run("tool_result_message", func(t *testing.T) {
		m := msgs[2]
		if m.Role != "user" {
			t.Error("tool_result should be in user message")
		}
		if len(m.Content) != 1 {
			t.Fatalf("expected 1 block, got %d", len(m.Content))
		}
		b := m.Content[0]
		t.Run("type", func(t *testing.T) {
			if b.Type != "tool_result" {
				t.Error("expected tool_result")
			}
		})
		t.Run("tool_use_id", func(t *testing.T) {
			if b.ToolUseID != "t1" {
				t.Errorf("expected t1, got %s", b.ToolUseID)
			}
		})
		t.Run("content", func(t *testing.T) {
			if b.Content != "file.txt" {
				t.Errorf("expected file.txt, got %s", b.Content)
			}
		})
		t.Run("is_error_omitted_when_false", func(t *testing.T) {
			if b.IsError != nil {
				t.Error("is_error should be nil (omitted) when false")
			}
		})
	})
}

func TestToRequestMessagesErrorResult(t *testing.T) {
	s := session.New(session.DefaultConfig(), "/tmp")
	s.PushMessage(message.Message{
		Role: message.RoleUser,
		Content: []message.ContentBlock{
			{Type: message.ContentToolResult, ToolUseID: "t1", Content: "denied", IsError: true},
		},
	})

	msgs := s.ToRequestMessages()
	b := msgs[0].Content[0]
	t.Run("is_error_present_when_true", func(t *testing.T) {
		if b.IsError == nil || *b.IsError != true {
			t.Error("is_error should be true")
		}
	})
}

func TestSessionSerialization(t *testing.T) {
	cfg := session.SessionConfig{
		Model:          "test-model",
		SystemPrompt:   "Be helpful",
		MaxTurns:       50,
		TokenBudget:    compact.TokenBudget{ContextWindow: 100000, MaxOutputTokens: 8000, CompactThreshold: 0.7},
		PermissionMode: permissions.Deny,
	}
	s := session.New(cfg, "/workspace")
	s.PushMessage(message.UserMessage("test"))
	s.TurnCount = 5
	s.TotalInputTokens = 1000
	s.TotalOutputTokens = 500

	data, err := json.Marshal(s)
	t.Run("marshal_succeeds", func(t *testing.T) {
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
	})
	t.Run("json_non_empty", func(t *testing.T) {
		if len(data) == 0 {
			t.Error("JSON is empty")
		}
	})

	var restored session.SessionState
	err = json.Unmarshal(data, &restored)
	t.Run("unmarshal_succeeds", func(t *testing.T) {
		if err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
	})
	t.Run("id_preserved", func(t *testing.T) {
		if restored.ID != s.ID {
			t.Errorf("ID %s != %s", restored.ID, s.ID)
		}
	})
	t.Run("cwd_preserved", func(t *testing.T) {
		if restored.CWD != "/workspace" {
			t.Errorf("CWD %s != /workspace", restored.CWD)
		}
	})
	t.Run("messages_preserved", func(t *testing.T) {
		if len(restored.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(restored.Messages))
		}
	})
	t.Run("turn_count_preserved", func(t *testing.T) {
		if restored.TurnCount != 5 {
			t.Errorf("expected 5, got %d", restored.TurnCount)
		}
	})
	t.Run("input_tokens_preserved", func(t *testing.T) {
		if restored.TotalInputTokens != 1000 {
			t.Errorf("expected 1000, got %d", restored.TotalInputTokens)
		}
	})
	t.Run("output_tokens_preserved", func(t *testing.T) {
		if restored.TotalOutputTokens != 500 {
			t.Errorf("expected 500, got %d", restored.TotalOutputTokens)
		}
	})
}
