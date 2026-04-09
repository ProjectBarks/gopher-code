package layout

import (
	"strings"
	"testing"
)

func TestFullscreenLayout_BasicRender(t *testing.T) {
	l := &FullscreenLayout{
		Width:         80,
		Height:        24,
		ScrollContent: "message 1\nmessage 2\nmessage 3",
		BottomContent: "> prompt here",
	}
	out := l.Render()
	if !strings.Contains(out, "message 1") {
		t.Error("should contain scroll content")
	}
	if !strings.Contains(out, "prompt here") {
		t.Error("should contain bottom content")
	}
}

func TestFullscreenLayout_ZeroSize(t *testing.T) {
	l := &FullscreenLayout{Width: 0, Height: 0}
	if l.Render() != "" {
		t.Error("zero size should produce empty output")
	}
}

func TestFullscreenLayout_ScrollTruncation(t *testing.T) {
	// Create more lines than fit
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "line")
	}

	l := &FullscreenLayout{
		Width:         80,
		Height:        10,
		ScrollContent: strings.Join(lines, "\n"),
		BottomContent: "prompt",
	}
	out := l.Render()
	outLines := strings.Split(out, "\n")
	// Should not exceed terminal height
	if len(outLines) > 12 { // some tolerance for padding
		t.Errorf("output has %d lines, should fit in ~10", len(outLines))
	}
}

func TestFullscreenLayout_WithStickyHeader(t *testing.T) {
	l := &FullscreenLayout{
		Width:         80,
		Height:        24,
		StickyHeader:  "=== Session abc ===",
		ScrollContent: "message",
		BottomContent: "prompt",
	}
	out := l.Render()
	if !strings.Contains(out, "Session abc") {
		t.Error("should show sticky header")
	}
}

func TestFullscreenLayout_WithStatusBar(t *testing.T) {
	l := &FullscreenLayout{
		Width:         80,
		Height:        24,
		ScrollContent: "content",
		BottomContent: "prompt",
		StatusBar:     "tokens: 1.2K | model: sonnet",
	}
	out := l.Render()
	if !strings.Contains(out, "tokens: 1.2K") {
		t.Error("should show status bar")
	}
}

func TestFullscreenLayout_WithNewMessagePill(t *testing.T) {
	l := &FullscreenLayout{
		Width:          80,
		Height:         24,
		ScrollContent:  "content",
		BottomContent:  "prompt",
		NewMessagePill: "↓ 3 new messages",
	}
	out := l.Render()
	if !strings.Contains(out, "3 new messages") {
		t.Error("should show pill")
	}
}

func TestFullscreenLayout_ModalMode(t *testing.T) {
	l := &FullscreenLayout{
		Width:         80,
		Height:        24,
		ScrollContent: "message 1\nmessage 2\nmessage 3\nmessage 4\nmessage 5",
		ModalContent:  "Modal dialog content\nOption 1\nOption 2",
	}
	out := l.Render()
	if !strings.Contains(out, "Modal dialog content") {
		t.Error("should show modal content")
	}
	// Should show divider
	if !strings.Contains(out, "▔") {
		t.Error("should show divider between peek and modal")
	}
}

func TestFullscreenLayout_ModalPeek(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "scroll line"
	}

	l := &FullscreenLayout{
		Width:         80,
		Height:        24,
		ScrollContent: strings.Join(lines, "\n"),
		ModalContent:  "modal",
	}
	out := l.Render()
	// Should show some scroll content (peek) above divider
	parts := strings.SplitN(out, "▔", 2)
	if len(parts) < 2 {
		t.Fatal("should have divider separating peek from modal")
	}
	peekPart := parts[0]
	if !strings.Contains(peekPart, "scroll line") {
		t.Error("peek should show scroll content")
	}
}

func TestFullscreenLayout_WithOverlay(t *testing.T) {
	l := &FullscreenLayout{
		Width:          80,
		Height:         24,
		ScrollContent:  "messages",
		OverlayContent: "permission request overlay",
		BottomContent:  "prompt",
	}
	out := l.Render()
	if !strings.Contains(out, "permission request overlay") {
		t.Error("should show overlay content in scroll area")
	}
}

func TestScrollRegionHeight(t *testing.T) {
	l := &FullscreenLayout{
		Width:         80,
		Height:        24,
		BottomContent: "line1\nline2\nline3", // 3 lines
	}
	h := l.ScrollRegionHeight()
	if h != 21 { // 24 - 3
		t.Errorf("scroll height = %d, want 21", h)
	}
}

func TestScrollRegionHeight_WithHeader(t *testing.T) {
	l := &FullscreenLayout{
		Width:         80,
		Height:        24,
		StickyHeader:  "header",
		BottomContent: "prompt",
	}
	h := l.ScrollRegionHeight()
	if h != 22 { // 24 - 1 header - 1 bottom
		t.Errorf("scroll height = %d, want 22", h)
	}
}

func TestScrollRegionHeight_MinimumOne(t *testing.T) {
	l := &FullscreenLayout{
		Width:         80,
		Height:        3,
		BottomContent: "line1\nline2\nline3\nline4\nline5",
	}
	h := l.ScrollRegionHeight()
	if h < 1 {
		t.Errorf("minimum scroll height should be 1, got %d", h)
	}
}

func TestModalRegionHeight(t *testing.T) {
	l := &FullscreenLayout{
		Width:  80,
		Height: 24,
	}
	h := l.ModalRegionHeight()
	if h < 3 {
		t.Errorf("modal height should be at least 3, got %d", h)
	}
	// Should be roughly: 24 - 2 peek - 0 header - 2 (divider+status) = 20
	if h != 20 {
		t.Errorf("modal height = %d, want 20", h)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"one", 1},
		{"a\nb", 2},
		{"a\nb\nc", 3},
	}
	for _, tt := range tests {
		if got := countLines(tt.s); got != tt.want {
			t.Errorf("countLines(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestSplitLines(t *testing.T) {
	if splitLines("") != nil {
		t.Error("empty string should return nil")
	}
	lines := splitLines("a\nb\nc")
	if len(lines) != 3 || lines[0] != "a" || lines[2] != "c" {
		t.Errorf("splitLines = %v", lines)
	}
}

func TestModalTranscriptPeek(t *testing.T) {
	if ModalTranscriptPeek != 2 {
		t.Errorf("ModalTranscriptPeek = %d, want 2", ModalTranscriptPeek)
	}
}
