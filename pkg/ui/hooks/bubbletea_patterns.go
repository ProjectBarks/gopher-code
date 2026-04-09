// Package hooks provides bubbletea equivalents of Ink React hooks.
//
// Source: ink/hooks/use-*.ts
//
// In TS, Ink uses React hooks for side effects and subscriptions.
// In Go/bubbletea, these map to: tea.Cmd for async operations,
// tea.Tick for intervals, struct fields for state, and Update()
// for event handling. This file provides the remaining hook equivalents.

package hooks

import (
	"io"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// useInterval → IntervalTicker (tea.Tick)
// Source: ink/hooks/use-interval.ts
// ---------------------------------------------------------------------------

// IntervalTickMsg is sent by an interval ticker.
type IntervalTickMsg struct {
	ID string // identifies which interval this belongs to
}

// IntervalTicker sends periodic tick messages.
type IntervalTicker struct {
	ID       string
	Interval time.Duration
	Active   bool
}

// NewIntervalTicker creates a ticker with the given interval.
func NewIntervalTicker(id string, interval time.Duration) *IntervalTicker {
	return &IntervalTicker{ID: id, Interval: interval, Active: true}
}

// Start returns a tea.Cmd that sends the first tick.
func (t *IntervalTicker) Start() tea.Cmd {
	if !t.Active {
		return nil
	}
	id := t.ID
	return tea.Tick(t.Interval, func(_ time.Time) tea.Msg {
		return IntervalTickMsg{ID: id}
	})
}

// Tick returns the next tick command (call from Update after IntervalTickMsg).
func (t *IntervalTicker) Tick() tea.Cmd {
	return t.Start()
}

// Stop pauses the ticker.
func (t *IntervalTicker) Stop() { t.Active = false }

// Resume resumes the ticker.
func (t *IntervalTicker) Resume() { t.Active = true }

// ---------------------------------------------------------------------------
// useAnimationFrame → AnimationFrame (high-frequency tick)
// Source: ink/hooks/use-animation-frame.ts
// ---------------------------------------------------------------------------

// AnimationFrameMsg is sent on each animation frame.
type AnimationFrameMsg struct {
	Time  time.Duration // time since animation started
	Frame int           // frame count
}

// AnimationFrame provides 60fps animation timing.
type AnimationFrame struct {
	start    time.Time
	frame    int
	interval time.Duration
	active   bool
}

// NewAnimationFrame creates a frame timer at ~60fps.
func NewAnimationFrame() *AnimationFrame {
	return &AnimationFrame{
		start:    time.Now(),
		interval: 16 * time.Millisecond,
		active:   true,
	}
}

// Start begins the animation loop.
func (a *AnimationFrame) Start() tea.Cmd {
	if !a.active {
		return nil
	}
	start := a.start
	return tea.Tick(a.interval, func(t time.Time) tea.Msg {
		return AnimationFrameMsg{Time: t.Sub(start)}
	})
}

// Next returns the next frame command (call from Update after AnimationFrameMsg).
func (a *AnimationFrame) Next() tea.Cmd {
	a.frame++
	return a.Start()
}

// Frame returns the current frame count.
func (a *AnimationFrame) Frame() int { return a.frame }

// Stop stops the animation.
func (a *AnimationFrame) Stop() { a.active = false }

// IsActive returns true if the animation is running.
func (a *AnimationFrame) IsActive() bool { return a.active }

// ---------------------------------------------------------------------------
// useTabStatus → TabStatus (OSC 21337 tab indicator)
// Source: ink/hooks/use-tab-status.ts
// ---------------------------------------------------------------------------

// TabStatusKind identifies the tab status state.
type TabStatusKind string

const (
	TabStatusIdle    TabStatusKind = "idle"
	TabStatusBusy    TabStatusKind = "busy"
	TabStatusWaiting TabStatusKind = "waiting"
)

// TabStatus manages the terminal tab status indicator (OSC 21337).
type TabStatus struct {
	writer io.Writer
	kind   TabStatusKind
}

// NewTabStatus creates a tab status controller.
func NewTabStatus(w io.Writer) *TabStatus {
	if w == nil {
		return &TabStatus{}
	}
	return &TabStatus{writer: w}
}

// Set updates the tab status indicator.
func (s *TabStatus) Set(kind TabStatusKind) {
	if s.writer == nil {
		return
	}
	s.kind = kind
	// OSC 21337 format: indicator=<color>;status=<text>;status-color=<color>
	var indicator, status, statusColor string
	switch kind {
	case TabStatusIdle:
		indicator = "rgb(0,215,95)"
		status = "Idle"
		statusColor = "rgb(136,136,136)"
	case TabStatusBusy:
		indicator = "rgb(255,149,0)"
		status = "Working…"
		statusColor = "rgb(255,149,0)"
	case TabStatusWaiting:
		indicator = "rgb(95,135,255)"
		status = "Waiting"
		statusColor = "rgb(95,135,255)"
	}
	seq := "\x1b]21337;indicator=" + indicator + ";status=" + status + ";status-color=" + statusColor + "\x07"
	s.writer.Write([]byte(seq))
}

// Clear removes the tab status indicator.
func (s *TabStatus) Clear() {
	if s.writer == nil {
		return
	}
	s.writer.Write([]byte("\x1b]21337;indicator=;status=;status-color=\x07"))
}

// Kind returns the current status kind.
func (s *TabStatus) Kind() TabStatusKind { return s.kind }

// ---------------------------------------------------------------------------
// useDeclaredCursor → CursorPosition tracking
// Source: ink/hooks/use-declared-cursor.ts
// ---------------------------------------------------------------------------

// CursorPosition tracks where the terminal cursor should be placed.
type CursorPosition struct {
	Row     int
	Col     int
	Visible bool
}

// DefaultCursorPosition returns a hidden cursor.
func DefaultCursorPosition() CursorPosition {
	return CursorPosition{Visible: false}
}

// ---------------------------------------------------------------------------
// useTerminalViewport → ViewportState
// Source: ink/hooks/use-terminal-viewport.ts
// ---------------------------------------------------------------------------

// ViewportState tracks the terminal viewport dimensions and scroll position.
type ViewportState struct {
	Width       int
	Height      int
	ScrollTop   int
	ScrollLeft  int
	IsFullscreen bool
}

// NewViewportState creates viewport state from terminal dimensions.
func NewViewportState(width, height int) ViewportState {
	return ViewportState{Width: width, Height: height}
}

// HandleResize updates viewport dimensions from a window size message.
func (v *ViewportState) HandleResize(width, height int) {
	v.Width = width
	v.Height = height
}
