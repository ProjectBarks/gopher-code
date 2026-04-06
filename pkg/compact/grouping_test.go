package compact

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: services/compact/grouping.ts

// testIDFunc returns a MessageIDFunc using a pre-built index→ID map.
func testIDFunc(idMap map[int]string) MessageIDFunc {
	return func(_ message.Message, index int) string {
		return idMap[index]
	}
}

func TestGroupMessagesByAPIRound_Empty(t *testing.T) {
	groups := GroupMessagesByAPIRound(nil, nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(groups))
	}
}

func TestGroupMessagesByAPIRound_SingleUserMessage(t *testing.T) {
	msgs := []message.Message{message.UserMessage("hello")}
	groups := GroupMessagesByAPIRound(msgs, nil)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0]) != 1 {
		t.Errorf("expected 1 message in group, got %d", len(groups[0]))
	}
}

func TestGroupMessagesByAPIRound_TwoRounds(t *testing.T) {
	// TS behavior: first assistant triggers a boundary (id !== undefined).
	// Result: [user], [asst(A), user(tr)], [asst(B), user(tr)] = 3 groups.
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu1", "Read", json.RawMessage(`{}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("tu1", "file contents", false),
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu2", "Bash", json.RawMessage(`{}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("tu2", "bash output", false),
		}},
	}

	// API response IDs: index 1="resp-A", index 3="resp-B"
	idMap := map[int]string{1: "resp-A", 3: "resp-B"}
	groups := GroupMessagesByAPIRound(msgs, testIDFunc(idMap))
	// 3 groups: leading user, round A, round B
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[0]) != 1 {
		t.Errorf("first group (leading user): expected 1 message, got %d", len(groups[0]))
	}
	if len(groups[1]) != 2 {
		t.Errorf("second group (round A): expected 2 messages, got %d", len(groups[1]))
	}
	if len(groups[2]) != 2 {
		t.Errorf("third group (round B): expected 2 messages, got %d", len(groups[2]))
	}
}

func TestGroupMessagesByAPIRound_SameIDStaysInOneGroup(t *testing.T) {
	// Streaming: multiple assistant messages with same API response ID.
	// First assistant still triggers boundary from undefined → "X", but after
	// that, subsequent assistants with same ID "X" stay in the same group.
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu1", "Read", json.RawMessage(`{}`)),
		}},
		{Role: message.RoleUser, Content: []message.ContentBlock{
			message.ToolResultBlock("tu1", "file1", false),
		}},
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.ToolUseBlock("tu2", "Bash", json.RawMessage(`{}`)),
		}},
	}

	// Same API response ID "X" for both assistant messages.
	idMap := map[int]string{1: "X", 3: "X"}
	groups := GroupMessagesByAPIRound(msgs, testIDFunc(idMap))
	// 2 groups: [user], [asst(X), user(tr), asst(X)]
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (leading user + same-ID round), got %d", len(groups))
	}
	if len(groups[0]) != 1 {
		t.Errorf("first group: expected 1 (user), got %d", len(groups[0]))
	}
	if len(groups[1]) != 3 {
		t.Errorf("second group: expected 3 (asst+user+asst, same ID), got %d", len(groups[1]))
	}
}

func TestGroupMessagesByAPIRound_TextOnlyAssistants(t *testing.T) {
	// Two distinct text-only assistant messages = three groups (leading user + 2 rounds).
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.TextBlock("response 1"),
		}},
		message.UserMessage("followup"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.TextBlock("response 2"),
		}},
	}

	idMap := map[int]string{1: "resp-1", 3: "resp-2"}
	groups := GroupMessagesByAPIRound(msgs, testIDFunc(idMap))
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups (leading user, round 1, round 2), got %d", len(groups))
	}
}

func TestGroupMessagesByAPIRound_NoAssistant(t *testing.T) {
	// Only user messages — one group (no boundaries fire).
	msgs := []message.Message{
		message.UserMessage("hello"),
		message.UserMessage("world"),
	}
	groups := GroupMessagesByAPIRound(msgs, nil)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}

func TestGroupMessagesByAPIRound_NilIDFunc(t *testing.T) {
	// Without an ID function, all IDs are "" — no boundaries fire.
	msgs := []message.Message{
		message.UserMessage("hello"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.TextBlock("a"),
		}},
		message.UserMessage("next"),
		{Role: message.RoleAssistant, Content: []message.ContentBlock{
			message.TextBlock("b"),
		}},
	}
	groups := GroupMessagesByAPIRound(msgs, nil)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group with nil idFunc (all IDs empty), got %d", len(groups))
	}
}

func TestGroupMessagesByToolUseID(t *testing.T) {
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			message.ToolUseBlock("abc123", "Read", json.RawMessage(`{}`)),
		},
	}
	id := GroupMessagesByToolUseID(msg, 0)
	if id != "abc123" {
		t.Errorf("expected abc123, got %q", id)
	}

	userMsg := message.UserMessage("hello")
	if got := GroupMessagesByToolUseID(userMsg, 0); got != "" {
		t.Errorf("expected empty for user message, got %q", got)
	}
}

func TestGroupMessagesByAPIRound_ManyRounds(t *testing.T) {
	// 5 rounds, each: user + assistant.
	// First assistant triggers boundary from "" → "resp-0".
	// Total groups: 1 (leading user) + 5 (each round starts at a new assistant) = 6.
	// Wait — let me trace: [u, a(0), u, a(1), u, a(2), u, a(3), u, a(4)]
	// 1. u → current=[u]
	// 2. a(0): "resp-0" != "" → PUSH [u], current=[a(0)]
	// 3. u → current=[a(0),u]
	// 4. a(1): "resp-1" != "resp-0" → PUSH [a(0),u], current=[a(1)]
	// ...continues...
	// Result: [u], [a(0),u], [a(1),u], [a(2),u], [a(3),u], [a(4)] = 6 groups
	var msgs []message.Message
	idMap := make(map[int]string)
	for i := 0; i < 5; i++ {
		msgs = append(msgs, message.UserMessage(fmt.Sprintf("msg-%d", i)))
		aIdx := len(msgs)
		msgs = append(msgs, message.Message{
			Role: message.RoleAssistant,
			Content: []message.ContentBlock{
				message.TextBlock(fmt.Sprintf("resp-%d", i)),
			},
		})
		idMap[aIdx] = fmt.Sprintf("resp-%d", i)
	}
	groups := GroupMessagesByAPIRound(msgs, testIDFunc(idMap))
	if len(groups) != 6 {
		t.Fatalf("expected 6 groups (leading user + 5 rounds), got %d", len(groups))
	}
	// First group is just the leading user message.
	if len(groups[0]) != 1 {
		t.Errorf("first group: expected 1 message, got %d", len(groups[0]))
	}
	// Each subsequent group has assistant + user (except last which has just assistant).
	for i := 1; i < 5; i++ {
		if len(groups[i]) != 2 {
			t.Errorf("group %d: expected 2 messages, got %d", i, len(groups[i]))
		}
	}
	// Last group: just the final assistant.
	if len(groups[5]) != 1 {
		t.Errorf("last group: expected 1 message, got %d", len(groups[5]))
	}
}
