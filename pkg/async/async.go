// Package async provides background task and async utility functions.
// Source: utils/async/ (background tasks, debounce, throttle)
package async

import (
	"sync"
	"time"
)

// Debouncer delays execution until after a quiet period.
type Debouncer struct {
	mu       sync.Mutex
	timer    *time.Timer
	delay    time.Duration
}

// NewDebouncer creates a debouncer with the given delay.
func NewDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{delay: delay}
}

// Call schedules fn to run after the delay. Resets if called again before firing.
func (d *Debouncer) Call(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, fn)
}

// Cancel stops the pending debounced call.
func (d *Debouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

// Throttler limits execution to at most once per interval.
type Throttler struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

// NewThrottler creates a throttler with the given interval.
func NewThrottler(interval time.Duration) *Throttler {
	return &Throttler{interval: interval}
}

// Allow returns true if enough time has passed since the last allowed call.
func (t *Throttler) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if time.Since(t.last) >= t.interval {
		t.last = time.Now()
		return true
	}
	return false
}
