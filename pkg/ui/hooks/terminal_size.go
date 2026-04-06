package hooks

import "sync"

// TerminalSize holds the current terminal dimensions.
// In Bubbletea, this is populated from tea.WindowSizeMsg events.
// Source: hooks/useTerminalSize.ts + ink/components/TerminalSizeContext.tsx
type TerminalSize struct {
	Width  int
	Height int
}

// TerminalSizeTracker tracks terminal dimensions and notifies listeners on resize.
// Unlike the React context-based approach, this is a simple observable struct.
type TerminalSizeTracker struct {
	mu        sync.RWMutex
	size      TerminalSize
	listeners []func(TerminalSize)
}

// NewTerminalSizeTracker creates a tracker with initial dimensions.
func NewTerminalSizeTracker(width, height int) *TerminalSizeTracker {
	return &TerminalSizeTracker{
		size: TerminalSize{Width: width, Height: height},
	}
}

// Size returns the current terminal dimensions.
func (t *TerminalSizeTracker) Size() TerminalSize {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size
}

// Width returns the current terminal width.
func (t *TerminalSizeTracker) Width() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size.Width
}

// Height returns the current terminal height.
func (t *TerminalSizeTracker) Height() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size.Height
}

// Update sets new dimensions and notifies listeners if changed.
// Called from the Bubbletea Update method on WindowSizeMsg.
func (t *TerminalSizeTracker) Update(width, height int) bool {
	t.mu.Lock()
	if t.size.Width == width && t.size.Height == height {
		t.mu.Unlock()
		return false
	}
	t.size = TerminalSize{Width: width, Height: height}
	// Copy listeners under lock, notify outside lock to avoid deadlocks.
	listeners := make([]func(TerminalSize), len(t.listeners))
	copy(listeners, t.listeners)
	size := t.size
	t.mu.Unlock()

	for _, fn := range listeners {
		if fn != nil {
			fn(size)
		}
	}
	return true
}

// OnResize registers a callback invoked when the terminal is resized.
// Returns an unsubscribe function.
func (t *TerminalSizeTracker) OnResize(fn func(TerminalSize)) func() {
	t.mu.Lock()
	t.listeners = append(t.listeners, fn)
	idx := len(t.listeners) - 1
	t.mu.Unlock()

	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		// Nil out to avoid slice shuffling; compacted on next subscribe.
		if idx < len(t.listeners) {
			t.listeners[idx] = nil
		}
	}
}
