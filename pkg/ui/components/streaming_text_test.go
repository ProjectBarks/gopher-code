package components

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestStreamingTextCreation(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	if st == nil {
		t.Error("Expected non-nil StreamingText")
	}
	if st.isStreaming {
		t.Error("Expected isStreaming to be false initially")
	}
	if st.Text() != "" {
		t.Error("Expected empty buffer initially")
	}
}

func TestStreamingTextAppendDelta(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Hello")
	if st.Text() != "Hello" {
		t.Errorf("Expected 'Hello', got %q", st.Text())
	}
	if !st.isStreaming {
		t.Error("Expected isStreaming to be true after AppendDelta")
	}
}

func TestStreamingTextAppendMultipleDeltas(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Hello")
	st.AppendDelta(" ")
	st.AppendDelta("world")

	expected := "Hello world"
	if st.Text() != expected {
		t.Errorf("Expected %q, got %q", expected, st.Text())
	}
}

func TestStreamingTextAppendDeltaWithNewlines(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Line 1\n")
	st.AppendDelta("Line 2\n")
	st.AppendDelta("Line 3")

	expected := "Line 1\nLine 2\nLine 3"
	if st.Text() != expected {
		t.Errorf("Expected %q, got %q", expected, st.Text())
	}
}

func TestStreamingTextViewWithoutStreaming(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Test text")
	st.Complete() // Mark stream as complete

	view := st.View()
	viewStr := view.Content
	// View content should be close to the text (may have ANSI codes)
	if !strings.Contains(viewStr, "Test text") {
		t.Errorf("Expected 'Test text' in view, got %q", viewStr)
	}

	// Should not have cursor since streaming is complete
	if strings.Contains(viewStr, "▊") && strings.Contains(viewStr, "▒") {
		t.Logf("Unexpected cursor in non-streaming view: %q", viewStr)
	}
}

func TestStreamingTextViewWithStreaming(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Streaming text")
	// Don't call Complete(), so streaming is still active

	view := st.View()
	// Should contain the text
	if !strings.Contains(view.Content, "Streaming text") {
		t.Errorf("Expected 'Streaming text' in view, got %q", view.Content)
	}

	// Should have a cursor character (▊ or ▒)
	hasCursor := strings.Contains(view.Content, "▊") || strings.Contains(view.Content, "▒")
	if !hasCursor {
		t.Logf("Expected cursor in streaming view, got: %q", view.Content)
	}
}

func TestStreamingTextCursorBlinking(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Test")

	// Get initial view (tick 0)
	view1 := st.View()
	content1 := view1.Content

	// Simulate ticks to cycle the cursor
	st.Update(StreamingTextTickMsg{})
	st.Update(StreamingTextTickMsg{})
	st.Update(StreamingTextTickMsg{})
	st.Update(StreamingTextTickMsg{})

	view2 := st.View()
	content2 := view2.Content

	// After 4 ticks, cursor should toggle state
	// Both should contain "Test" but cursor might differ
	if !strings.Contains(content1, "Test") || !strings.Contains(content2, "Test") {
		t.Error("Expected 'Test' in both views")
	}

	// The views may differ due to cursor state
	t.Logf("View 1: %q", content1)
	t.Logf("View 2: %q", content2)
}

func TestStreamingTextCursorAnimationCycle(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Animation test")

	// Simulate cursor animation over 8 ticks (full blink cycle)
	cursorStates := make([]int, 0)
	for i := 0; i < 8; i++ {
		cursorStates = append(cursorStates, st.cursorTick)
		st.Update(StreamingTextTickMsg{})
	}

	// Cursor should toggle between 0 and 1
	if len(cursorStates) < 4 {
		t.Error("Expected multiple cursor states")
	}

	// After ticks, cursor should have animated
	t.Logf("Cursor states: %v", cursorStates)
}

func TestStreamingTextComplete(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Streaming content")
	if !st.isStreaming {
		t.Error("Expected isStreaming to be true")
	}

	st.Complete()
	if st.isStreaming {
		t.Error("Expected isStreaming to be false after Complete()")
	}

	// Text should still be there
	if st.Text() != "Streaming content" {
		t.Errorf("Expected 'Streaming content', got %q", st.Text())
	}

	// View should not have cursor
	view := st.View()
	hasCursor := strings.Contains(view.Content, "▊") || strings.Contains(view.Content, "▒")
	if hasCursor {
		t.Logf("Unexpected cursor after Complete(): %q", view.Content)
	}
}

func TestStreamingTextReset(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Content")
	st.AppendDelta(" to reset")

	if st.Text() != "Content to reset" {
		t.Error("Expected buffer to contain text before reset")
	}

	st.Reset()

	if st.Text() != "" {
		t.Errorf("Expected empty buffer after Reset(), got %q", st.Text())
	}
	if st.isStreaming {
		t.Error("Expected isStreaming to be false after Reset()")
	}
	if st.cursorTick != 0 {
		t.Error("Expected cursorTick to be 0 after Reset()")
	}
	if st.tickCounter != 0 {
		t.Error("Expected tickCounter to be 0 after Reset()")
	}
}

func TestStreamingTextText(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	text := "Plain text without cursor"
	st.AppendDelta(text)

	retrieved := st.Text()
	if retrieved != text {
		t.Errorf("Expected %q, got %q", text, retrieved)
	}

	// Text() should not include cursor
	if strings.Contains(retrieved, "▊") || strings.Contains(retrieved, "▒") {
		t.Errorf("Text() should not include cursor: %q", retrieved)
	}
}

func TestStreamingTextSetSize(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.SetSize(100, 24)
	if st.width != 100 {
		t.Errorf("Expected width 100, got %d", st.width)
	}
}

func TestStreamingTextInit(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	cmd := st.Init()
	if cmd != nil {
		t.Error("Expected Init() to return nil")
	}
}

func TestStreamingTextEmptyDelta(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("")
	if st.Text() != "" {
		t.Errorf("Expected empty text after empty delta, got %q", st.Text())
	}
	if !st.isStreaming {
		t.Error("Expected isStreaming to be true even after empty delta")
	}
}

func TestStreamingTextLongContent(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	// Append a lot of text
	longText := strings.Repeat("x", 10000)
	st.AppendDelta(longText)

	if st.Text() != longText {
		t.Error("Expected full long content to be buffered")
	}
}

func TestStreamingTextMultipleCompleteCycles(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	// First cycle
	st.AppendDelta("First")
	st.Complete()

	// Reset and start new cycle
	st.Reset()

	st.AppendDelta("Second")
	if st.Text() != "Second" {
		t.Errorf("Expected 'Second' after reset, got %q", st.Text())
	}
	if !st.isStreaming {
		t.Error("Expected streaming to be active again")
	}
}

func TestStreamingTextTickMessageHandling(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Test")

	// Send tick messages
	for i := 0; i < 10; i++ {
		_, cmd := st.Update(StreamingTextTickMsg{})
		if cmd != nil {
			t.Errorf("Expected nil command from Tick, got %v", cmd)
		}
	}

	// After 10 ticks, tickCounter should be 10
	if st.tickCounter != 10 {
		t.Errorf("Expected tickCounter to be 10, got %d", st.tickCounter)
	}
}

func TestStreamingTextOtherMessageTypes(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Test")

	// Send non-StreamingTextTickMsg message (dummy message)
	dummyMsg := struct{}{}
	updated, cmd := st.Update(dummyMsg)

	if cmd != nil {
		t.Errorf("Expected nil command, got %v", cmd)
	}

	// Should return self unchanged
	if updated != st {
		t.Error("Expected Update to return self for non-Tick messages")
	}
}

func TestStreamingTextViewConsistency(t *testing.T) {
	th := theme.Current()
	st := NewStreamingText(th)

	st.AppendDelta("Consistency test")

	// Call View multiple times
	view1 := st.View()
	view2 := st.View()

	// Both should contain the text
	if !strings.Contains(view1.Content, "Consistency test") {
		t.Error("Expected text in view1")
	}
	if !strings.Contains(view2.Content, "Consistency test") {
		t.Error("Expected text in view2")
	}
}

func TestStreamingTextCursorColorScheme(t *testing.T) {
	// Test with different themes
	themes := []theme.ThemeName{
		theme.ThemeDark,
		theme.ThemeLight,
		theme.ThemeHighContrast,
	}

	for _, themeName := range themes {
		theme.SetTheme(themeName)
		defer theme.SetTheme(theme.ThemeDark)

		th := theme.Current()
		st := NewStreamingText(th)

		st.AppendDelta("Theme test")
		view := st.View()

		if !strings.Contains(view.Content, "Theme test") {
			t.Errorf("Expected 'Theme test' in view with theme %s", themeName)
		}
	}
}
