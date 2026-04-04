package components

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// stripANSI removes all ANSI escape sequences from a string.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func getTestBubble() *MessageBubble {
	return NewMessageBubble(theme.Current(), 80)
}

func TestMessageBubbleCreation(t *testing.T) {
	mb := getTestBubble()
	if mb == nil {
		t.Fatal("MessageBubble should not be nil")
	}
	if mb.width != 80 {
		t.Errorf("Expected width 80, got %d", mb.width)
	}
}

func TestMessageBubbleNilMessage(t *testing.T) {
	mb := getTestBubble()
	result := mb.Render(nil)
	if result != "" {
		t.Errorf("Expected empty string for nil message, got %q", result)
	}
}

func TestMessageBubbleUserTextMessage(t *testing.T) {
	mb := getTestBubble()
	msg := message.UserMessage("Hello, world!")
	result := mb.Render(&msg)

	if result == "" {
		t.Error("User message should produce output")
	}
	if !strings.Contains(result, "Hello, world!") {
		t.Errorf("Expected user text in output, got %q", result)
	}
}

func TestMessageBubbleUserMultilineMessage(t *testing.T) {
	mb := getTestBubble()
	msg := message.UserMessage("Line one\nLine two\nLine three")
	result := mb.Render(&msg)

	if result == "" {
		t.Error("Multi-line user message should produce output")
	}
	// Should contain all lines
	if !strings.Contains(result, "Line one") || !strings.Contains(result, "Line two") {
		t.Errorf("Expected all lines in output, got %q", result)
	}
}

func TestMessageBubbleAssistantTextMessage(t *testing.T) {
	mb := getTestBubble()
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Hello! I can help you with that."},
		},
	}
	result := mb.Render(&msg)

	if result == "" {
		t.Error("Assistant message should produce output")
	}
	plain := stripANSI(result)
	if !strings.Contains(plain, "Hello! I can help you with that.") {
		t.Errorf("Expected assistant text in output, got %q", plain)
	}
}

func TestMessageBubbleAssistantToolUseBlock(t *testing.T) {
	mb := getTestBubble()
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Let me check that file."},
			{Type: message.ContentToolUse, Name: "bash", ID: "tool-1", Input: json.RawMessage(`{"command":"ls"}`)},
		},
	}
	result := mb.Render(&msg)
	plain := stripANSI(result)

	if !strings.Contains(plain, "bash") {
		t.Errorf("Expected tool name 'bash' in output, got %q", plain)
	}
	if !strings.Contains(plain, "Let me check that file.") {
		t.Errorf("Expected text block in output, got %q", plain)
	}
}

func TestMessageBubbleToolResultSuccess(t *testing.T) {
	mb := getTestBubble()
	block := message.ContentBlock{
		Type:    message.ContentToolResult,
		Content: "file1.go\nfile2.go",
		IsError: false,
	}
	result := mb.RenderContent(block)

	if !strings.Contains(result, "file1.go") {
		t.Errorf("Expected result content in output, got %q", result)
	}
	if !strings.Contains(result, "⎿") {
		t.Errorf("Expected connector character ⎿ in output, got %q", result)
	}
}

func TestMessageBubbleToolResultError(t *testing.T) {
	mb := getTestBubble()
	block := message.ContentBlock{
		Type:    message.ContentToolResult,
		Content: "command not found",
		IsError: true,
	}
	result := mb.RenderContent(block)

	if !strings.Contains(result, "command not found") {
		t.Errorf("Expected error content in output, got %q", result)
	}
}

func TestMessageBubbleToolResultEmpty(t *testing.T) {
	mb := getTestBubble()
	block := message.ContentBlock{
		Type:    message.ContentToolResult,
		Content: "",
		IsError: false,
	}
	result := mb.RenderContent(block)

	if !strings.Contains(result, "no content") {
		t.Errorf("Expected '(no content)' for empty result, got %q", result)
	}
}

func TestMessageBubbleThinkingBlock(t *testing.T) {
	mb := getTestBubble()
	block := message.ContentBlock{
		Type:     message.ContentThinking,
		Thinking: "Let me analyze the code structure...",
	}
	result := mb.RenderContent(block)

	if !strings.Contains(result, "Thinking") {
		t.Errorf("Expected 'Thinking' label in output, got %q", result)
	}
	if !strings.Contains(result, "analyze the code") {
		t.Errorf("Expected thinking text in output, got %q", result)
	}
}

func TestMessageBubbleEmptyTextBlock(t *testing.T) {
	mb := getTestBubble()
	block := message.ContentBlock{Type: message.ContentText, Text: ""}
	result := mb.RenderContent(block)

	if result != "" {
		t.Errorf("Expected empty string for empty text block, got %q", result)
	}
}

func TestMessageBubbleSetWidth(t *testing.T) {
	mb := getTestBubble()
	mb.SetWidth(120)
	if mb.width != 120 {
		t.Errorf("Expected width 120, got %d", mb.width)
	}
}

func TestMessageBubbleWordWrapping(t *testing.T) {
	mb := NewMessageBubble(theme.Current(), 40)
	longText := "This is a very long message that should definitely be word wrapped to fit within the narrow column width"
	msg := message.UserMessage(longText)
	result := mb.Render(&msg)

	// With 40 char width, lines should be shorter than the full text
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Error("Expected word wrapping to produce multiple lines")
	}
}

func TestMessageBubbleMixedContentBlocks(t *testing.T) {
	mb := getTestBubble()
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Here's the result:"},
			{Type: message.ContentToolUse, Name: "read_file", ID: "t1"},
			{Type: message.ContentText, Text: "The file contains important data."},
		},
	}
	result := mb.Render(&msg)
	plain := stripANSI(result)

	if !strings.Contains(plain, "the result") {
		t.Errorf("Expected first text block, got %q", plain)
	}
	if !strings.Contains(plain, "read_file") {
		t.Errorf("Expected tool name, got %q", plain)
	}
	if !strings.Contains(plain, "important data") {
		t.Errorf("Expected second text block, got %q", plain)
	}
}

func TestMessageBubbleLongToolResult(t *testing.T) {
	mb := getTestBubble()
	// Create a very long result that should be truncated
	longResult := strings.Repeat("line of output\n", 100)
	block := message.ContentBlock{
		Type:    message.ContentToolResult,
		Content: longResult,
		IsError: false,
	}
	result := mb.RenderContent(block)

	// Should be truncated
	if !strings.Contains(result, "truncated") {
		t.Error("Expected long result to be truncated")
	}
}

func TestMessageBubbleToolUseWithLargeInput(t *testing.T) {
	mb := getTestBubble()
	// Input larger than 200 bytes should not be displayed
	largeInput := strings.Repeat("x", 300)
	block := message.ContentBlock{
		Type:  message.ContentToolUse,
		Name:  "bash",
		ID:    "t1",
		Input: json.RawMessage(`"` + largeInput + `"`),
	}
	result := mb.RenderContent(block)

	// Should show tool name but not the large input
	if !strings.Contains(result, "bash") {
		t.Error("Expected tool name")
	}
	if strings.Contains(result, largeInput) {
		t.Error("Large input should not be displayed")
	}
}

// Benchmark
func BenchmarkMessageBubbleRenderUser(b *testing.B) {
	mb := getTestBubble()
	msg := message.UserMessage("Hello, world!")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Render(&msg)
	}
}

func BenchmarkMessageBubbleRenderAssistant(b *testing.B) {
	mb := getTestBubble()
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Here's a helpful response with **markdown** and `code`."},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Render(&msg)
	}
}

func BenchmarkMessageBubbleRenderMixed(b *testing.B) {
	mb := getTestBubble()
	msg := message.Message{
		Role: message.RoleAssistant,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Let me check."},
			{Type: message.ContentToolUse, Name: "bash", ID: "t1"},
			{Type: message.ContentText, Text: "Done."},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Render(&msg)
	}
}
