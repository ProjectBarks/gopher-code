package bridge

// FlushGate is a state machine for gating message writes during an initial
// flush. When a bridge session starts, historical messages are flushed to the
// server via a single HTTP POST. During that flush, new messages must be
// queued to prevent them from arriving at the server interleaved with the
// historical messages.
//
// Lifecycle:
//
//	Start()      -> Enqueue() returns true, items are queued
//	End()        -> returns queued items for draining, Enqueue() returns false
//	Drop()       -> discards queued items (permanent transport close)
//	Deactivate() -> clears active flag without dropping items
//	               (transport replacement -- new transport will drain)
//
// FlushGate is NOT safe for concurrent use. The caller must coordinate access.
// The zero value is a valid, inactive gate.
type FlushGate[T any] struct {
	active  bool
	pending []T
}

// Active reports whether the gate is currently active (queuing items).
func (g *FlushGate[T]) Active() bool {
	return g.active
}

// PendingCount returns the number of items currently queued.
func (g *FlushGate[T]) PendingCount() int {
	return len(g.pending)
}

// Start marks the flush as in-progress. Enqueue will start queuing items.
func (g *FlushGate[T]) Start() {
	g.active = true
}

// End ends the flush and returns any queued items for draining.
// The caller is responsible for sending the returned items.
func (g *FlushGate[T]) End() []T {
	g.active = false
	result := g.pending
	g.pending = nil
	return result
}

// Enqueue queues items if the gate is active and returns true.
// If the gate is not active, it returns false and the caller should send directly.
func (g *FlushGate[T]) Enqueue(items ...T) bool {
	if !g.active {
		return false
	}
	g.pending = append(g.pending, items...)
	return true
}

// Drop discards all queued items (permanent transport close).
// Returns the number of items dropped.
func (g *FlushGate[T]) Drop() int {
	g.active = false
	count := len(g.pending)
	g.pending = nil
	return count
}

// Deactivate clears the active flag without dropping queued items.
// Used when the transport is replaced -- the new transport's flush will
// drain the pending items.
func (g *FlushGate[T]) Deactivate() {
	g.active = false
}
