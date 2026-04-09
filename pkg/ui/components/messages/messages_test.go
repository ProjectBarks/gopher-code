package messages

import (
	"strings"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/message"
)

func TestRenderConversation_Empty(t *testing.T) {
	got := RenderConversation(nil, RenderOptions{Width: 80})
	if got != "" {
		t.Error("empty should return empty")
	}
}

func TestRenderConversation_Basic(t *testing.T) {
	msgs := []RenderableMessage{
		{Type: "user", Message: message.UserMessage("Hello")},
		{Type: "assistant", Message: message.AssistantMessage("Hi there!")},
	}
	got := RenderConversation(msgs, RenderOptions{Width: 80})
	if !strings.Contains(got, "Hello") {
		t.Error("should contain user text")
	}
	if !strings.Contains(got, "Hi there!") {
		t.Error("should contain assistant text")
	}
}

func TestRenderMessage_User(t *testing.T) {
	rm := RenderableMessage{
		Type:    "user",
		Message: message.UserMessage("What is Go?"),
	}
	got := RenderMessage(rm, RenderOptions{Width: 80})
	if !strings.Contains(got, "What is Go?") {
		t.Errorf("should contain text: %q", got)
	}
	if !strings.Contains(got, "❯") {
		t.Error("should have user indicator")
	}
}

func TestRenderMessage_Assistant(t *testing.T) {
	rm := RenderableMessage{
		Type:    "assistant",
		Message: message.AssistantMessage("Go is a programming language."),
	}
	got := RenderMessage(rm, RenderOptions{Width: 80})
	if !strings.Contains(got, "Go is a programming language.") {
		t.Errorf("should contain text: %q", got)
	}
}

func TestRenderMessage_AssistantStreaming(t *testing.T) {
	rm := RenderableMessage{
		Type:        "assistant",
		Message:     message.AssistantMessage("partial text"),
		IsStreaming: true,
	}
	got := RenderMessage(rm, RenderOptions{Width: 80})
	if !strings.Contains(got, "▌") {
		t.Error("streaming should show cursor")
	}
}

func TestRenderMessage_ToolUse(t *testing.T) {
	rm := RenderableMessage{
		Type: "assistant",
		Message: message.Message{
			Role: message.RoleAssistant,
			Content: []message.ContentBlock{
				{Type: message.ContentToolUse, Name: "Bash", IsLoading: true},
			},
		},
	}
	got := RenderMessage(rm, RenderOptions{Width: 80})
	if !strings.Contains(got, "Bash") {
		t.Error("should show tool name")
	}
	if !strings.Contains(got, "⎿") {
		t.Error("should show tool connector")
	}
}

func TestRenderMessage_Thinking(t *testing.T) {
	rm := RenderableMessage{
		Type: "assistant",
		Message: message.Message{
			Role: message.RoleAssistant,
			Content: []message.ContentBlock{
				{Type: message.ContentThinking, Thinking: "Let me think about this..."},
			},
		},
	}
	// Without verbose, thinking should not appear
	got := RenderMessage(rm, RenderOptions{Width: 80, Verbose: false})
	if strings.Contains(got, "think about") {
		t.Error("non-verbose should not show thinking")
	}

	// With verbose, thinking should appear
	got = RenderMessage(rm, RenderOptions{Width: 80, Verbose: true})
	if !strings.Contains(got, "think about") {
		t.Error("verbose should show thinking")
	}
}

func TestRenderMessage_System(t *testing.T) {
	rm := RenderableMessage{
		Type:    "system",
		Message: message.Message{Content: []message.ContentBlock{{Type: message.ContentText, Text: "Session started"}}},
	}
	got := RenderMessage(rm, RenderOptions{Width: 80})
	if !strings.Contains(got, "Session started") {
		t.Error("should show system text")
	}
}

func TestRenderMessage_Collapsed(t *testing.T) {
	rm := RenderableMessage{
		Type:          "collapsed",
		CollapsedText: "Read 3 files",
	}
	got := RenderMessage(rm, RenderOptions{Width: 80})
	if !strings.Contains(got, "Read 3 files") {
		t.Error("should show collapsed text")
	}
}

func TestRenderMessage_ToolResult_Error(t *testing.T) {
	rm := RenderableMessage{
		Type: "user",
		Message: message.Message{
			Role: message.RoleUser,
			Content: []message.ContentBlock{
				{Type: message.ContentToolResult, Content: "command not found", IsError: true},
			},
		},
	}
	got := RenderMessage(rm, RenderOptions{Width: 80})
	if !strings.Contains(got, "Error") {
		t.Error("should show error indicator")
	}
}

func TestCountByRole(t *testing.T) {
	msgs := []RenderableMessage{
		{Type: "user"}, {Type: "assistant"}, {Type: "user"}, {Type: "system"}, {Type: "assistant"},
	}
	u, a := CountByRole(msgs)
	if u != 2 || a != 2 {
		t.Errorf("user=%d assistant=%d", u, a)
	}
}

func TestLastAssistantText(t *testing.T) {
	msgs := []RenderableMessage{
		{Type: "assistant", Message: message.AssistantMessage("first")},
		{Type: "user", Message: message.UserMessage("question")},
		{Type: "assistant", Message: message.AssistantMessage("last answer")},
	}
	got := LastAssistantText(msgs)
	if got != "last answer" {
		t.Errorf("got %q", got)
	}
}

func TestLastAssistantText_None(t *testing.T) {
	msgs := []RenderableMessage{{Type: "user", Message: message.UserMessage("hi")}}
	if LastAssistantText(msgs) != "" {
		t.Error("no assistant should return empty")
	}
}

func TestHasToolInProgress(t *testing.T) {
	msgs := []RenderableMessage{
		{Type: "assistant", Message: message.Message{
			Content: []message.ContentBlock{{Type: message.ContentToolUse, Name: "Bash", IsLoading: true}},
		}},
	}
	if !HasToolInProgress(msgs) {
		t.Error("should detect in-progress tool")
	}

	msgs2 := []RenderableMessage{
		{Type: "assistant", Message: message.AssistantMessage("done")},
	}
	if HasToolInProgress(msgs2) {
		t.Error("no tools should return false")
	}
}

func TestConversationToRenderable(t *testing.T) {
	conv := []message.Message{
		message.UserMessage("hello"),
		message.AssistantMessage("hi"),
	}
	result := ConversationToRenderable(conv)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result[0].Type != "user" {
		t.Error("first should be user")
	}
	if result[1].Type != "assistant" {
		t.Error("second should be assistant")
	}
}

func TestRenderTurnSeparator(t *testing.T) {
	got := RenderTurnSeparator(40)
	if !strings.Contains(got, "─") {
		t.Error("should contain separator char")
	}
}

func TestRenderNewMessagesDivider(t *testing.T) {
	got := RenderNewMessagesDivider(3, 60)
	if !strings.Contains(got, "3 new messages") {
		t.Error("should show count")
	}

	got = RenderNewMessagesDivider(1, 60)
	if !strings.Contains(got, "1 new message") {
		t.Error("singular should not have 's'")
	}
}

func TestFormatTimestamp(t *testing.T) {
	now := time.Now()
	got := formatTimestamp(now)
	if got == "" {
		t.Error("should produce a timestamp")
	}
	// Today's timestamps should be HH:MM format
	if !strings.Contains(got, ":") {
		t.Error("should contain colon")
	}
}

func TestTruncateContent(t *testing.T) {
	if truncateContent("short", 100) != "short" {
		t.Error("short should not truncate")
	}
	got := truncateContent("this is a long string that should be truncated", 10)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("should end with ellipsis: %q", got)
	}
	if len(got) > 14 { // 10 bytes + multi-byte "…"
		t.Errorf("should truncate: %q (len=%d)", got, len(got))
	}
}

func TestRenderMessage_Timestamps(t *testing.T) {
	rm := RenderableMessage{
		Type:      "user",
		Message:   message.UserMessage("hi"),
		Timestamp: time.Now(),
	}
	got := RenderMessage(rm, RenderOptions{Width: 80, ShowTimestamps: true})
	if !strings.Contains(got, ":") {
		t.Error("should show timestamp when enabled")
	}
}
