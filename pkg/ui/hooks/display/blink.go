package display

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// DefaultBlinkInterval is the default blink cycle period (600ms matches TS).
const DefaultBlinkInterval = 600 * time.Millisecond

// BlinkMsg is sent on each blink tick to trigger a re-render.
type BlinkMsg struct{ ID int }

// Blink provides a periodic visible/hidden toggle for cursor and indicator
// blinking. All Blink instances with the same tick source stay synchronized
// because they derive state from a monotonic counter.
//
// TS equivalent: useBlink in src/hooks/useBlink.ts
type Blink struct {
	mu       sync.Mutex
	id       int
	interval time.Duration
	enabled  bool
	visible  bool
	count    int // monotonic toggle counter
}

// NewBlink creates a Blink with the given ID and interval.
// The blink starts disabled (always visible).
func NewBlink(id int, interval time.Duration) *Blink {
	if interval <= 0 {
		interval = DefaultBlinkInterval
	}
	return &Blink{
		id:       id,
		interval: interval,
		visible:  true,
	}
}

// Enable starts the blink cycle. The next Tick will toggle visibility.
func (b *Blink) Enable() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enabled = true
	b.count = 0
	b.visible = true
}

// Disable stops the blink cycle and locks visibility to true.
func (b *Blink) Disable() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enabled = false
	b.visible = true
	b.count = 0
}

// IsEnabled reports whether blinking is active.
func (b *Blink) IsEnabled() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.enabled
}

// Visible reports the current blink state. When disabled, always true.
func (b *Blink) Visible() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.visible
}

// Toggle advances the blink by one tick. Visible state is derived from
// the even/odd count (even = visible, odd = hidden), matching the TS
// `Math.floor(time / intervalMs) % 2 === 0` pattern.
func (b *Blink) Toggle() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.enabled {
		return
	}
	b.count++
	b.visible = b.count%2 == 0
}

// Tick returns a tea.Cmd that sends a BlinkMsg after the configured interval.
// Returns nil when disabled.
func (b *Blink) Tick() tea.Cmd {
	b.mu.Lock()
	id := b.id
	interval := b.interval
	enabled := b.enabled
	b.mu.Unlock()

	if !enabled {
		return nil
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return BlinkMsg{ID: id}
	})
}
