// Package events provides event types and propagation control for the TUI.
//
// Source: ink/events/*.ts
//
// In TS, Ink has a DOM-style event system with capture/bubble phases.
// In Go/bubbletea, events are tea.Msg structs in Update(). This package
// provides the event types and a Handled flag for propagation control
// (stopImmediatePropagation equivalent).
package events

import tea "charm.land/bubbletea/v2"

// Handled wraps a tea.Msg to indicate it was consumed and should not
// propagate further. This is the Go equivalent of stopImmediatePropagation().
//
// Usage: if a component handles a key, return HandledMsg{} from Update()
// so parent models know not to process it again.
type Handled struct {
	Msg tea.Msg
}

// ClickMsg is a mouse click event at screen coordinates.
// Source: ink/events/click-event.ts
type ClickMsg struct {
	// Col is the 0-indexed screen column.
	Col int
	// Row is the 0-indexed screen row.
	Row int
	// LocalCol is the column relative to the receiving component.
	LocalCol int
	// LocalRow is the row relative to the receiving component.
	LocalRow int
	// CellIsBlank is true if the clicked cell has no visible content.
	CellIsBlank bool
	// stopped is true if propagation was stopped.
	stopped bool
}

// StopPropagation prevents this click from bubbling to parent components.
func (e *ClickMsg) StopPropagation() { e.stopped = true }

// Stopped returns true if propagation was stopped.
func (e *ClickMsg) Stopped() bool { return e.stopped }

// FocusChangeMsg indicates a component gained or lost focus.
// Source: ink/events/focus-event.ts
type FocusChangeMsg struct {
	// Focused is true if the component gained focus.
	Focused bool
	// ComponentID identifies which component changed focus.
	ComponentID string
}

// ScrollMsg indicates a scroll event (mouse wheel or keyboard).
type ScrollMsg struct {
	// Delta is positive for scroll down, negative for scroll up.
	Delta int
	// Col is the column where the scroll occurred (for mouse wheel).
	Col int
	// Row is the row where the scroll occurred.
	Row int
}

// InputMsg wraps a key press with additional metadata.
// Source: ink/events/input-event.ts
type InputMsg struct {
	// Key is the original bubbletea key press.
	Key tea.KeyPressMsg
	// Input is the text input character(s), or empty for special keys.
	Input string
	// stopped is true if propagation was stopped.
	stopped bool
}

// StopPropagation prevents this input from being handled by other components.
func (e *InputMsg) StopPropagation() { e.stopped = true }

// Stopped returns true if propagation was stopped.
func (e *InputMsg) Stopped() bool { return e.stopped }

// NewInputMsg creates an InputMsg from a bubbletea key press.
func NewInputMsg(key tea.KeyPressMsg) InputMsg {
	input := ""
	if key.Code >= 32 && key.Code < 127 && key.Mod == 0 {
		input = string(rune(key.Code))
	}
	return InputMsg{Key: key, Input: input}
}

// PasteMsg indicates text was pasted (multi-character input).
type PasteMsg struct {
	Text string
}

// ResizeMsg indicates the terminal was resized (wraps tea.WindowSizeMsg).
type ResizeMsg struct {
	Width  int
	Height int
}

// NewResizeMsg creates a ResizeMsg from a bubbletea window size message.
func NewResizeMsg(msg tea.WindowSizeMsg) ResizeMsg {
	return ResizeMsg{Width: msg.Width, Height: msg.Height}
}

// TerminalFocusMsg indicates the terminal gained/lost OS focus.
// Source: ink/events/terminal-focus-event.ts
type TerminalFocusMsg struct {
	Focused bool
}

// ---------------------------------------------------------------------------
// Event dispatch helpers
// ---------------------------------------------------------------------------

// IsKeyPress returns true if the message is a key press.
func IsKeyPress(msg tea.Msg) bool {
	_, ok := msg.(tea.KeyPressMsg)
	return ok
}

// IsClick returns true if the message is a click event.
func IsClick(msg tea.Msg) bool {
	_, ok := msg.(*ClickMsg)
	if ok {
		return true
	}
	_, ok = msg.(ClickMsg)
	return ok
}

// IsScroll returns true if the message is a scroll event.
func IsScroll(msg tea.Msg) bool {
	_, ok := msg.(ScrollMsg)
	return ok
}

// KeyIs returns true if the message is a specific key.
func KeyIs(msg tea.Msg, code rune) bool {
	key, ok := msg.(tea.KeyPressMsg)
	return ok && key.Code == code
}

// KeyHasMod returns true if the key has the specified modifier.
func KeyHasMod(msg tea.Msg, mod tea.KeyMod) bool {
	key, ok := msg.(tea.KeyPressMsg)
	return ok && key.Mod&mod != 0
}
