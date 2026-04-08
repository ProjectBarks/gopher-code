package ui

import (
	"strings"
	"testing"
)

func TestGetPlatform(t *testing.T) {
	p := GetPlatform()
	if p == "" {
		t.Error("platform should not be empty")
	}
	// On macOS CI, should be "macos"
	// On Linux CI, should be "linux" or "wsl"
	// Just verify it doesn't panic and returns a known value
	valid := map[Platform]bool{
		PlatformMacOS: true, PlatformLinux: true, PlatformWindows: true,
		PlatformWSL: true, PlatformUnknown: true,
	}
	if !valid[p] {
		t.Errorf("unexpected platform: %q", p)
	}
}

func TestTerminalWidth(t *testing.T) {
	w := TerminalWidth(80)
	if w <= 0 {
		t.Errorf("width = %d, should be > 0", w)
	}
}

func TestRenderTruncatedContent_Short(t *testing.T) {
	got := RenderTruncatedContent("line1\nline2", 80)
	if got != "line1\nline2" {
		t.Errorf("short content should not be truncated, got %q", got)
	}
}

func TestRenderTruncatedContent_ExactlyMax(t *testing.T) {
	lines := make([]string, MaxLinesToShow)
	for i := range lines {
		lines[i] = "line"
	}
	got := RenderTruncatedContent(strings.Join(lines, "\n"), 80)
	if strings.Contains(got, "...") {
		t.Error("exactly MaxLinesToShow should not truncate")
	}
}

func TestRenderTruncatedContent_Truncated(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "some content here"
	}
	got := RenderTruncatedContent(strings.Join(lines, "\n"), 80)
	if !strings.Contains(got, "... +") {
		t.Errorf("should show truncation indicator, got %q", got)
	}
}

func TestRenderTruncatedContent_OnePlusLine(t *testing.T) {
	// 4 lines = MaxLinesToShow + 1 → should show all (no "... +1 line")
	lines := make([]string, MaxLinesToShow+1)
	for i := range lines {
		lines[i] = "line"
	}
	got := RenderTruncatedContent(strings.Join(lines, "\n"), 80)
	if strings.Contains(got, "...") {
		t.Error("MaxLinesToShow+1 should show all lines, not truncate")
	}
}

func TestRenderTruncatedContent_Empty(t *testing.T) {
	if got := RenderTruncatedContent("", 80); got != "" {
		t.Errorf("empty should return empty, got %q", got)
	}
	if got := RenderTruncatedContent("  \n  ", 80); got != "" {
		t.Errorf("whitespace-only should return empty, got %q", got)
	}
}

func TestVisibleLen(t *testing.T) {
	if n := visibleLen("hello"); n != 5 {
		t.Errorf("plain text len = %d, want 5", n)
	}
	// ANSI colored text
	if n := visibleLen("\033[31mred\033[0m"); n != 3 {
		t.Errorf("ANSI text len = %d, want 3", n)
	}
}

func TestIsColorTerminal(t *testing.T) {
	// Just verify it doesn't panic
	_ = IsColorTerminal()
}
