package text

import (
	"strings"
	"testing"
)

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"hello", 5},
		{"", 0},
		{"abc", 3},
		{"\x1b[31mred\x1b[0m", 3},                   // ANSI stripped
		{"日本語", 6},                                    // CJK = width 2 each
		{"a日b", 4},                                    // mixed
		{"\x1b[1;32mhi\x1b[0m", 2},                   // bold green
		{"tab\there", 7},                               // tab = 0 width (control)
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := DisplayWidth(tt.s)
			if got != tt.want {
				t.Errorf("DisplayWidth(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

func TestRuneWidth(t *testing.T) {
	if RuneWidth('a') != 1 {
		t.Error("ASCII should be width 1")
	}
	if RuneWidth('日') != 2 {
		t.Error("CJK should be width 2")
	}
	if RuneWidth('\n') != 0 {
		t.Error("control char should be width 0")
	}
	if RuneWidth('Ａ') != 2 { // fullwidth A
		t.Error("fullwidth should be width 2")
	}
}

func TestTruncateAnsi_End(t *testing.T) {
	got := TruncateAnsi("hello world", 8, TruncateEnd)
	if got != "hello w…" {
		t.Errorf("got %q", got)
	}
}

func TestTruncateAnsi_Start(t *testing.T) {
	got := TruncateAnsi("hello world", 8, TruncateStart)
	if !strings.HasPrefix(got, "…") {
		t.Errorf("should start with ellipsis: %q", got)
	}
	if DisplayWidth(got) > 8 {
		t.Errorf("should fit in 8 columns, got %d: %q", DisplayWidth(got), got)
	}
}

func TestTruncateAnsi_Middle(t *testing.T) {
	got := TruncateAnsi("hello world", 8, TruncateMiddle)
	if !strings.Contains(got, "…") {
		t.Error("should contain ellipsis")
	}
	if DisplayWidth(got) > 8 {
		t.Errorf("should fit in 8 columns: %q", got)
	}
}

func TestTruncateAnsi_NoTruncation(t *testing.T) {
	got := TruncateAnsi("hi", 10, TruncateEnd)
	if got != "hi" {
		t.Errorf("short string should not be truncated: %q", got)
	}
}

func TestTruncateAnsi_TinyWidth(t *testing.T) {
	if TruncateAnsi("hello", 0, TruncateEnd) != "" {
		t.Error("width 0 should return empty")
	}
	if TruncateAnsi("hello", 1, TruncateEnd) != "…" {
		t.Error("width 1 should return just ellipsis")
	}
}

func TestWrapText_NoWrap(t *testing.T) {
	got := WrapText("short", 80)
	if got != "short" {
		t.Errorf("got %q", got)
	}
}

func TestWrapText_Simple(t *testing.T) {
	got := WrapText("hello world foo bar", 10)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("should wrap into multiple lines: %q", got)
	}
	for _, line := range lines {
		if DisplayWidth(line) > 10 {
			t.Errorf("line too wide: %q (%d)", line, DisplayWidth(line))
		}
	}
}

func TestWrapText_PreservesNewlines(t *testing.T) {
	got := WrapText("line1\nline2", 80)
	if got != "line1\nline2" {
		t.Errorf("should preserve newlines: %q", got)
	}
}

func TestWrapText_LongWord(t *testing.T) {
	got := WrapText("abcdefghijklmnop", 5)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Error("long word should hard-break")
	}
}

func TestWrapText_ZeroWidth(t *testing.T) {
	got := WrapText("hello", 0)
	if got != "hello" {
		t.Errorf("zero width should return original: %q", got)
	}
}

func TestExpandTabs(t *testing.T) {
	got := ExpandTabs("a\tb", 4)
	if got != "a   b" {
		t.Errorf("got %q", got)
	}

	got = ExpandTabs("ab\tc", 4)
	if got != "ab  c" {
		t.Errorf("got %q", got)
	}
}

func TestExpandTabs_NoTabs(t *testing.T) {
	got := ExpandTabs("hello", 4)
	if got != "hello" {
		t.Error("no tabs should pass through")
	}
}

func TestExpandTabs_MultiLine(t *testing.T) {
	got := ExpandTabs("a\tb\nc\td", 4)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// Both lines should expand tabs independently
	if !strings.Contains(lines[0], "   ") {
		t.Errorf("line 1 tabs not expanded: %q", lines[0])
	}
}

func TestPadRight(t *testing.T) {
	got := PadRight("hi", 5)
	if got != "hi   " {
		t.Errorf("got %q", got)
	}
	got = PadRight("hello world", 5)
	if got != "hello world" {
		t.Error("longer string should not be padded")
	}
}

func TestPadLeft(t *testing.T) {
	got := PadLeft("hi", 5)
	if got != "   hi" {
		t.Errorf("got %q", got)
	}
}

func TestCenter(t *testing.T) {
	got := Center("hi", 6)
	if got != "  hi  " {
		t.Errorf("got %q", got)
	}
	got = Center("hi", 7)
	if got != "  hi   " {
		t.Errorf("odd padding: got %q", got)
	}
}

func TestIsWhitespace(t *testing.T) {
	if !IsWhitespace("") {
		t.Error("empty should be whitespace")
	}
	if !IsWhitespace("  \t\n") {
		t.Error("spaces/tabs/newlines should be whitespace")
	}
	if IsWhitespace("a") {
		t.Error("'a' should not be whitespace")
	}
}

func TestVisibleLength(t *testing.T) {
	if VisibleLength("hello") != 5 {
		t.Error("plain text")
	}
	if VisibleLength("\x1b[31mhi\x1b[0m") != 2 {
		t.Error("ANSI should be stripped")
	}
}
