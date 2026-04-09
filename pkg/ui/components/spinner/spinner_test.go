package spinner

import (
	"strings"
	"testing"

)

func TestFrameAt(t *testing.T) {
	// Should cycle through frames without panic
	for i := 0; i < 100; i++ {
		g := FrameAt(i)
		if g == "" {
			t.Fatalf("FrameAt(%d) returned empty string", i)
		}
	}
	// Frame 0 should be the first glyph
	if FrameAt(0) != GlyphFrames[0] {
		t.Errorf("FrameAt(0) = %q, want %q", FrameAt(0), GlyphFrames[0])
	}
}

func TestGlyphFrames(t *testing.T) {
	// Should be forward + reverse = 2 * len(Glyphs)
	if len(GlyphFrames) != 2*len(Glyphs) {
		t.Errorf("GlyphFrames len = %d, want %d", len(GlyphFrames), 2*len(Glyphs))
	}
	// Last element of forward should equal first element of reverse
	if GlyphFrames[len(Glyphs)-1] != GlyphFrames[len(Glyphs)] {
		t.Error("bounce should mirror at midpoint")
	}
}

func TestToolUseSpinner(t *testing.T) {
	s := NewToolUseSpinner("Bash")
	if !s.IsActive() {
		t.Error("should be active initially")
	}
	if s.ToolName != "Bash" {
		t.Errorf("ToolName = %q", s.ToolName)
	}

	v := s.View()
	if !strings.Contains(v, "Running") {
		t.Errorf("active view should contain 'Running': %q", v)
	}
	if !strings.Contains(v, "Bash") {
		t.Errorf("should contain tool name: %q", v)
	}

	s.Tick()
	s.Stop()
	if s.IsActive() {
		t.Error("should not be active after stop")
	}

	v = s.View()
	if strings.Contains(v, "Running") {
		t.Error("stopped view should not contain 'Running'")
	}
	if !strings.Contains(v, "Bash") {
		t.Error("stopped view should still contain tool name")
	}
}

func TestAgentSpinner(t *testing.T) {
	s := NewAgentSpinner("Explore", "blue")
	if !s.IsActive() {
		t.Error("should be active")
	}

	v := s.View()
	if !strings.Contains(v, "Agent") {
		t.Errorf("should contain 'Agent': %q", v)
	}
	if !strings.Contains(v, "Explore") {
		t.Errorf("should contain agent type: %q", v)
	}
	if !strings.Contains(v, "working") {
		t.Errorf("should contain 'working': %q", v)
	}

	s.Tick()
	s.Stop()
	v = s.View()
	if !strings.Contains(v, "finished") {
		t.Errorf("stopped view should contain 'finished': %q", v)
	}
}

func TestAgentSpinner_NoColor(t *testing.T) {
	s := NewAgentSpinner("Plan", "")
	v := s.View()
	if !strings.Contains(v, "Plan") {
		t.Errorf("should contain agent type: %q", v)
	}
}

func TestLoadingState(t *testing.T) {
	s := NewLoadingState("Connecting to server...")
	v := s.View()
	if !strings.Contains(v, "Connecting to server...") {
		t.Errorf("should contain message: %q", v)
	}

	s.Tick()
	v2 := s.View()
	// View should still contain message after tick
	if !strings.Contains(v2, "Connecting to server...") {
		t.Error("message should persist after tick")
	}
}

func TestShimmerText(t *testing.T) {
	text := "hello"
	color := "3"

	result := ShimmerText(text, 0, color)
	if result == text {
		t.Error("shimmer should modify text (add ANSI codes)")
	}
	// Should be longer than original due to ANSI escape codes
	if len(result) <= len(text) {
		t.Error("shimmer result should be longer due to styling")
	}
}

func TestShimmerText_Empty(t *testing.T) {
	result := ShimmerText("", 0, "3")
	if result != "" {
		t.Error("empty input should return empty")
	}
}

func TestShimmerText_CyclesThrough(t *testing.T) {
	text := "ab"
	color := "3"

	r0 := ShimmerText(text, 0, color)
	r1 := ShimmerText(text, 1, color)
	// Different frames should highlight different characters
	if r0 == r1 {
		t.Error("different frames should produce different output")
	}
}

func TestMode_Constants(t *testing.T) {
	if ModeThinking != "thinking" {
		t.Error("wrong")
	}
	if ModeToolUse != "tool_use" {
		t.Error("wrong")
	}
	if ModeAgent != "agent" {
		t.Error("wrong")
	}
	if ModeLoading != "loading" {
		t.Error("wrong")
	}
}
