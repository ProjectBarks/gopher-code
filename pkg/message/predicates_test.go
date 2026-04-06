package message

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAssistantMessage(t *testing.T) {
	assert.True(t, IsAssistantMessage(Message{Role: RoleAssistant}))
	assert.False(t, IsAssistantMessage(Message{Role: RoleUser}))
}

func TestIsUserMessage(t *testing.T) {
	assert.True(t, IsUserMessage(Message{Role: RoleUser}))
	assert.False(t, IsUserMessage(Message{Role: RoleAssistant}))
}

func TestIsToolUseMessage(t *testing.T) {
	t.Run("assistant with tool_use", func(t *testing.T) {
		m := Message{Role: RoleAssistant, Content: []ContentBlock{
			ToolUseBlock("t1", "bash", json.RawMessage(`{}`)),
		}}
		assert.True(t, IsToolUseMessage(m))
	})

	t.Run("assistant with text only", func(t *testing.T) {
		m := Message{Role: RoleAssistant, Content: []ContentBlock{TextBlock("hello")}}
		assert.False(t, IsToolUseMessage(m))
	})

	t.Run("user message with tool_use blocks is false", func(t *testing.T) {
		// Only assistant messages count
		m := Message{Role: RoleUser, Content: []ContentBlock{
			ToolUseBlock("t1", "bash", json.RawMessage(`{}`)),
		}}
		assert.False(t, IsToolUseMessage(m))
	})

	t.Run("mixed content", func(t *testing.T) {
		m := Message{Role: RoleAssistant, Content: []ContentBlock{
			TextBlock("I'll run this"),
			ToolUseBlock("t1", "bash", json.RawMessage(`{}`)),
		}}
		assert.True(t, IsToolUseMessage(m))
	})

	t.Run("empty content", func(t *testing.T) {
		m := Message{Role: RoleAssistant, Content: nil}
		assert.False(t, IsToolUseMessage(m))
	})
}

func TestIsToolResultMessage(t *testing.T) {
	t.Run("user with tool_result first", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: []ContentBlock{
			ToolResultBlock("t1", "ok", false),
		}}
		assert.True(t, IsToolResultMessage(m))
	})

	t.Run("user with text first", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: []ContentBlock{TextBlock("hello")}}
		assert.False(t, IsToolResultMessage(m))
	})

	t.Run("assistant message", func(t *testing.T) {
		m := Message{Role: RoleAssistant, Content: []ContentBlock{
			ToolResultBlock("t1", "ok", false),
		}}
		assert.False(t, IsToolResultMessage(m))
	})

	t.Run("empty content", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: nil}
		assert.False(t, IsToolResultMessage(m))
	})
}

func TestIsHumanTurn(t *testing.T) {
	t.Run("plain user message", func(t *testing.T) {
		assert.True(t, IsHumanTurn(UserMessage("hello")))
	})

	t.Run("tool_result message is not human", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: []ContentBlock{
			ToolResultBlock("t1", "ok", false),
		}}
		assert.False(t, IsHumanTurn(m))
	})

	t.Run("assistant message", func(t *testing.T) {
		assert.False(t, IsHumanTurn(Message{Role: RoleAssistant}))
	})

	t.Run("empty user message", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: nil}
		assert.True(t, IsHumanTurn(m))
	})
}

func TestIsSyntheticMessage(t *testing.T) {
	t.Run("interrupt message", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: []ContentBlock{TextBlock(InterruptMessage)}}
		assert.True(t, IsSyntheticMessage(m))
	})

	t.Run("cancel message", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: []ContentBlock{TextBlock(CancelMessage)}}
		assert.True(t, IsSyntheticMessage(m))
	})

	t.Run("regular message", func(t *testing.T) {
		m := UserMessage("hello")
		assert.False(t, IsSyntheticMessage(m))
	})

	t.Run("empty content", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: nil}
		assert.False(t, IsSyntheticMessage(m))
	})

	t.Run("tool_use first block", func(t *testing.T) {
		m := Message{Role: RoleAssistant, Content: []ContentBlock{
			ToolUseBlock("t1", "bash", json.RawMessage(`{}`)),
		}}
		assert.False(t, IsSyntheticMessage(m))
	})
}

func TestGetLastAssistantMessage(t *testing.T) {
	t.Run("returns last assistant", func(t *testing.T) {
		msgs := []Message{
			UserMessage("q1"),
			{Role: RoleAssistant, Content: []ContentBlock{TextBlock("a1")}},
			UserMessage("q2"),
			{Role: RoleAssistant, Content: []ContentBlock{TextBlock("a2")}},
		}
		got := GetLastAssistantMessage(msgs)
		assert.NotNil(t, got)
		assert.Equal(t, "a2", got.Content[0].Text)
	})

	t.Run("no assistant messages", func(t *testing.T) {
		msgs := []Message{UserMessage("q1")}
		assert.Nil(t, GetLastAssistantMessage(msgs))
	})

	t.Run("empty slice", func(t *testing.T) {
		assert.Nil(t, GetLastAssistantMessage(nil))
	})
}

func TestHasToolCallsInLastAssistantTurn(t *testing.T) {
	t.Run("last assistant has tool_use", func(t *testing.T) {
		msgs := []Message{
			UserMessage("do it"),
			{Role: RoleAssistant, Content: []ContentBlock{
				ToolUseBlock("t1", "bash", json.RawMessage(`{}`)),
			}},
		}
		assert.True(t, HasToolCallsInLastAssistantTurn(msgs))
	})

	t.Run("last assistant has no tool_use", func(t *testing.T) {
		msgs := []Message{
			UserMessage("do it"),
			{Role: RoleAssistant, Content: []ContentBlock{TextBlock("done")}},
		}
		assert.False(t, HasToolCallsInLastAssistantTurn(msgs))
	})

	t.Run("no assistant messages", func(t *testing.T) {
		assert.False(t, HasToolCallsInLastAssistantTurn([]Message{UserMessage("hello")}))
	})
}

func TestHasSuccessfulToolCall(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		msgs := []Message{
			UserMessage("run bash"),
			{Role: RoleAssistant, Content: []ContentBlock{
				ToolUseBlock("t1", "bash", json.RawMessage(`{}`)),
			}},
			{Role: RoleUser, Content: []ContentBlock{
				ToolResultBlock("t1", "output", false),
			}},
		}
		assert.True(t, HasSuccessfulToolCall(msgs, "bash"))
	})

	t.Run("failed call", func(t *testing.T) {
		msgs := []Message{
			UserMessage("run bash"),
			{Role: RoleAssistant, Content: []ContentBlock{
				ToolUseBlock("t1", "bash", json.RawMessage(`{}`)),
			}},
			{Role: RoleUser, Content: []ContentBlock{
				ToolResultBlock("t1", "error!", true),
			}},
		}
		assert.False(t, HasSuccessfulToolCall(msgs, "bash"))
	})

	t.Run("no such tool", func(t *testing.T) {
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{TextBlock("hi")}},
		}
		assert.False(t, HasSuccessfulToolCall(msgs, "bash"))
	})

	t.Run("no matching result", func(t *testing.T) {
		msgs := []Message{
			UserMessage("run bash"),
			{Role: RoleAssistant, Content: []ContentBlock{
				ToolUseBlock("t1", "bash", json.RawMessage(`{}`)),
			}},
			// No tool_result
		}
		assert.False(t, HasSuccessfulToolCall(msgs, "bash"))
	})
}
