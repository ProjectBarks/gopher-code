package messages

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

func TestRenderAssistantBlock_Text(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: "Hello world"}
	got := RenderAssistantBlock(block, AssistantBlockOptions{})
	if !strings.Contains(got, "Hello world") {
		t.Errorf("should contain text: %q", got)
	}
}

func TestRenderAssistantBlock_TextEmpty(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: ""}
	got := RenderAssistantBlock(block, AssistantBlockOptions{})
	if got != "" {
		t.Errorf("empty text should return empty: %q", got)
	}
}

func TestRenderAssistantBlock_TextWithDot(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: "response"}
	got := RenderAssistantBlock(block, AssistantBlockOptions{ShowDot: true})
	if !strings.Contains(got, "●") {
		t.Error("ShowDot should render role indicator")
	}
}

func TestRenderAssistantBlock_ToolUse(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentToolUse, Name: "Bash"}
	got := RenderAssistantBlock(block, AssistantBlockOptions{})
	if !strings.Contains(got, "Bash") {
		t.Errorf("should contain tool name: %q", got)
	}
	if !strings.Contains(got, ResponseConnector) {
		t.Error("should have response connector")
	}
}

func TestRenderAssistantBlock_ToolUseLoading(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentToolUse, Name: "Read", IsLoading: true}
	got := RenderAssistantBlock(block, AssistantBlockOptions{})
	if !strings.Contains(got, "…") {
		t.Error("loading should show spinner")
	}
}

func TestRenderAssistantBlock_ToolUseConcurrent(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentToolUse, Name: "Bash", IsLoading: true}
	got := RenderAssistantBlock(block, AssistantBlockOptions{ToolCallCount: 3})
	if !strings.Contains(got, "3 concurrent") {
		t.Errorf("should show concurrent count: %q", got)
	}
}

func TestRenderAssistantBlock_Thinking(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentThinking, Thinking: "Let me analyze this..."}

	// Non-verbose: hidden
	got := RenderAssistantBlock(block, AssistantBlockOptions{Verbose: false})
	if got != "" {
		t.Error("thinking should be hidden in non-verbose mode")
	}

	// Verbose: shown
	got = RenderAssistantBlock(block, AssistantBlockOptions{Verbose: true})
	if !strings.Contains(got, "analyze") {
		t.Errorf("verbose should show thinking: %q", got)
	}
	if !strings.Contains(got, "💭") {
		t.Error("should have thinking emoji")
	}
}

func TestRenderAssistantBlock_ThinkingTruncated(t *testing.T) {
	longThinking := strings.Repeat("think ", 200)
	block := message.ContentBlock{Type: message.ContentThinking, Thinking: longThinking}
	got := RenderAssistantBlock(block, AssistantBlockOptions{Verbose: true})
	if !strings.HasSuffix(strings.TrimSpace(got), "…") {
		// The truncation adds "…" but it may be inside ANSI codes
		if len(got) > 600 {
			t.Error("long thinking should be truncated")
		}
	}
}

func TestRenderAssistantBlock_RedactedThinking(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentRedactedThinking}

	got := RenderAssistantBlock(block, AssistantBlockOptions{Verbose: false})
	if got != "" {
		t.Error("redacted should be hidden in non-verbose")
	}

	got = RenderAssistantBlock(block, AssistantBlockOptions{Verbose: true})
	if !strings.Contains(got, "hidden") {
		t.Errorf("verbose should show redacted: %q", got)
	}
}

func TestRenderAssistantBlock_APIError(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: "API Error: something went wrong"}
	got := RenderAssistantBlock(block, AssistantBlockOptions{})
	if !strings.Contains(got, "⚠") {
		t.Error("API error should show warning icon")
	}
}

func TestRenderAssistantBlock_RateLimit(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: "Rate limit exceeded, retry after 30s"}
	got := RenderAssistantBlock(block, AssistantBlockOptions{})
	if !strings.Contains(got, "⏳") {
		t.Error("rate limit should show clock icon")
	}
}

func TestRenderAssistantBlock_Interrupted(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: "Response interrupted by user"}
	got := RenderAssistantBlock(block, AssistantBlockOptions{})
	if !strings.Contains(got, "⏸") {
		t.Error("interrupted should show pause icon")
	}
}

func TestRenderAssistantMessage_Full(t *testing.T) {
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentThinking, Thinking: "analyzing..."},
			{Type: message.ContentText, Text: "Here is the answer."},
			{Type: message.ContentToolUse, Name: "Bash", IsLoading: true},
		},
	}

	// Non-verbose: thinking hidden
	got := RenderAssistantMessage(msg, AssistantBlockOptions{Verbose: false})
	if strings.Contains(got, "analyzing") {
		t.Error("non-verbose should hide thinking")
	}
	if !strings.Contains(got, "Here is the answer") {
		t.Error("should contain text")
	}
	if !strings.Contains(got, "Bash") {
		t.Error("should contain tool use")
	}

	// Verbose: thinking shown
	got = RenderAssistantMessage(msg, AssistantBlockOptions{Verbose: true})
	if !strings.Contains(got, "analyzing") {
		t.Error("verbose should show thinking")
	}
}

func TestRenderAssistantMessage_Streaming(t *testing.T) {
	msg := message.AssistantMessage("partial response")
	got := RenderAssistantMessage(msg, AssistantBlockOptions{IsStreaming: true})
	if !strings.Contains(got, "▌") {
		t.Error("streaming should show cursor")
	}
}

func TestIsEmptyText(t *testing.T) {
	if !isEmptyText("") {
		t.Error("empty should be empty")
	}
	if !isEmptyText("  \n  ") {
		t.Error("whitespace should be empty")
	}
	if !isEmptyText("[no response]") {
		t.Error("no response should be empty")
	}
	if isEmptyText("hello") {
		t.Error("hello should not be empty")
	}
}

func TestIsAPIError(t *testing.T) {
	if !isAPIError("API Error: timeout") {
		t.Error("API Error prefix should match")
	}
	if !isAPIError("Invalid API key provided") {
		t.Error("invalid key should match")
	}
	if isAPIError("normal text") {
		t.Error("normal text should not match")
	}
}

func TestIsRateLimitError(t *testing.T) {
	if !isRateLimitError("Rate limit exceeded") {
		t.Error("should detect rate limit")
	}
	if isRateLimitError("normal text") {
		t.Error("normal should not match")
	}
}
