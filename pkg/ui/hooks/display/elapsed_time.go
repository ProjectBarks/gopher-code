package display

import (
	"fmt"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ElapsedTimeMsg is sent on each tick to trigger a re-render with the
// updated elapsed duration.
type ElapsedTimeMsg struct{ ID int }

// ElapsedTime tracks time since a start point and produces a formatted
// duration string suitable for spinners and status lines. It supports
// pausing, end-time freezing (for completed tasks), and configurable
// tick intervals.
//
// TS equivalent: useElapsedTime in src/hooks/useElapsedTime.ts
type ElapsedTime struct {
	mu       sync.Mutex
	id       int
	start    time.Time
	end      *time.Time // if set, duration is frozen at end-start
	paused   time.Duration
	interval time.Duration
	running  bool
}

// NewElapsedTime creates a new tracker. The timer starts in the stopped state;
// call Start to begin ticking.
func NewElapsedTime(id int, interval time.Duration) *ElapsedTime {
	if interval <= 0 {
		interval = time.Second
	}
	return &ElapsedTime{
		id:       id,
		interval: interval,
	}
}

// Start begins (or restarts) the timer from now.
func (e *ElapsedTime) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.start = time.Now()
	e.end = nil
	e.paused = 0
	e.running = true
}

// StartFrom begins the timer from a specific time.
func (e *ElapsedTime) StartFrom(t time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.start = t
	e.end = nil
	e.paused = 0
	e.running = true
}

// Stop freezes the elapsed duration at the current moment.
func (e *ElapsedTime) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	now := time.Now()
	e.end = &now
	e.running = false
}

// StopAt freezes the elapsed duration at a specific end time.
func (e *ElapsedTime) StopAt(t time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.end = &t
	e.running = false
}

// AddPaused adds to the accumulated paused duration that is subtracted
// from the total elapsed time.
func (e *ElapsedTime) AddPaused(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.paused += d
}

// IsRunning reports whether the timer is actively ticking.
func (e *ElapsedTime) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// Elapsed returns the raw duration.
func (e *ElapsedTime) Elapsed() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.elapsed()
}

func (e *ElapsedTime) elapsed() time.Duration {
	if e.start.IsZero() {
		return 0
	}
	var ref time.Time
	if e.end != nil {
		ref = *e.end
	} else {
		ref = time.Now()
	}
	d := ref.Sub(e.start) - e.paused
	if d < 0 {
		return 0
	}
	return d
}

// Format returns a compact human-readable elapsed string (e.g. "1m 23s").
// Matches the TS formatDuration with hideTrailingZeros.
func (e *ElapsedTime) Format() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return FormatDuration(e.elapsed())
}

// Tick returns a tea.Cmd that sends an ElapsedTimeMsg after the configured
// interval. Wire this into your bubbletea Update loop.
func (e *ElapsedTime) Tick() tea.Cmd {
	e.mu.Lock()
	id := e.id
	interval := e.interval
	running := e.running
	e.mu.Unlock()

	if !running {
		return nil
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return ElapsedTimeMsg{ID: id}
	})
}

// FormatDuration formats a duration into a compact human-readable string,
// hiding trailing zero components.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0 && s > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
