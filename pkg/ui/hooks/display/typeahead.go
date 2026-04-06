// Package display provides bubbletea-oriented hooks for scroll and display
// concerns: typeahead buffering, elapsed time tracking, blink toggling, and
// minimum display time enforcement.
//
// Virtual scrolling is NOT reimplemented here. Use bubbles/v2/viewport
// directly; see scroll.go for rationale and helper constructors.
package display

import (
	"sync"

	tea "charm.land/bubbletea/v2"
)

// Typeahead buffers keyboard messages received while the UI is in a blocking
// state (e.g. waiting for a tool result or permission prompt). Once the block
// is lifted, buffered keystrokes are replayed in order so the user never
// "loses" input typed during a slow render cycle.
//
// This replaces the 1384-LOC React useTypeahead hook. The suggestion/autocomplete
// portion of that hook lives elsewhere; this struct is purely the input
// buffering primitive.
type Typeahead struct {
	mu      sync.Mutex
	blocked bool
	buf     []tea.KeyPressMsg
}

// NewTypeahead returns a Typeahead that starts unblocked.
func NewTypeahead() *Typeahead {
	return &Typeahead{}
}

// Block starts buffering. Subsequent Push calls queue keystrokes instead of
// letting them through.
func (t *Typeahead) Block() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.blocked = true
}

// Unblock stops buffering and returns all queued keystrokes in order.
// The internal buffer is cleared.
func (t *Typeahead) Unblock() []tea.KeyPressMsg {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.blocked = false
	if len(t.buf) == 0 {
		return nil
	}
	out := t.buf
	t.buf = nil
	return out
}

// IsBlocked reports whether the typeahead is currently buffering.
func (t *Typeahead) IsBlocked() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.blocked
}

// Push handles an incoming keystroke. If blocked, the key is buffered and
// ok is false (caller should swallow the event). If not blocked, ok is true
// (caller should process the event normally).
func (t *Typeahead) Push(key tea.KeyPressMsg) (ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.blocked {
		return true
	}
	t.buf = append(t.buf, key)
	return false
}

// Len returns the number of buffered keystrokes.
func (t *Typeahead) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.buf)
}

// Clear discards all buffered keystrokes without replaying them.
func (t *Typeahead) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = nil
}
