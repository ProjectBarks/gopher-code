package display

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// MinDisplayTimeMsg signals that the minimum display window has elapsed and
// a pending value can now be shown.
type MinDisplayTimeMsg struct{ ID int }

// MinDisplayTime throttles value changes so each distinct value stays visible
// for at least a configured duration. This prevents fast-cycling progress text
// from flickering past before it is readable.
//
// Unlike debounce (wait for quiet) or throttle (limit rate), this guarantees
// each value gets its minimum screen time before being replaced.
//
// TS equivalent: useMinDisplayTime in src/hooks/useMinDisplayTime.ts
type MinDisplayTime[T comparable] struct {
	mu        sync.Mutex
	id        int
	minD      time.Duration
	displayed T
	pending   *T
	shownAt   time.Time
	timerSet  bool

	// nowFn is injectable for testing; defaults to time.Now.
	nowFn func() time.Time
}

// NewMinDisplayTime creates a new throttle with the given minimum display
// duration. The initial displayed value is the zero value of T.
func NewMinDisplayTime[T comparable](id int, minDuration time.Duration) *MinDisplayTime[T] {
	return &MinDisplayTime[T]{
		id:    id,
		minD:  minDuration,
		nowFn: time.Now,
	}
}

// NewMinDisplayTimeWith creates a throttle with an explicit initial value.
func NewMinDisplayTimeWith[T comparable](id int, minDuration time.Duration, initial T) *MinDisplayTime[T] {
	return &MinDisplayTime[T]{
		id:        id,
		minD:      minDuration,
		displayed: initial,
		nowFn:     time.Now,
	}
}

// Update proposes a new value. If enough time has elapsed since the last
// change, the displayed value updates immediately and cmd is nil. Otherwise
// the value is stored as pending and a tea.Cmd is returned that will fire
// MinDisplayTimeMsg after the remaining wait.
func (m *MinDisplayTime[T]) Update(value T) tea.Cmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	if value == m.displayed {
		m.pending = nil
		return nil
	}

	now := m.nowFn()
	elapsed := now.Sub(m.shownAt)

	if elapsed >= m.minD {
		m.displayed = value
		m.shownAt = now
		m.pending = nil
		m.timerSet = false
		return nil
	}

	// Store pending; schedule a flush after the remaining time.
	m.pending = &value
	if m.timerSet {
		// Timer already in flight for a previous pending value; the msg
		// handler will pick up the latest pending.
		return nil
	}
	m.timerSet = true
	remaining := m.minD - elapsed
	id := m.id
	return tea.Tick(remaining, func(time.Time) tea.Msg {
		return MinDisplayTimeMsg{ID: id}
	})
}

// Flush is called when MinDisplayTimeMsg arrives. It promotes the pending
// value to displayed. Returns true if the displayed value actually changed.
func (m *MinDisplayTime[T]) Flush() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timerSet = false
	if m.pending == nil {
		return false
	}
	m.displayed = *m.pending
	m.shownAt = m.nowFn()
	m.pending = nil
	return true
}

// Displayed returns the current value that should be rendered.
func (m *MinDisplayTime[T]) Displayed() T {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.displayed
}
