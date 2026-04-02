package query

import "sync"

// Source: utils/QueryGuard.ts

// GuardStatus represents the state of the query guard.
// Source: utils/QueryGuard.ts:6-8
type GuardStatus string

const (
	GuardIdle        GuardStatus = "idle"
	GuardDispatching GuardStatus = "dispatching"
	GuardRunning     GuardStatus = "running"
)

// QueryGuard is a synchronous state machine for the query lifecycle.
//
// Three states:
//   - idle: no query, safe to dequeue and process
//   - dispatching: an item was dequeued, async chain hasn't started yet
//   - running: query is executing
//
// Transitions:
//   - idle → dispatching (Reserve)
//   - dispatching → running (TryStart)
//   - idle → running (TryStart, for direct submissions)
//   - running → idle (End / ForceEnd)
//   - dispatching → idle (CancelReservation)
//
// Source: utils/QueryGuard.ts:29-121
type QueryGuard struct {
	mu         sync.Mutex
	status     GuardStatus
	generation int
	listeners  []func()
}

// NewQueryGuard creates a new query guard in idle state.
func NewQueryGuard() *QueryGuard {
	return &QueryGuard{status: GuardIdle}
}

// Reserve transitions idle → dispatching for queue processing.
// Returns false if not idle.
// Source: utils/QueryGuard.ts:38-43
func (g *QueryGuard) Reserve() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status != GuardIdle {
		return false
	}
	g.status = GuardDispatching
	g.notify()
	return true
}

// CancelReservation transitions dispatching → idle.
// Source: utils/QueryGuard.ts:49-53
func (g *QueryGuard) CancelReservation() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status != GuardDispatching {
		return
	}
	g.status = GuardIdle
	g.notify()
}

// TryStart starts a query. Returns the generation number on success,
// or -1 if a query is already running.
// Accepts transitions from both idle and dispatching.
// Source: utils/QueryGuard.ts:61-67
func (g *QueryGuard) TryStart() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status == GuardRunning {
		return -1
	}
	g.status = GuardRunning
	g.generation++
	g.notify()
	return g.generation
}

// End completes a query. Returns true if this generation is current
// (caller should perform cleanup). Returns false for stale generations.
// Source: utils/QueryGuard.ts:74-80
func (g *QueryGuard) End(generation int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.generation != generation {
		return false
	}
	if g.status != GuardRunning {
		return false
	}
	g.status = GuardIdle
	g.notify()
	return true
}

// ForceEnd terminates any active query regardless of generation.
// Increments generation so stale cleanup is skipped.
// Source: utils/QueryGuard.ts:88-93
func (g *QueryGuard) ForceEnd() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status == GuardIdle {
		return
	}
	g.status = GuardIdle
	g.generation++
	g.notify()
}

// IsActive returns true when dispatching or running.
// Source: utils/QueryGuard.ts:99-101
func (g *QueryGuard) IsActive() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.status != GuardIdle
}

// Status returns the current guard status.
func (g *QueryGuard) Status() GuardStatus {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.status
}

// Generation returns the current generation number.
// Source: utils/QueryGuard.ts:103-105
func (g *QueryGuard) Generation() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.generation
}

// Subscribe registers a listener for state changes.
// Returns an unsubscribe function.
func (g *QueryGuard) Subscribe(fn func()) func() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.listeners = append(g.listeners, fn)
	idx := len(g.listeners) - 1
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		if idx < len(g.listeners) {
			g.listeners[idx] = nil
		}
	}
}

func (g *QueryGuard) notify() {
	for _, fn := range g.listeners {
		if fn != nil {
			fn()
		}
	}
}
