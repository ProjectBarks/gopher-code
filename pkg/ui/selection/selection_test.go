package selection

import (
	"testing"
)

func TestNew(t *testing.T) {
	s := New()
	if s.HasSelection() {
		t.Error("new should have no selection")
	}
	if s.IsDragging {
		t.Error("should not be dragging")
	}
}

func TestState_StartAndEnd(t *testing.T) {
	s := New()
	s.Start(5, 3, ModeChar)

	if !s.IsDragging {
		t.Error("should be dragging after Start")
	}
	if !s.HasSelection() {
		t.Error("should have selection after Start")
	}
	if s.Anchor.Col != 5 || s.Anchor.Row != 3 {
		t.Errorf("anchor = %+v", s.Anchor)
	}

	s.Update(10, 5)
	if s.Focus.Col != 10 || s.Focus.Row != 5 {
		t.Errorf("focus = %+v", s.Focus)
	}

	s.End()
	if s.IsDragging {
		t.Error("should not be dragging after End")
	}
	if !s.HasSelection() {
		t.Error("selection should persist after End")
	}
}

func TestState_Clear(t *testing.T) {
	s := New()
	s.Start(0, 0, ModeChar)
	s.Update(10, 10)
	s.Clear()

	if s.HasSelection() {
		t.Error("should have no selection after Clear")
	}
}

func TestState_IsEmpty(t *testing.T) {
	s := New()
	if !s.IsEmpty() {
		t.Error("no selection should be empty")
	}

	s.Start(5, 3, ModeChar)
	if !s.IsEmpty() {
		t.Error("anchor == focus should be empty")
	}

	s.Update(6, 3)
	if s.IsEmpty() {
		t.Error("different focus should not be empty")
	}
}

func TestState_Normalized(t *testing.T) {
	s := New()

	// Forward selection
	s.Start(5, 2, ModeChar)
	s.Update(10, 4)
	start, end := s.Normalized()
	if start.Row != 2 || end.Row != 4 {
		t.Errorf("forward: start=%+v end=%+v", start, end)
	}

	// Backward selection
	s.Start(10, 4, ModeChar)
	s.Update(5, 2)
	start, end = s.Normalized()
	if start.Row != 2 || end.Row != 4 {
		t.Errorf("backward: start=%+v end=%+v", start, end)
	}
}

func TestState_ContainsPoint(t *testing.T) {
	s := New()
	s.Start(2, 1, ModeChar)
	s.Update(8, 3)

	// Within range
	if !s.ContainsPoint(5, 2) {
		t.Error("middle row should be contained")
	}
	if !s.ContainsPoint(3, 1) {
		t.Error("start row after start col should be contained")
	}
	if !s.ContainsPoint(5, 3) {
		t.Error("end row before end col should be contained")
	}

	// Outside range
	if s.ContainsPoint(0, 0) {
		t.Error("before selection should not be contained")
	}
	if s.ContainsPoint(10, 4) {
		t.Error("after selection should not be contained")
	}
	if s.ContainsPoint(1, 1) {
		t.Error("start row before start col should not be contained")
	}
	if s.ContainsPoint(9, 3) {
		t.Error("end row after end col should not be contained")
	}
}

func TestState_GetSelectedText(t *testing.T) {
	content := "line zero\nline one here\nline two there\nline three"
	s := New()

	// Select "one here\nline two"
	s.Start(5, 1, ModeChar)
	s.Update(7, 2)

	got := s.GetSelectedText(content)
	if got != "one here\nline two" {
		t.Errorf("selected = %q", got)
	}
}

func TestState_GetSelectedText_SingleLine(t *testing.T) {
	content := "hello world"
	s := New()
	s.Start(6, 0, ModeChar)
	s.Update(10, 0)

	got := s.GetSelectedText(content)
	if got != "world" {
		t.Errorf("selected = %q", got)
	}
}

func TestState_GetSelectedText_Empty(t *testing.T) {
	s := New()
	if s.GetSelectedText("hello") != "" {
		t.Error("no selection should return empty")
	}
}

func TestState_GetSelectedText_NoSelectionChange(t *testing.T) {
	content := "abc"
	s := New()
	s.Start(1, 0, ModeChar)
	// Focus same as anchor
	if s.GetSelectedText(content) != "" {
		t.Error("empty selection should return empty text")
	}
}

func TestExpandToWord(t *testing.T) {
	line := "hello world-test foo"

	start, end := ExpandToWord(line, 7)
	// col 7 = 'o' in "world-test"
	word := string([]rune(line)[start : end+1])
	if word != "world-test" {
		t.Errorf("word = %q, start=%d end=%d", word, start, end)
	}
}

func TestExpandToWord_AtStart(t *testing.T) {
	line := "hello world"
	start, end := ExpandToWord(line, 0)
	word := string([]rune(line)[start : end+1])
	if word != "hello" {
		t.Errorf("word = %q", word)
	}
}

func TestExpandToWord_AtEnd(t *testing.T) {
	line := "hello world"
	start, end := ExpandToWord(line, 10)
	word := string([]rune(line)[start : end+1])
	if word != "world" {
		t.Errorf("word = %q", word)
	}
}

func TestExpandToLine(t *testing.T) {
	start, end := ExpandToLine("hello world")
	if start != 0 {
		t.Errorf("start = %d", start)
	}
	if end != 10 {
		t.Errorf("end = %d", end)
	}
}

func TestSelectionMode_Constants(t *testing.T) {
	if ModeChar != 0 {
		t.Error("ModeChar should be 0")
	}
	if ModeWord != 1 {
		t.Error("ModeWord should be 1")
	}
	if ModeLine != 2 {
		t.Error("ModeLine should be 2")
	}
}

func TestUpdate_NotDragging(t *testing.T) {
	s := New()
	s.Update(5, 5) // should not panic when not dragging
	if s.Focus != nil {
		t.Error("update without drag should not set focus")
	}
}
