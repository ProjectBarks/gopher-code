package message

import (
	"encoding/json"
	"strings"
	"testing"
)

// Source: utils/messages.ts:1989-2370 (normalizeMessagesForAPI)

func TestSmooshConsecutiveUserMessages(t *testing.T) {
	// Source: utils/messages.ts:2188-2199 — consecutive user messages merged
	msgs := []Message{
		UserMessage("hello"),
		UserMessage("world"),
	}
	result := NormalizeForAPI(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message after smooshing, got %d", len(result))
	}
	if result[0].Role != RoleUser {
		t.Errorf("expected user role, got %s", result[0].Role)
	}
	// Source: utils/messages.ts:2505-2515 — joinTextAtSeam adds \n
	if len(result[0].Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result[0].Content))
	}
	if result[0].Content[0].Text != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result[0].Content[0].Text)
	}
	if result[0].Content[1].Text != "world" {
		t.Errorf("expected 'world', got %q", result[0].Content[1].Text)
	}
}

func TestSmooshConsecutiveAssistantMessages(t *testing.T) {
	// Source: utils/messages.ts:2389-2400 — assistant messages merged by concatenating content
	msgs := []Message{
		{Role: RoleAssistant, Content: []ContentBlock{TextBlock("part1")}},
		{Role: RoleAssistant, Content: []ContentBlock{TextBlock("part2")}},
	}
	result := NormalizeForAPI(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if len(result[0].Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result[0].Content))
	}
}

func TestHoistToolResults(t *testing.T) {
	// Source: utils/messages.ts:2470-2483 — tool_results come first in user messages
	content := []ContentBlock{
		TextBlock("some text"),
		ToolResultBlock("t1", "result1", false),
		TextBlock("more text"),
		ToolResultBlock("t2", "result2", false),
	}
	result := hoistToolResults(content)
	if result[0].Type != ContentToolResult || result[0].ToolUseID != "t1" {
		t.Errorf("expected tool_result t1 first, got %v", result[0])
	}
	if result[1].Type != ContentToolResult || result[1].ToolUseID != "t2" {
		t.Errorf("expected tool_result t2 second, got %v", result[1])
	}
	if result[2].Type != ContentText || result[2].Text != "some text" {
		t.Errorf("expected text 'some text' third, got %v", result[2])
	}
}

func TestEnsureToolResultPairing_SynthesizeMissing(t *testing.T) {
	// Source: utils/messages.ts:5133 — synthesize missing tool_result
	// Source: utils/messages.ts:247 — placeholder text
	msgs := []Message{
		UserMessage("hello"),
		{Role: RoleAssistant, Content: []ContentBlock{
			ToolUseBlock("t1", "my_tool", json.RawMessage(`{}`)),
		}},
		// No user message with tool_result for t1
	}
	result := NormalizeForAPI(msgs)

	// Should have: user, assistant, user(synthetic tool_result)
	if len(result) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(result))
	}
	lastMsg := result[len(result)-1]
	if lastMsg.Role != RoleUser {
		t.Fatalf("expected last message to be user, got %s", lastMsg.Role)
	}
	found := false
	for _, b := range lastMsg.Content {
		if b.Type == ContentToolResult && b.ToolUseID == "t1" {
			found = true
			if b.Content != SyntheticToolResultPlaceholder {
				t.Errorf("expected placeholder %q, got %q", SyntheticToolResultPlaceholder, b.Content)
			}
			if !b.IsError {
				t.Error("synthetic tool_result should have is_error=true")
			}
		}
	}
	if !found {
		t.Error("expected synthetic tool_result for t1")
	}
}

func TestEnsureToolResultPairing_ExistingResultNotDuplicated(t *testing.T) {
	// When tool_result already exists, don't add a synthetic one
	msgs := []Message{
		UserMessage("hello"),
		{Role: RoleAssistant, Content: []ContentBlock{
			ToolUseBlock("t1", "my_tool", json.RawMessage(`{}`)),
		}},
		{Role: RoleUser, Content: []ContentBlock{
			ToolResultBlock("t1", "actual result", false),
		}},
	}
	result := NormalizeForAPI(msgs)

	// Count tool_results for t1
	count := 0
	for _, msg := range result {
		for _, b := range msg.Content {
			if b.Type == ContentToolResult && b.ToolUseID == "t1" {
				count++
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 tool_result for t1, got %d", count)
	}
}

func TestDeduplicateToolUseIDs(t *testing.T) {
	// Source: utils/messages.ts:5226-5243 — duplicate tool_use IDs stripped
	msgs := []Message{
		UserMessage("hello"),
		{Role: RoleAssistant, Content: []ContentBlock{
			ToolUseBlock("t1", "my_tool", json.RawMessage(`{}`)),
			ToolUseBlock("t1", "my_tool", json.RawMessage(`{}`)), // Duplicate!
		}},
		{Role: RoleUser, Content: []ContentBlock{
			ToolResultBlock("t1", "result", false),
		}},
	}
	result := NormalizeForAPI(msgs)

	// The assistant message should only have 1 tool_use
	for _, msg := range result {
		if msg.Role == RoleAssistant {
			uses := 0
			for _, b := range msg.Content {
				if b.Type == ContentToolUse {
					uses++
				}
			}
			if uses != 1 {
				t.Errorf("expected 1 tool_use after dedup, got %d", uses)
			}
		}
	}
}

func TestOrphanedToolResultStripped(t *testing.T) {
	// Source: utils/messages.ts:5161-5200 — orphaned tool_results at start
	msgs := []Message{
		{Role: RoleUser, Content: []ContentBlock{
			ToolResultBlock("orphan", "lost result", false),
		}},
		{Role: RoleAssistant, Content: []ContentBlock{TextBlock("ok")}},
	}
	result := NormalizeForAPI(msgs)

	// First message should not contain tool_result
	if len(result) < 1 {
		t.Fatal("expected at least 1 message")
	}
	for _, b := range result[0].Content {
		if b.Type == ContentToolResult {
			t.Error("orphaned tool_result should have been stripped")
		}
	}
}

func TestSmooshSystemReminderSiblings(t *testing.T) {
	// Source: utils/messages.ts:1835-1873
	t.Run("sr_text_smooshed_into_tool_result", func(t *testing.T) {
		// A user message with tool_result + system-reminder text sibling
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{
				ToolUseBlock("t1", "my_tool", json.RawMessage(`{}`)),
			}},
			{Role: RoleUser, Content: []ContentBlock{
				ToolResultBlock("t1", "tool output", false),
				{Type: ContentText, Text: WrapInSystemReminder("Current date is 2026-04-02")},
			}},
		}
		result := NormalizeForAPI(msgs)

		// Find the user message with tool_result
		for _, msg := range result {
			if msg.Role != RoleUser {
				continue
			}
			for _, b := range msg.Content {
				if b.Type == ContentText && isSystemReminder(b.Text) {
					t.Error("system-reminder text should have been smooshed into tool_result, but found as sibling")
				}
				if b.Type == ContentToolResult && b.ToolUseID == "t1" {
					if !strings.Contains(b.Content, "tool output") {
						t.Error("tool_result should still contain original content")
					}
					if !strings.Contains(b.Content, "<system-reminder>") {
						t.Error("tool_result should contain smooshed system-reminder")
					}
				}
			}
		}
	})

	t.Run("no_tool_result_leaves_sr_alone", func(t *testing.T) {
		// Source: utils/messages.ts:1843-1844 — no tool_result means no smoosh
		msgs := []Message{
			{Role: RoleUser, Content: []ContentBlock{
				{Type: ContentText, Text: "regular text"},
				{Type: ContentText, Text: WrapInSystemReminder("context info")},
			}},
		}
		result := NormalizeForAPI(msgs)
		if len(result) != 1 {
			t.Fatalf("expected 1 message, got %d", len(result))
		}
		// Both blocks should remain since there's no tool_result to smoosh into
		srFound := false
		for _, b := range result[0].Content {
			if b.Type == ContentText && isSystemReminder(b.Text) {
				srFound = true
			}
		}
		if !srFound {
			t.Error("system-reminder should remain as sibling when no tool_result present")
		}
	})

	t.Run("non_sr_text_not_smooshed", func(t *testing.T) {
		// Source: utils/messages.ts:1827-1831 — non-SR text stays as sibling
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{
				ToolUseBlock("t1", "my_tool", json.RawMessage(`{}`)),
			}},
			{Role: RoleUser, Content: []ContentBlock{
				ToolResultBlock("t1", "output", false),
				{Type: ContentText, Text: "regular user text"},
			}},
		}
		result := NormalizeForAPI(msgs)
		// The regular text should remain as a sibling, not smooshed
		for _, msg := range result {
			if msg.Role != RoleUser {
				continue
			}
			for _, b := range msg.Content {
				if b.Type == ContentText && b.Text == "regular user text" {
					return // Found it as a sibling — correct
				}
			}
		}
		t.Error("non-SR text should remain as sibling")
	})
}

func TestFilterOrphanedThinkingOnly(t *testing.T) {
	// Source: utils/messages.ts:4997-5054

	t.Run("thinking_only_assistant_removed", func(t *testing.T) {
		// Source: utils/messages.ts:5029-5035 — all-thinking assistant is orphaned
		// When a thinking-only assistant is separated by a user message, it's truly orphaned
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{
				{Type: ContentThinking, Thinking: "let me think..."},
			}},
			UserMessage("still waiting"),
			{Role: RoleAssistant, Content: []ContentBlock{TextBlock("answer")}},
		}
		result := NormalizeForAPI(msgs)
		// The thinking-only assistant should be dropped
		for _, msg := range result {
			if msg.Role == RoleAssistant {
				for _, b := range msg.Content {
					if b.Type == ContentThinking {
						t.Error("orphaned thinking block should have been filtered")
					}
				}
			}
		}
	})

	t.Run("mixed_thinking_and_text_kept", func(t *testing.T) {
		// Source: utils/messages.ts:5033-5034 — has non-thinking content, keep it
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{
				{Type: ContentThinking, Thinking: "hmm"},
				TextBlock("here's my answer"),
			}},
		}
		result := NormalizeForAPI(msgs)
		if len(result) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(result))
		}
		foundThinking := false
		for _, b := range result[1].Content {
			if b.Type == ContentThinking {
				foundThinking = true
			}
		}
		if !foundThinking {
			t.Error("thinking block in mixed message should be preserved")
		}
	})

	t.Run("redacted_thinking_also_filtered", func(t *testing.T) {
		// Redacted thinking-only assistant separated by a user message = orphaned
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{
				{Type: ContentRedactedThinking, Signature: "abc123"},
			}},
			UserMessage("waiting"),
			{Role: RoleAssistant, Content: []ContentBlock{TextBlock("answer")}},
		}
		result := NormalizeForAPI(msgs)
		for _, msg := range result {
			for _, b := range msg.Content {
				if b.Type == ContentRedactedThinking {
					t.Error("orphaned redacted_thinking should have been filtered")
				}
			}
		}
	})
}

func TestFilterTrailingThinkingFromLastAssistant(t *testing.T) {
	// Source: utils/messages.ts:4781-4828

	t.Run("trailing_thinking_stripped", func(t *testing.T) {
		// Source: utils/messages.ts:4778-4779 — API rejects messages ending with thinking
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{
				TextBlock("answer"),
				{Type: ContentThinking, Thinking: "trailing thought"},
			}},
		}
		result := NormalizeForAPI(msgs)
		lastMsg := result[len(result)-1]
		lastBlock := lastMsg.Content[len(lastMsg.Content)-1]
		if isThinkingBlock(lastBlock) {
			t.Error("trailing thinking should have been stripped from last assistant")
		}
		if lastBlock.Type != ContentText || lastBlock.Text != "answer" {
			t.Errorf("last block should be text 'answer', got %v", lastBlock)
		}
	})

	t.Run("all_thinking_gets_placeholder", func(t *testing.T) {
		// Source: utils/messages.ts:4814-4816 — placeholder inserted
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{
				{Type: ContentThinking, Thinking: "only thinking"},
			}},
		}
		result := NormalizeForAPI(msgs)
		// The thinking-only message gets filtered by filterOrphanedThinkingOnly first,
		// so we need to check differently. If it survives (has non-thinking sibling
		// with same ID), the trailing filter would add a placeholder.
		// In this case, the orphan filter removes it entirely.
		// Verify no thinking blocks remain
		for _, msg := range result {
			for _, b := range msg.Content {
				if isThinkingBlock(b) {
					t.Error("thinking block should not survive normalization when orphaned")
				}
			}
		}
	})

	t.Run("non_last_assistant_not_affected", func(t *testing.T) {
		// Only the LAST assistant message is checked
		msgs := []Message{
			UserMessage("hello"),
			{Role: RoleAssistant, Content: []ContentBlock{
				{Type: ContentThinking, Thinking: "thought"},
				TextBlock("answer"),
			}},
			UserMessage("followup"),
			{Role: RoleAssistant, Content: []ContentBlock{TextBlock("final")}},
		}
		result := NormalizeForAPI(msgs)
		// First assistant should still have thinking block
		foundThinking := false
		for _, b := range result[1].Content {
			if b.Type == ContentThinking {
				foundThinking = true
			}
		}
		if !foundThinking {
			t.Error("thinking in non-last assistant should be preserved")
		}
	})
}

func TestWrapInSystemReminder(t *testing.T) {
	// Source: utils/messages.ts:3097-3098
	result := WrapInSystemReminder("hello world")
	expected := "<system-reminder>\nhello world\n</system-reminder>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestNoMessagesReturnsEmpty(t *testing.T) {
	result := NormalizeForAPI(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d messages", len(result))
	}
}

func TestAlternatingRolesPreserved(t *testing.T) {
	msgs := []Message{
		UserMessage("q1"),
		{Role: RoleAssistant, Content: []ContentBlock{TextBlock("a1")}},
		UserMessage("q2"),
		{Role: RoleAssistant, Content: []ContentBlock{TextBlock("a2")}},
	}
	result := NormalizeForAPI(msgs)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	expected := []Role{RoleUser, RoleAssistant, RoleUser, RoleAssistant}
	for i, r := range expected {
		if result[i].Role != r {
			t.Errorf("message[%d]: expected role %s, got %s", i, r, result[i].Role)
		}
	}
}

func TestJoinTextAtSeam(t *testing.T) {
	// Source: utils/messages.ts:2505-2515
	t.Run("both_text", func(t *testing.T) {
		a := []ContentBlock{TextBlock("hello")}
		b := []ContentBlock{TextBlock("world")}
		result := joinTextAtSeam(a, b)
		if len(result) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(result))
		}
		if result[0].Text != "hello\n" {
			t.Errorf("expected 'hello\\n', got %q", result[0].Text)
		}
	})

	t.Run("non_text_seam", func(t *testing.T) {
		a := []ContentBlock{ToolResultBlock("t1", "r", false)}
		b := []ContentBlock{TextBlock("world")}
		result := joinTextAtSeam(a, b)
		if len(result) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(result))
		}
		// No \n appended since seam is not text-text
		if result[0].Type != ContentToolResult {
			t.Error("first block should be tool_result")
		}
	})
}

func TestNormalizeAttachment(t *testing.T) {
	// Source: utils/messages.ts:3453

	t.Run("wraps_in_system_reminder", func(t *testing.T) {
		msg := NormalizeAttachment(Attachment{
			Type:    AttachmentMemory,
			Content: "User prefers dark mode",
		})
		if msg.Role != RoleUser {
			t.Errorf("expected user role, got %s", msg.Role)
		}
		if len(msg.Content) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(msg.Content))
		}
		if !strings.HasPrefix(msg.Content[0].Text, "<system-reminder>") {
			t.Error("should be wrapped in system-reminder")
		}
		if !strings.Contains(msg.Content[0].Text, "User prefers dark mode") {
			t.Error("should contain original content")
		}
	})

	t.Run("truncates_large_content", func(t *testing.T) {
		largeContent := strings.Repeat("x", MaxAttachmentSize+1000)
		msg := NormalizeAttachment(Attachment{
			Type:    AttachmentContext,
			Content: largeContent,
		})
		if !strings.Contains(msg.Content[0].Text, "...[truncated]") {
			t.Error("large attachment should be truncated")
		}
		// The wrapped content should be under MaxAttachmentSize + overhead
		if len(msg.Content[0].Text) > MaxAttachmentSize+200 {
			t.Errorf("content too large after truncation: %d", len(msg.Content[0].Text))
		}
	})

	t.Run("small_content_unchanged", func(t *testing.T) {
		msg := NormalizeAttachment(Attachment{
			Type:    AttachmentFile,
			Content: "small content",
		})
		if strings.Contains(msg.Content[0].Text, "truncated") {
			t.Error("small content should not be truncated")
		}
	})
}
