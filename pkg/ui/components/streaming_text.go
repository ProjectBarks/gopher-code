package components

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// StreamingText buffers text deltas during streaming with an animated cursor.
// Used to display real-time LLM responses as they arrive.
type StreamingText struct {
	buffer      strings.Builder
	cursorTick  int           // Counter for cursor animation (0-3 cycles)
	isStreaming bool          // True while receiving deltas
	theme       theme.Theme   // Theme for cursor styling
	width       int           // Available width for rendering
	lastTick    time.Time     // Track last tick time
	tickCounter int           // Count ticks to control blink rate
}

// NewStreamingText creates a new StreamingText buffer.
func NewStreamingText(t theme.Theme) *StreamingText {
	return &StreamingText{
		theme:       t,
		isStreaming: false,
		width:       80,
		lastTick:    time.Now(),
		tickCounter: 0,
	}
}

// AppendDelta adds text to the buffer.
// This is called as text deltas arrive from the LLM.
func (st *StreamingText) AppendDelta(text string) {
	st.buffer.WriteString(text)
	st.isStreaming = true
}

// TickMsg is a custom message for cursor animation.
type StreamingTextTickMsg struct{}

// Update handles messages (tick for cursor animation).
func (st *StreamingText) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case StreamingTextTickMsg:
		st.tickCounter++
		// Cursor blinks every 4 ticks (roughly 500ms at 8Hz)
		st.cursorTick = (st.tickCounter / 4) % 2
		return st, nil
	}
	return st, nil
}

// View renders the buffered text with an optional blinking cursor.
func (st *StreamingText) View() tea.View {
	content := st.buffer.String()

	// Add cursor if still streaming
	if st.isStreaming {
		cursor := st.renderCursor()
		content += cursor
	}

	return tea.NewView(content)
}

// renderCursor renders the blinking cursor animation.
func (st *StreamingText) renderCursor() string {
	cs := st.theme.Colors()
	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.Cursor)).
		Blink(true)

	// Toggle between solid and empty cursor
	if st.cursorTick == 0 {
		return cursorStyle.Render("▊") // Filled cursor
	}
	return cursorStyle.Render("▒") // Light cursor (blink effect)
}

// Complete marks the stream as complete (removes cursor).
func (st *StreamingText) Complete() {
	st.isStreaming = false
}

// Reset clears the buffer and resets state.
func (st *StreamingText) Reset() {
	st.buffer.Reset()
	st.isStreaming = false
	st.cursorTick = 0
	st.tickCounter = 0
}

// Text returns the buffered text without the cursor.
func (st *StreamingText) Text() string {
	return st.buffer.String()
}

// SetSize sets the available width for rendering.
func (st *StreamingText) SetSize(width, height int) {
	st.width = width
}

// Init initializes the component.
func (st *StreamingText) Init() tea.Cmd {
	return nil
}

// Ensure StreamingText implements tea.Model interface.
var _ tea.Model = (*StreamingText)(nil)
