package hooks

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Source: ink/components/ClockContext.tsx, TerminalFocusContext.tsx, StdinContext.ts
//
// In TS, these are React contexts providing terminal state to all components.
// In Go, they're struct fields tracked via tea.Msg in the Update loop.

// ---------------------------------------------------------------------------
// Terminal Focus — Source: ink/components/TerminalFocusContext.tsx
// ---------------------------------------------------------------------------

// FocusState describes how the terminal focus was determined.
type FocusState string

const (
	FocusUnknown  FocusState = "unknown"  // no focus events received yet
	FocusFocused  FocusState = "focused"  // terminal has focus
	FocusBlurred  FocusState = "blurred"  // terminal lost focus
)

// FocusTracker tracks terminal focus state via tea.FocusMsg/tea.BlurMsg.
type FocusTracker struct {
	mu      sync.RWMutex
	focused bool
	state   FocusState
}

// NewFocusTracker creates a tracker that assumes the terminal is focused.
func NewFocusTracker() *FocusTracker {
	return &FocusTracker{focused: true, state: FocusUnknown}
}

// IsFocused returns true if the terminal currently has focus.
func (f *FocusTracker) IsFocused() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.focused
}

// State returns the current focus state.
func (f *FocusTracker) State() FocusState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state
}

// HandleMsg processes focus-related tea messages. Returns true if state changed.
func (f *FocusTracker) HandleMsg(msg tea.Msg) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch msg.(type) {
	case tea.FocusMsg:
		changed := !f.focused || f.state != FocusFocused
		f.focused = true
		f.state = FocusFocused
		return changed
	case tea.BlurMsg:
		changed := f.focused || f.state != FocusBlurred
		f.focused = false
		f.state = FocusBlurred
		return changed
	}
	return false
}

// ---------------------------------------------------------------------------
// Animation Clock — Source: ink/components/ClockContext.tsx
// ---------------------------------------------------------------------------

// FrameIntervalMS is the default animation frame interval.
const FrameIntervalMS = 16 // ~60fps

// BlurredFrameIntervalMS is the frame interval when terminal is blurred.
const BlurredFrameIntervalMS = 32 // ~30fps

// AnimationTickMsg is sent by the clock on each animation frame.
type AnimationTickMsg struct {
	Elapsed time.Duration
}

// Clock provides synchronized animation timing.
// In TS, this is a subscriber-based clock context. In Go, it sends
// AnimationTickMsg via tea.Tick when active.
type Clock struct {
	mu       sync.Mutex
	start    time.Time
	interval time.Duration
	active   bool
}

// NewClock creates a clock with the default frame interval.
func NewClock() *Clock {
	return &Clock{
		start:    time.Now(),
		interval: FrameIntervalMS * time.Millisecond,
	}
}

// Start begins sending animation tick messages.
func (c *Clock) Start() tea.Cmd {
	c.mu.Lock()
	c.active = true
	interval := c.interval
	start := c.start
	c.mu.Unlock()

	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return AnimationTickMsg{Elapsed: t.Sub(start)}
	})
}

// Stop stops the clock.
func (c *Clock) Stop() {
	c.mu.Lock()
	c.active = false
	c.mu.Unlock()
}

// IsActive returns true if the clock is running.
func (c *Clock) IsActive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active
}

// Tick returns the next tick command if the clock is active.
// Call this from Update() after handling AnimationTickMsg.
func (c *Clock) Tick() tea.Cmd {
	c.mu.Lock()
	active := c.active
	interval := c.interval
	start := c.start
	c.mu.Unlock()

	if !active {
		return nil
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return AnimationTickMsg{Elapsed: t.Sub(start)}
	})
}

// Now returns milliseconds elapsed since the clock started.
func (c *Clock) Now() int64 {
	return time.Since(c.start).Milliseconds()
}

// SetFocused adjusts the tick interval based on terminal focus.
func (c *Clock) SetFocused(focused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if focused {
		c.interval = FrameIntervalMS * time.Millisecond
	} else {
		c.interval = BlurredFrameIntervalMS * time.Millisecond
	}
}

// ---------------------------------------------------------------------------
// Cursor Visibility — Source: ink cursor management
// ---------------------------------------------------------------------------

// CursorState tracks whether the terminal cursor should be visible.
type CursorState struct {
	visible bool
}

// NewCursorState creates a cursor state (default visible).
func NewCursorState() *CursorState {
	return &CursorState{visible: true}
}

// IsVisible returns true if the cursor should be shown.
func (c *CursorState) IsVisible() bool { return c.visible }

// Show makes the cursor visible.
func (c *CursorState) Show() { c.visible = true }

// Hide makes the cursor invisible.
func (c *CursorState) Hide() { c.visible = false }
