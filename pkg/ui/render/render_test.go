package render

import (
	"strings"
	"testing"
	"time"
)

func TestFrameRate_ShouldRender(t *testing.T) {
	fr := NewFrameRate(60)
	if !fr.ShouldRender() {
		t.Error("first render should always be allowed")
	}
	// Immediate second render should be throttled
	if fr.ShouldRender() {
		t.Error("immediate second render should be throttled")
	}
	// After sleeping past interval, should render
	time.Sleep(20 * time.Millisecond)
	if !fr.ShouldRender() {
		t.Error("should render after interval")
	}
}

func TestFrameRate_MarkDirty(t *testing.T) {
	fr := NewFrameRate(60)
	if fr.IsDirty() {
		t.Error("should not be dirty initially")
	}
	fr.MarkDirty()
	if !fr.IsDirty() {
		t.Error("should be dirty after MarkDirty")
	}
}

func TestCompose(t *testing.T) {
	got := Compose("header", "", "content", "", "footer")
	if !strings.Contains(got, "header") {
		t.Error("should contain header")
	}
	if !strings.Contains(got, "content") {
		t.Error("should contain content")
	}
	if !strings.Contains(got, "footer") {
		t.Error("should contain footer")
	}
	// Empty sections should be filtered
	lines := strings.Split(got, "\n")
	for _, line := range lines {
		if line == "" {
			t.Error("empty sections should be filtered")
			break
		}
	}
}

func TestCompose_AllEmpty(t *testing.T) {
	got := Compose("", "", "")
	if got != "" {
		t.Errorf("all empty should produce empty, got %q", got)
	}
}

func TestComposeWithDivider(t *testing.T) {
	got := ComposeWithDivider(40, "top", "bottom")
	if !strings.Contains(got, "top") {
		t.Error("should contain top")
	}
	if !strings.Contains(got, "bottom") {
		t.Error("should contain bottom")
	}
	if !strings.Contains(got, "─") {
		t.Error("should contain divider")
	}
}

func TestRenderBorder(t *testing.T) {
	got := RenderBorder("hello\nworld", 20, DefaultBorder, "7")
	if !strings.Contains(got, "╭") {
		t.Error("should contain top-left corner")
	}
	if !strings.Contains(got, "╯") {
		t.Error("should contain bottom-right corner")
	}
	if !strings.Contains(got, "hello") {
		t.Error("should contain content")
	}
}

func TestRenderBorder_Sharp(t *testing.T) {
	got := RenderBorder("text", 15, SharpBorder, "7")
	if !strings.Contains(got, "┌") {
		t.Error("should contain sharp top-left")
	}
}

func TestRenderDivider(t *testing.T) {
	got := RenderDivider(10, "")
	if !strings.Contains(got, "─") {
		t.Error("should contain default divider char")
	}
}

func TestRenderDivider_Custom(t *testing.T) {
	got := RenderDivider(5, "▔")
	if !strings.Contains(got, "▔") {
		t.Error("should contain custom divider char")
	}
}

func TestTruncateView(t *testing.T) {
	view := "line1\nline2\nline3\nline4\nline5"
	got := TruncateView(view, 3)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("should have 3 lines, got %d", len(lines))
	}
}

func TestTruncateView_NoTruncation(t *testing.T) {
	view := "a\nb"
	got := TruncateView(view, 10)
	if got != view {
		t.Error("should not truncate when within limit")
	}
}

func TestPadView(t *testing.T) {
	got := PadView("a\nb", 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Errorf("should have 5 lines, got %d", len(lines))
	}
}

func TestPadView_Truncate(t *testing.T) {
	got := PadView("a\nb\nc\nd\ne", 3)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("should truncate to 3 lines, got %d", len(lines))
	}
}

func TestClearAndRender(t *testing.T) {
	got := ClearAndRender("hello")
	if !strings.HasPrefix(got, "\x1b[2J") {
		t.Error("should start with clear screen")
	}
	if !strings.Contains(got, "hello") {
		t.Error("should contain content")
	}
}

func TestDefaultBorder(t *testing.T) {
	if DefaultBorder.TopLeft != "╭" {
		t.Error("wrong top-left")
	}
	if DefaultBorder.BottomRight != "╯" {
		t.Error("wrong bottom-right")
	}
}

func TestHeavyDivider(t *testing.T) {
	if HeavyDivider != "▔" {
		t.Error("wrong heavy divider")
	}
}
