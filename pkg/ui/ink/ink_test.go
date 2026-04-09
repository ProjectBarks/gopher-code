package ink

import (
	"strings"
	"testing"
)

func TestLink(t *testing.T) {
	got := Link("https://example.com", "click here")
	// Should contain either the hyperlink or the text
	if !strings.Contains(got, "click here") {
		t.Error("should contain text")
	}
}

func TestLink_EmptyText(t *testing.T) {
	got := Link("https://example.com", "")
	if !strings.Contains(got, "example.com") {
		t.Error("empty text should show URL")
	}
}

func TestLinkWithFallback(t *testing.T) {
	got := LinkWithFallback("https://example.com", "", "see docs")
	// On non-hyperlink terminals, should use fallback
	if got == "" {
		t.Error("should not be empty")
	}
}

func TestRenderButton(t *testing.T) {
	got := RenderButton("OK", ButtonState{Focused: true})
	if !strings.Contains(got, "OK") {
		t.Error("should contain label")
	}
}

func TestRenderTextButton(t *testing.T) {
	got := RenderTextButton("Cancel", false)
	if !strings.Contains(got, "Cancel") {
		t.Error("should contain label")
	}
	focused := RenderTextButton("Cancel", true)
	if focused == got {
		t.Error("focused should look different")
	}
}

func TestSpacer(t *testing.T) {
	got := Spacer(5)
	if got != "     " {
		t.Errorf("Spacer(5) = %q", got)
	}
	if Spacer(0) != "" {
		t.Error("Spacer(0) should be empty")
	}
}

func TestNewline(t *testing.T) {
	if Newline(1) != "\n" {
		t.Error("Newline(1) should be one newline")
	}
	if Newline(3) != "\n\n\n" {
		t.Error("Newline(3) should be three newlines")
	}
	if Newline(0) != "\n" {
		t.Error("Newline(0) should default to 1")
	}
}

func TestRawAnsi(t *testing.T) {
	raw := "\x1b[31mred\x1b[0m"
	if RawAnsi(raw) != raw {
		t.Error("RawAnsi should pass through")
	}
}

func TestAlternateScreen(t *testing.T) {
	enter := EnterAlternateScreen()
	exit := ExitAlternateScreen()
	if !strings.Contains(enter, "1049h") {
		t.Error("enter should contain 1049h")
	}
	if !strings.Contains(exit, "1049l") {
		t.Error("exit should contain 1049l")
	}
}

func TestHBox(t *testing.T) {
	got := HBox("a", "b", "c")
	if !strings.Contains(got, "a") || !strings.Contains(got, "c") {
		t.Error("HBox should contain all parts")
	}
}

func TestVBox(t *testing.T) {
	got := VBox("line1", "line2")
	if !strings.Contains(got, "line1") || !strings.Contains(got, "line2") {
		t.Error("VBox should contain all parts")
	}
}

func TestBordered(t *testing.T) {
	got := Bordered("content", "blue")
	if !strings.Contains(got, "content") {
		t.Error("should contain content")
	}
}

func TestBold(t *testing.T) {
	got := Bold("important")
	if got == "important" {
		t.Error("Bold should add styling")
	}
	if !strings.Contains(got, "important") {
		t.Error("should contain text")
	}
}

func TestDim(t *testing.T) {
	got := Dim("subtle")
	if !strings.Contains(got, "subtle") {
		t.Error("should contain text")
	}
}

func TestColored(t *testing.T) {
	got := Colored("red text", "9")
	if !strings.Contains(got, "red text") {
		t.Error("should contain text")
	}
}

func TestThemedText(t *testing.T) {
	got := ThemedText("hello", "primary")
	if !strings.Contains(got, "hello") {
		t.Error("should contain text")
	}
}

func TestNoSelect(t *testing.T) {
	if NoSelect("text") != "text" {
		t.Error("NoSelect should be a no-op")
	}
}

func TestFigures(t *testing.T) {
	if Tick != "✓" {
		t.Error("wrong tick")
	}
	if Cross != "✗" {
		t.Error("wrong cross")
	}
	if Pointer != "❯" {
		t.Error("wrong pointer")
	}
}

func TestStatusFigure(t *testing.T) {
	if StatusFigure("ok") != Tick {
		t.Error("ok should be tick")
	}
	if StatusFigure("error") != Cross {
		t.Error("error should be cross")
	}
	if StatusFigure("running") != Play {
		t.Error("running should be play")
	}
	if StatusFigure("unknown") != Bullet {
		t.Error("unknown should be bullet")
	}
}

func TestFormatKeyValue(t *testing.T) {
	got := FormatKeyValue("Name", "Claude")
	if !strings.Contains(got, "Name") || !strings.Contains(got, "Claude") {
		t.Error("should contain key and value")
	}
}

func TestPadded(t *testing.T) {
	got := Padded("content", 1, 2, 1, 2)
	if !strings.Contains(got, "content") {
		t.Error("should contain content")
	}
}
