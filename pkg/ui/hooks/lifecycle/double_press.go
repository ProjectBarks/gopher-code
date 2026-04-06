// Package lifecycle provides reusable primitives for exit and lifecycle management.
package lifecycle

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// DoublePressTimeoutMS is the window within which a second press must arrive
// to count as a double-press. Matches TS DOUBLE_PRESS_TIMEOUT_MS = 800.
const DoublePressTimeoutMS = 800

// doublePressResetMsg is sent after the timeout to clear pending state.
type doublePressResetMsg struct {
	id int // generation counter to ignore stale resets
}

// DoublePress detects two presses within an 800ms window.
//
// Usage:
//  1. Call Press() on key-down. It returns (fired bool, cmd tea.Cmd).
//     - First press: fired=false, cmd schedules a timeout reset.
//     - Second press within window: fired=true, cmd=nil.
//  2. Forward messages through Update() so the timeout reset is processed.
//  3. Call Reset() on any unrelated key to cancel the pending state.
//  4. Pending() reports whether a first press is awaiting confirmation.
type DoublePress struct {
	mu      sync.Mutex
	pending bool
	gen     int // incremented on every press/reset to expire stale timeouts
}

// NewDoublePress creates a new double-press detector.
func NewDoublePress() *DoublePress {
	return &DoublePress{}
}

// Press records a key press. Returns true if this is the confirming second
// press within the timeout window. When it returns false (first press), the
// returned tea.Cmd schedules the 800ms timeout reset — the caller must
// execute it.
func (dp *DoublePress) Press() (fired bool, cmd tea.Cmd) {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	if dp.pending {
		// Second press within window — fire.
		dp.pending = false
		dp.gen++
		return true, nil
	}

	// First press — arm the timeout.
	dp.pending = true
	dp.gen++
	gen := dp.gen
	cmd = tea.Tick(time.Duration(DoublePressTimeoutMS)*time.Millisecond, func(t time.Time) tea.Msg {
		return doublePressResetMsg{id: gen}
	})
	return false, cmd
}

// Update processes timeout messages. Call this from your model's Update.
func (dp *DoublePress) Update(msg tea.Msg) {
	if m, ok := msg.(doublePressResetMsg); ok {
		dp.mu.Lock()
		defer dp.mu.Unlock()
		if m.id == dp.gen {
			dp.pending = false
		}
	}
}

// Reset cancels the pending state (e.g. when an unrelated key is pressed).
func (dp *DoublePress) Reset() {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.pending = false
	dp.gen++ // expire any in-flight timeout
}

// Pending reports whether a first press is awaiting a second press.
func (dp *DoublePress) Pending() bool {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	return dp.pending
}
