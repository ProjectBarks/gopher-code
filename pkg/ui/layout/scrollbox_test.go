package layout

import (
	"strings"
	"testing"
)

func TestNewScrollBox(t *testing.T) {
	s := NewScrollBox(80, 10)
	if s.Width() != 80 {
		t.Errorf("width = %d", s.Width())
	}
	if s.Height() != 10 {
		t.Errorf("height = %d", s.Height())
	}
	if !s.IsSticky() {
		t.Error("should start sticky")
	}
	if !s.AtBottom() {
		t.Error("should start at bottom")
	}
}

func TestScrollBox_SetContent(t *testing.T) {
	s := NewScrollBox(80, 5)
	content := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7"
	s.SetContent(content)

	if s.ScrollHeight() != 7 {
		t.Errorf("scroll height = %d, want 7", s.ScrollHeight())
	}
	// Should still be sticky after SetContent
	if !s.IsSticky() {
		t.Error("should remain sticky after SetContent")
	}
}

func TestScrollBox_AppendContent(t *testing.T) {
	s := NewScrollBox(80, 5)
	s.SetContent("line 1")
	s.AppendContent("line 2")
	s.AppendContent("line 3")

	if s.ScrollHeight() != 3 {
		t.Errorf("scroll height = %d, want 3", s.ScrollHeight())
	}
}

func TestScrollBox_AppendToEmpty(t *testing.T) {
	s := NewScrollBox(80, 5)
	s.AppendContent("first line")
	if s.ScrollHeight() != 1 {
		t.Errorf("scroll height = %d, want 1", s.ScrollHeight())
	}
}

func TestScrollBox_ScrollTo(t *testing.T) {
	s := NewScrollBox(80, 3)
	s.SetContent(strings.Repeat("line\n", 20))

	s.ScrollTo(5)
	if s.ScrollTop() != 5 {
		t.Errorf("scroll top = %d, want 5", s.ScrollTop())
	}
	if s.IsSticky() {
		t.Error("ScrollTo should disable sticky")
	}
}

func TestScrollBox_ScrollBy(t *testing.T) {
	s := NewScrollBox(80, 3)
	s.SetContent(strings.Repeat("line\n", 20))
	s.ScrollTo(10)

	s.ScrollBy(-3)
	if s.ScrollTop() != 7 {
		t.Errorf("scroll top = %d, want 7", s.ScrollTop())
	}
	if s.IsSticky() {
		t.Error("ScrollBy up should disable sticky")
	}
}

func TestScrollBox_ScrollToBottom(t *testing.T) {
	s := NewScrollBox(80, 3)
	s.SetContent(strings.Repeat("line\n", 20))
	s.ScrollTo(5) // break sticky

	s.ScrollToBottom()
	if !s.IsSticky() {
		t.Error("ScrollToBottom should enable sticky")
	}
	if !s.AtBottom() {
		t.Error("should be at bottom")
	}
}

func TestScrollBox_SetSticky(t *testing.T) {
	s := NewScrollBox(80, 5)
	s.SetSticky(false)
	if s.IsSticky() {
		t.Error("should not be sticky")
	}
	s.SetSticky(true)
	if !s.IsSticky() {
		t.Error("should be sticky")
	}
}

func TestScrollBox_SetSize(t *testing.T) {
	s := NewScrollBox(80, 24)
	s.SetSize(120, 40)
	if s.Width() != 120 {
		t.Errorf("width = %d", s.Width())
	}
	if s.Height() != 40 {
		t.Errorf("height = %d", s.Height())
	}
}

func TestScrollBox_ViewportHeight(t *testing.T) {
	s := NewScrollBox(80, 15)
	if s.ViewportHeight() != 15 {
		t.Errorf("viewport height = %d", s.ViewportHeight())
	}
}

func TestScrollBox_View(t *testing.T) {
	s := NewScrollBox(80, 3)
	s.SetContent("a\nb\nc\nd\ne")
	v := s.View()
	// Should render something (viewport renders visible portion)
	if v == "" {
		t.Error("view should not be empty")
	}
}

func TestScrollBox_ScrollPercent(t *testing.T) {
	s := NewScrollBox(80, 5)
	s.SetContent(strings.Repeat("line\n", 20))
	// At bottom
	pct := s.ScrollPercent()
	// Should be close to 1.0 (at bottom)
	if pct < 0 || pct > 1.0 {
		t.Errorf("scroll percent = %f, should be 0-1", pct)
	}
}
