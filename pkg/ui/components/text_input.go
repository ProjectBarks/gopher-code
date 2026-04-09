package components

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Source: components/TextInput.tsx, components/BaseTextInput.tsx
//
// In TS, TextInput is a React component wrapping BaseTextInput with
// voice waveform, clipboard hints, and animation. In Go, we wrap
// bubbles/v2/textinput with our styling and add placeholder rendering.

// TextInputModel wraps bubbles/v2/textinput with Claude Code styling.
type TextInputModel struct {
	inner       textinput.Model
	placeholder string
	focused     bool
	width       int
}

// NewTextInput creates a styled text input.
func NewTextInput(placeholder string) TextInputModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 0 // no limit
	ti.Prompt = ""   // we render our own prompt

	return TextInputModel{
		inner:       ti,
		placeholder: placeholder,
		width:       80,
	}
}

// Init initializes the text input.
func (m TextInputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles key events.
func (m TextInputModel) Update(msg tea.Msg) (TextInputModel, tea.Cmd) {
	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}

// View renders the text input.
func (m TextInputModel) View() string {
	return m.inner.View()
}

// Focus gives the input focus.
func (m *TextInputModel) Focus() tea.Cmd {
	m.focused = true
	return m.inner.Focus()
}

// Blur removes focus.
func (m *TextInputModel) Blur() {
	m.focused = false
	m.inner.Blur()
}

// Focused returns true if the input has focus.
func (m TextInputModel) Focused() bool {
	return m.focused
}

// Value returns the current text.
func (m TextInputModel) Value() string {
	return m.inner.Value()
}

// SetValue sets the text content.
func (m *TextInputModel) SetValue(s string) {
	m.inner.SetValue(s)
}

// SetWidth sets the input width.
func (m *TextInputModel) SetWidth(w int) {
	m.width = w
	m.inner.SetWidth(w)
}

// SetPlaceholder updates the placeholder text.
func (m *TextInputModel) SetPlaceholder(s string) {
	m.placeholder = s
	m.inner.Placeholder = s
}

// SetPrompt sets the prompt prefix (e.g., "> ").
func (m *TextInputModel) SetPrompt(s string) {
	m.inner.Prompt = s
}

// SetCharLimit sets the maximum character limit. 0 = unlimited.
func (m *TextInputModel) SetCharLimit(n int) {
	m.inner.CharLimit = n
}

// CursorEnd moves the cursor to the end of the input.
func (m *TextInputModel) CursorEnd() {
	m.inner.CursorEnd()
}

// CursorStart moves the cursor to the start.
func (m *TextInputModel) CursorStart() {
	m.inner.CursorStart()
}

// Reset clears the input value.
func (m *TextInputModel) Reset() {
	m.inner.Reset()
}

// IsEmpty returns true if the input has no text.
func (m TextInputModel) IsEmpty() bool {
	return strings.TrimSpace(m.inner.Value()) == ""
}

// RenderWithStyle renders the input with custom styling.
func (m TextInputModel) RenderWithStyle(style lipgloss.Style) string {
	return style.Render(m.inner.View())
}
