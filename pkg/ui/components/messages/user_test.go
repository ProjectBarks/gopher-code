package messages

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

func TestRenderUserBlock_Text(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: "Hello Claude"}
	got := RenderUserBlock(block, UserBlockOptions{})
	if !strings.Contains(got, "Hello Claude") {
		t.Errorf("should contain text: %q", got)
	}
	if !strings.Contains(got, "❯") {
		t.Error("should have prompt indicator")
	}
}

func TestRenderUserBlock_TextEmpty(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: ""}
	got := RenderUserBlock(block, UserBlockOptions{})
	if got != "" {
		t.Error("empty should return empty")
	}
}

func TestRenderUserBlock_TextContinuation(t *testing.T) {
	block := message.ContentBlock{Type: message.ContentText, Text: "more text"}
	got := RenderUserBlock(block, UserBlockOptions{IsContinuation: true})
	if strings.Contains(got, "❯") {
		t.Error("continuation should not show prompt")
	}
}

func TestRenderUserBlock_TextTruncation(t *testing.T) {
	longText := strings.Repeat("x", 20000)
	block := message.ContentBlock{Type: message.ContentText, Text: longText}
	got := RenderUserBlock(block, UserBlockOptions{})
	if !strings.Contains(got, "omitted") {
		t.Error("long text should be truncated with omission notice")
	}
}

func TestRenderUserBlock_ToolResult_Success(t *testing.T) {
	block := message.ContentBlock{
		Type:    message.ContentToolResult,
		Content: "file contents here",
	}
	got := RenderUserBlock(block, UserBlockOptions{ToolName: "Read"})
	if !strings.Contains(got, "file contents here") {
		t.Error("should show result content")
	}
	if !strings.Contains(got, ResponseConnector) {
		t.Error("should have connector")
	}
}

func TestRenderUserBlock_ToolResult_Empty(t *testing.T) {
	block := message.ContentBlock{
		Type:    message.ContentToolResult,
		Content: "",
	}
	got := RenderUserBlock(block, UserBlockOptions{})
	if got != "" {
		t.Error("empty result should return empty")
	}
}

func TestRenderUserBlock_ToolResult_Error(t *testing.T) {
	block := message.ContentBlock{
		Type:    message.ContentToolResult,
		Content: "command not found: foobar",
		IsError: true,
	}
	got := RenderUserBlock(block, UserBlockOptions{})
	if !strings.Contains(got, "Error") {
		t.Error("error should show Error label")
	}
	if !strings.Contains(got, "foobar") {
		t.Error("should show error content")
	}
}

func TestRenderUserBlock_ToolResult_LongTruncated(t *testing.T) {
	longContent := strings.Repeat("long line of output text\n", 100)
	block := message.ContentBlock{
		Type:    message.ContentToolResult,
		Content: longContent,
	}
	got := RenderUserBlock(block, UserBlockOptions{Verbose: false})
	if !strings.Contains(got, "lines") {
		t.Errorf("long result should be truncated: %q", got[:min(200, len(got))])
	}
}

func TestRenderToolReject(t *testing.T) {
	got := RenderToolReject("Bash", "dangerous command")
	if !strings.Contains(got, "Bash") {
		t.Error("should show tool name")
	}
	if !strings.Contains(got, "denied") {
		t.Error("should show denied")
	}
	if !strings.Contains(got, "dangerous command") {
		t.Error("should show reason")
	}
}

func TestRenderToolReject_NoReason(t *testing.T) {
	got := RenderToolReject("Write", "")
	if !strings.Contains(got, "Write") {
		t.Error("should show tool name")
	}
}

func TestRenderToolCancel(t *testing.T) {
	got := RenderToolCancel("Bash")
	if !strings.Contains(got, "cancelled") {
		t.Error("should show cancelled")
	}
	if !strings.Contains(got, "⏸") {
		t.Error("should show pause icon")
	}
}

func TestRenderPlanRejected(t *testing.T) {
	got := RenderPlanRejected()
	if !strings.Contains(got, "rejected") {
		t.Error("should show rejected")
	}
}

func TestRenderUserImage(t *testing.T) {
	got := RenderUserImage("screenshot.png", 1920, 1080)
	if !strings.Contains(got, "screenshot.png") {
		t.Error("should show filename")
	}
	if !strings.Contains(got, "1920x1080") {
		t.Error("should show dimensions")
	}
	if !strings.Contains(got, "🖼") {
		t.Error("should show image icon")
	}
}

func TestRenderUserImage_NoDimensions(t *testing.T) {
	got := RenderUserImage("photo.jpg", 0, 0)
	if !strings.Contains(got, "photo.jpg") {
		t.Error("should show filename")
	}
	if strings.Contains(got, "0x0") {
		t.Error("should not show zero dimensions")
	}
}

func TestRenderCommandOutput(t *testing.T) {
	got := RenderCommandOutput("ls -la", "total 42\ndrwxr-xr-x  5 user")
	if !strings.Contains(got, "$ ls -la") {
		t.Error("should show command")
	}
	if !strings.Contains(got, "total 42") {
		t.Error("should show output")
	}
}

func TestRenderCommandOutput_LongTruncated(t *testing.T) {
	output := strings.Repeat("line\n", 50)
	got := RenderCommandOutput("find .", output)
	if !strings.Contains(got, "more lines") {
		t.Error("long output should be truncated")
	}
}

func TestRenderUserMessage_Full(t *testing.T) {
	msg := message.Message{
		Role: message.RoleUser,
		Content: []message.ContentBlock{
			{Type: message.ContentText, Text: "Fix the bug"},
			{Type: message.ContentToolResult, Content: "File contents", ToolUseID: "t1"},
		},
	}
	got := RenderUserMessage(msg, UserBlockOptions{})
	if !strings.Contains(got, "Fix the bug") {
		t.Error("should render text block")
	}
	if !strings.Contains(got, "File contents") {
		t.Error("should render tool result")
	}
}

func TestTruncateHeadTail(t *testing.T) {
	s := strings.Repeat("x", 10000)
	got := truncateHeadTail(s, 100, 100)
	if !strings.Contains(got, "omitted") {
		t.Error("should contain omission notice")
	}
	if len(got) > 500 {
		t.Errorf("truncated result too long: %d", len(got))
	}
}

func TestTruncateHeadTail_Short(t *testing.T) {
	got := truncateHeadTail("short", 100, 100)
	if got != "short" {
		t.Error("short string should not be truncated")
	}
}
