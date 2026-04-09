package layout

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
)

// Source: ink/components/ScrollBox.tsx
//
// ScrollBox wraps bubbles/viewport with Claude-specific behavior:
//   - Sticky scroll: auto-pin to bottom when content grows
//   - Scroll position tracking for "N new messages" pill
//   - SetContent with auto-follow

// ScrollBox is a scrollable content area with sticky-scroll support.
type ScrollBox struct {
	viewport viewport.Model
	sticky   bool
	content  string
	atBottom bool
}

// NewScrollBox creates a scrollable area with the given dimensions.
func NewScrollBox(width, height int) ScrollBox {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	return ScrollBox{
		viewport: vp,
		sticky:   true,
		atBottom: true,
	}
}

// Init initializes the viewport.
func (s ScrollBox) Init() tea.Cmd {
	return s.viewport.Init()
}

// Update handles input and scroll messages.
func (s ScrollBox) Update(msg tea.Msg) (ScrollBox, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.viewport.SetWidth(msg.Width)
		s.viewport.SetHeight(msg.Height)
		if s.sticky {
			s.viewport.GotoBottom()
		}
		return s, nil
	}

	prevOffset := s.viewport.YOffset()
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	if s.viewport.YOffset() != prevOffset {
		s.sticky = s.viewport.AtBottom()
	}
	s.atBottom = s.viewport.AtBottom()
	return s, cmd
}

// View renders the visible portion of the content.
func (s ScrollBox) View() string {
	return s.viewport.View()
}

// SetContent replaces the scroll content. If sticky, auto-scrolls to bottom.
func (s *ScrollBox) SetContent(content string) {
	s.content = content
	s.viewport.SetContent(content)
	if s.sticky {
		s.viewport.GotoBottom()
	}
	s.atBottom = s.viewport.AtBottom()
}

// AppendContent adds text to the end. If sticky, auto-scrolls to bottom.
func (s *ScrollBox) AppendContent(text string) {
	if s.content == "" {
		s.content = text
	} else {
		s.content = s.content + "\n" + text
	}
	s.viewport.SetContent(s.content)
	if s.sticky {
		s.viewport.GotoBottom()
	}
	s.atBottom = s.viewport.AtBottom()
}

// ScrollToBottom pins the viewport to the bottom and enables sticky scroll.
func (s *ScrollBox) ScrollToBottom() {
	s.viewport.GotoBottom()
	s.sticky = true
	s.atBottom = true
}

// ScrollTo moves to a specific line offset. Disables sticky scroll.
func (s *ScrollBox) ScrollTo(y int) {
	s.viewport.SetYOffset(y)
	s.sticky = false
	s.atBottom = s.viewport.AtBottom()
}

// ScrollBy moves the viewport by a relative offset. Disables sticky if up.
func (s *ScrollBox) ScrollBy(dy int) {
	s.viewport.SetYOffset(s.viewport.YOffset() + dy)
	if dy < 0 {
		s.sticky = false
	}
	s.atBottom = s.viewport.AtBottom()
}

// IsSticky returns true if the viewport auto-follows new content.
func (s *ScrollBox) IsSticky() bool { return s.sticky }

// SetSticky enables or disables auto-follow.
func (s *ScrollBox) SetSticky(sticky bool) { s.sticky = sticky }

// AtBottom returns true if the viewport is at the bottom.
func (s *ScrollBox) AtBottom() bool { return s.atBottom }

// ScrollTop returns the current scroll position (line offset).
func (s *ScrollBox) ScrollTop() int { return s.viewport.YOffset() }

// ScrollHeight returns the total content height in lines.
func (s *ScrollBox) ScrollHeight() int {
	if s.content == "" {
		return 0
	}
	return strings.Count(s.content, "\n") + 1
}

// ViewportHeight returns the visible viewport height.
func (s *ScrollBox) ViewportHeight() int { return s.viewport.Height() }

// ScrollPercent returns the scroll position as 0.0–1.0.
func (s *ScrollBox) ScrollPercent() float64 {
	return s.viewport.ScrollPercent()
}

// SetSize updates the viewport dimensions.
func (s *ScrollBox) SetSize(width, height int) {
	s.viewport.SetWidth(width)
	s.viewport.SetHeight(height)
}

// Width returns the viewport width.
func (s *ScrollBox) Width() int { return s.viewport.Width() }

// Height returns the viewport height.
func (s *ScrollBox) Height() int { return s.viewport.Height() }
