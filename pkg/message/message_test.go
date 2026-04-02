package message_test

import (
	"encoding/json"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

func TestUserMessage(t *testing.T) {
	m := message.UserMessage("hello")

	t.Run("role_is_user", func(t *testing.T) {
		if m.Role != message.RoleUser {
			t.Errorf("expected user, got %s", m.Role)
		}
	})
	t.Run("one_content_block", func(t *testing.T) {
		if len(m.Content) != 1 {
			t.Fatalf("expected 1, got %d", len(m.Content))
		}
	})
	t.Run("block_type_text", func(t *testing.T) {
		if m.Content[0].Type != message.ContentText {
			t.Error("expected text type")
		}
	})
	t.Run("block_text", func(t *testing.T) {
		if m.Content[0].Text != "hello" {
			t.Errorf("expected hello, got %s", m.Content[0].Text)
		}
	})
}

func TestTextBlock(t *testing.T) {
	b := message.TextBlock("test")
	t.Run("type", func(t *testing.T) {
		if b.Type != message.ContentText {
			t.Error("wrong type")
		}
	})
	t.Run("text", func(t *testing.T) {
		if b.Text != "test" {
			t.Error("wrong text")
		}
	})
}

func TestToolUseBlock(t *testing.T) {
	input := json.RawMessage(`{"command":"ls"}`)
	b := message.ToolUseBlock("t1", "Bash", input)

	t.Run("type", func(t *testing.T) {
		if b.Type != message.ContentToolUse {
			t.Error("wrong type")
		}
	})
	t.Run("id", func(t *testing.T) {
		if b.ID != "t1" {
			t.Error("wrong id")
		}
	})
	t.Run("name", func(t *testing.T) {
		if b.Name != "Bash" {
			t.Error("wrong name")
		}
	})
	t.Run("input", func(t *testing.T) {
		var parsed map[string]interface{}
		json.Unmarshal(b.Input, &parsed)
		if parsed["command"] != "ls" {
			t.Error("wrong input")
		}
	})
}

func TestToolResultBlock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		b := message.ToolResultBlock("t1", "output", false)
		t.Run("type", func(t *testing.T) {
			if b.Type != message.ContentToolResult {
				t.Error("wrong type")
			}
		})
		t.Run("tool_use_id", func(t *testing.T) {
			if b.ToolUseID != "t1" {
				t.Error("wrong tool_use_id")
			}
		})
		t.Run("content", func(t *testing.T) {
			if b.Content != "output" {
				t.Error("wrong content")
			}
		})
		t.Run("not_error", func(t *testing.T) {
			if b.IsError {
				t.Error("should not be error")
			}
		})
	})
	t.Run("error", func(t *testing.T) {
		b := message.ToolResultBlock("t2", "failed", true)
		t.Run("is_error", func(t *testing.T) {
			if !b.IsError {
				t.Error("should be error")
			}
		})
	})
}

func TestToolUses(t *testing.T) {
	m := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "thinking"},
			{Type: message.ContentToolUse, ID: "t1", Name: "Bash"},
			{Type: message.ContentText, Text: "more thinking"},
			{Type: message.ContentToolUse, ID: "t2", Name: "Read"},
		},
	}

	uses := m.ToolUses()
	t.Run("count", func(t *testing.T) {
		if len(uses) != 2 {
			t.Fatalf("expected 2, got %d", len(uses))
		}
	})
	t.Run("first_id", func(t *testing.T) {
		if uses[0].ID != "t1" {
			t.Error("wrong first id")
		}
	})
	t.Run("second_id", func(t *testing.T) {
		if uses[1].ID != "t2" {
			t.Error("wrong second id")
		}
	})
	t.Run("first_name", func(t *testing.T) {
		if uses[0].Name != "Bash" {
			t.Error("wrong first name")
		}
	})
	t.Run("second_name", func(t *testing.T) {
		if uses[1].Name != "Read" {
			t.Error("wrong second name")
		}
	})
}

func TestToolUsesEmpty(t *testing.T) {
	m := message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "just text"}},
	}
	uses := m.ToolUses()
	t.Run("empty", func(t *testing.T) {
		if len(uses) != 0 {
			t.Errorf("expected 0, got %d", len(uses))
		}
	})
}

func TestContentBlockSerialization(t *testing.T) {
	blocks := []struct {
		name  string
		block message.ContentBlock
		check func(t *testing.T, data map[string]interface{})
	}{
		{
			"text_block",
			message.TextBlock("hello"),
			func(t *testing.T, data map[string]interface{}) {
				t.Run("has_type", func(t *testing.T) {
					if data["type"] != "text" {
						t.Error("wrong type")
					}
				})
				t.Run("has_text", func(t *testing.T) {
					if data["text"] != "hello" {
						t.Error("wrong text")
					}
				})
			},
		},
		{
			"tool_use_block",
			message.ToolUseBlock("id1", "Bash", json.RawMessage(`{"cmd":"ls"}`)),
			func(t *testing.T, data map[string]interface{}) {
				t.Run("has_type", func(t *testing.T) {
					if data["type"] != "tool_use" {
						t.Error("wrong type")
					}
				})
				t.Run("has_id", func(t *testing.T) {
					if data["id"] != "id1" {
						t.Error("wrong id")
					}
				})
				t.Run("has_name", func(t *testing.T) {
					if data["name"] != "Bash" {
						t.Error("wrong name")
					}
				})
				t.Run("has_input", func(t *testing.T) {
					if data["input"] == nil {
						t.Error("missing input")
					}
				})
			},
		},
		{
			"tool_result_block",
			message.ToolResultBlock("id1", "output", false),
			func(t *testing.T, data map[string]interface{}) {
				t.Run("has_type", func(t *testing.T) {
					if data["type"] != "tool_result" {
						t.Error("wrong type")
					}
				})
				t.Run("has_tool_use_id", func(t *testing.T) {
					if data["tool_use_id"] != "id1" {
						t.Error("wrong tool_use_id")
					}
				})
			},
		},
		{
			"tool_result_error",
			message.ToolResultBlock("id2", "failed", true),
			func(t *testing.T, data map[string]interface{}) {
				t.Run("is_error_true", func(t *testing.T) {
					if data["is_error"] != true {
						t.Error("is_error should be true")
					}
				})
			},
		},
	}

	for _, tc := range blocks {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.block)
			t.Run("marshal_ok", func(t *testing.T) {
				if err != nil {
					t.Fatalf("marshal failed: %v", err)
				}
			})

			var parsed map[string]interface{}
			json.Unmarshal(data, &parsed)
			tc.check(t, parsed)

			// Roundtrip
			t.Run("roundtrip", func(t *testing.T) {
				var restored message.ContentBlock
				if err := json.Unmarshal(data, &restored); err != nil {
					t.Fatalf("unmarshal failed: %v", err)
				}
				if restored.Type != tc.block.Type {
					t.Errorf("type mismatch: %s != %s", restored.Type, tc.block.Type)
				}
			})
		})
	}
}

func TestMessageSerialization(t *testing.T) {
	m := message.UserMessage("test")
	data, err := json.Marshal(m)
	t.Run("marshal_ok", func(t *testing.T) {
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("json_has_role", func(t *testing.T) {
		var parsed map[string]interface{}
		json.Unmarshal(data, &parsed)
		if parsed["role"] != "user" {
			t.Error("wrong role in JSON")
		}
	})
	t.Run("json_has_content", func(t *testing.T) {
		var parsed map[string]interface{}
		json.Unmarshal(data, &parsed)
		content, ok := parsed["content"].([]interface{})
		if !ok || len(content) == 0 {
			t.Error("missing or empty content")
		}
	})
	t.Run("roundtrip", func(t *testing.T) {
		var restored message.Message
		json.Unmarshal(data, &restored)
		if restored.Role != m.Role {
			t.Error("role mismatch")
		}
		if len(restored.Content) != len(m.Content) {
			t.Error("content length mismatch")
		}
	})
}

func TestRoleConstants(t *testing.T) {
	t.Run("user", func(t *testing.T) {
		if string(message.RoleUser) != "user" {
			t.Error("wrong")
		}
	})
	t.Run("assistant", func(t *testing.T) {
		if string(message.RoleAssistant) != "assistant" {
			t.Error("wrong")
		}
	})
}

func TestContentBlockTypeConstants(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		if string(message.ContentText) != "text" {
			t.Error("wrong")
		}
	})
	t.Run("tool_use", func(t *testing.T) {
		if string(message.ContentToolUse) != "tool_use" {
			t.Error("wrong")
		}
	})
	t.Run("tool_result", func(t *testing.T) {
		if string(message.ContentToolResult) != "tool_result" {
			t.Error("wrong")
		}
	})
}
