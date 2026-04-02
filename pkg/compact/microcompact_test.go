package compact

import (
	"encoding/json"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: services/compact/microCompact.ts

func TestCompactableTools(t *testing.T) {
	// Source: services/compact/microCompact.ts:41-50
	expected := []string{"Read", "Bash", "Grep", "Glob", "WebSearch", "WebFetch", "Edit", "Write"}
	for _, name := range expected {
		if !CompactableTools[name] {
			t.Errorf("expected %s to be compactable", name)
		}
	}
	// Tools NOT eligible
	notEligible := []string{"Agent", "NotebookEdit", "LS", "SendMessage", "TaskCreate"}
	for _, name := range notEligible {
		if CompactableTools[name] {
			t.Errorf("expected %s to NOT be compactable", name)
		}
	}
}

func TestCollectCompactableToolIDs(t *testing.T) {
	// Source: services/compact/microCompact.ts:226-241
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("t1", "Read", json.RawMessage(`{}`)),
			message.ToolUseBlock("t2", "Agent", json.RawMessage(`{}`)), // Not compactable
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("t1", "file contents", false),
			message.ToolResultBlock("t2", "agent output", false),
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("t3", "Bash", json.RawMessage(`{}`)),
		}},
	}
	ids := CollectCompactableToolIDs(msgs)
	if len(ids) != 2 {
		t.Fatalf("expected 2 compactable IDs, got %d", len(ids))
	}
	if ids[0] != "t1" || ids[1] != "t3" {
		t.Errorf("expected [t1, t3], got %v", ids)
	}
}

func TestMicroCompactMessages_KeepRecent(t *testing.T) {
	// Source: services/compact/microCompact.ts:461-462
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("t1", "Read", json.RawMessage(`{}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("t1", "old file content that is very long", false),
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("t2", "Read", json.RawMessage(`{}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("t2", "recent file content", false),
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.TextBlock("done"),
		}},
	}

	// keepRecent=1: only the last compactable result (t2) is kept
	result, saved := MicroCompactMessages(msgs, 1)
	if saved == 0 {
		t.Fatal("expected some tokens saved")
	}

	// t1 should be cleared
	for _, msg := range result {
		for _, b := range msg.Content {
			if b.Type == message.ContentToolResult && b.ToolUseID == "t1" {
				if b.Content != TimeBasedMCClearedMessage {
					t.Errorf("t1 should be cleared, got %q", b.Content)
				}
			}
			// t2 should be kept
			if b.Type == message.ContentToolResult && b.ToolUseID == "t2" {
				if b.Content == TimeBasedMCClearedMessage {
					t.Error("t2 should NOT be cleared (kept as recent)")
				}
			}
		}
	}
}

func TestMicroCompactMessages_KeepRecentFloor(t *testing.T) {
	// Source: services/compact/microCompact.ts:461 — floor at 1
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("t1", "Bash", json.RawMessage(`{}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("t1", "output", false),
		}},
	}

	// keepRecent=0 should be floored to 1
	_, saved := MicroCompactMessages(msgs, 0)
	if saved != 0 {
		t.Error("with only 1 compactable tool and keepRecent=1 (floored), nothing should be cleared")
	}
}

func TestMicroCompactMessages_NonCompactableToolsUntouched(t *testing.T) {
	// Source: services/compact/microCompact.ts:41-50 — only COMPACTABLE_TOOLS
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("t1", "Agent", json.RawMessage(`{}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("t1", "agent output that is important", false),
		}},
	}

	result, saved := MicroCompactMessages(msgs, 1)
	if saved != 0 {
		t.Error("non-compactable tool results should not be cleared")
	}
	for _, msg := range result {
		for _, b := range msg.Content {
			if b.Type == message.ContentToolResult && b.ToolUseID == "t1" {
				if b.Content == TimeBasedMCClearedMessage {
					t.Error("Agent tool result should NOT be cleared")
				}
			}
		}
	}
}

func TestAdjustIndexToPreserveAPIInvariants(t *testing.T) {
	// Source: services/compact/sessionMemoryCompact.ts:232-314

	t.Run("no_adjustment_needed", func(t *testing.T) {
		msgs := []message.Message{
			message.UserMessage("hello"),
			{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("hi")}},
			message.UserMessage("world"),
			{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("ok")}},
		}
		idx := AdjustIndexToPreserveAPIInvariants(msgs, 2)
		if idx != 2 {
			t.Errorf("expected 2, got %d", idx)
		}
	})

	t.Run("pulls_in_tool_use_for_orphaned_result", func(t *testing.T) {
		// Source: services/compact/sessionMemoryCompact.ts:269-285
		msgs := []message.Message{
			message.UserMessage("hello"),
			{Role: message.RoleAssistant, Content: []message.ContentBlock{
				message.ToolUseBlock("t1", "Read", json.RawMessage(`{}`)),
			}},
			{Role: message.RoleUser, Content: []message.ContentBlock{
				message.ToolResultBlock("t1", "content", false),
			}},
			{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("done")}},
		}
		// Start at index 2 (tool_result message) — should pull back to include the tool_use at index 1
		idx := AdjustIndexToPreserveAPIInvariants(msgs, 2)
		if idx != 1 {
			t.Errorf("expected 1 (pull back to include tool_use), got %d", idx)
		}
	})

	t.Run("already_includes_tool_use", func(t *testing.T) {
		msgs := []message.Message{
			message.UserMessage("hello"),
			{Role: message.RoleAssistant, Content: []message.ContentBlock{
				message.ToolUseBlock("t1", "Read", json.RawMessage(`{}`)),
			}},
			{Role: message.RoleUser, Content: []message.ContentBlock{
				message.ToolResultBlock("t1", "content", false),
			}},
		}
		// Start at index 1 — tool_use is already included
		idx := AdjustIndexToPreserveAPIInvariants(msgs, 1)
		if idx != 1 {
			t.Errorf("expected 1 (no adjustment needed), got %d", idx)
		}
	})

	t.Run("boundary_conditions", func(t *testing.T) {
		msgs := []message.Message{message.UserMessage("hello")}
		// startIndex 0 — no adjustment
		if idx := AdjustIndexToPreserveAPIInvariants(msgs, 0); idx != 0 {
			t.Errorf("expected 0, got %d", idx)
		}
		// startIndex >= len — no adjustment
		if idx := AdjustIndexToPreserveAPIInvariants(msgs, 5); idx != 5 {
			t.Errorf("expected 5, got %d", idx)
		}
	})
}

func TestEstimateMessageTokens(t *testing.T) {
	// Source: services/compact/microCompact.ts:164-205
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.TextBlock("hello world"), // 11 chars ~ 3 tokens
		}},
	}
	est := EstimateMessageTokens(msgs)
	// 11/4 = 2.75, ceil = 3, padded by 4/3 = 4
	if est != 4 {
		t.Errorf("expected ~4 estimated tokens, got %d", est)
	}
}

func TestRoughTokenCountEstimation(t *testing.T) {
	// 4 chars per token
	if got := RoughTokenCountEstimation("hello world!"); got != 3 { // 12/4=3
		t.Errorf("expected 3, got %d", got)
	}
	if got := RoughTokenCountEstimation(""); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}
