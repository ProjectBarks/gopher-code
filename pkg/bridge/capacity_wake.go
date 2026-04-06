// Source: src/bridge/capacityWake.ts
package bridge

import (
	"context"
	"sync"
)

// CapacityWake signals the poll loop to wake up early when capacity becomes
// available (e.g. a session ends, freeing a slot). The poll loop blocks on
// either its timer OR the wake signal, whichever fires first.
type CapacityWake struct {
	outer context.Context
	mu    sync.Mutex
	wake  chan struct{} // closed on Wake(), then re-armed
}

// NewCapacityWake creates a CapacityWake bound to the given outer context.
// When outer is cancelled the signal contexts are also cancelled.
func NewCapacityWake(outer context.Context) *CapacityWake {
	return &CapacityWake{
		outer: outer,
		wake:  make(chan struct{}),
	}
}

// Wake aborts the current at-capacity sleep and re-arms so the next Signal
// call gets a fresh channel. Calling Wake with no outstanding Signal is a
// safe no-op (the closed channel is simply replaced).
func (cw *CapacityWake) Wake() {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	select {
	case <-cw.wake:
		// already closed — nothing to do
	default:
		close(cw.wake)
	}
	cw.wake = make(chan struct{})
}

// Signal returns a context.Context that is cancelled when either the outer
// context is done OR Wake is called, whichever happens first. A background
// goroutine is spawned; callers who resolve normally (without abort) should
// use SignalWithCleanup to stop it.
func (cw *CapacityWake) Signal() context.Context {
	ctx, _ := cw.signalInternal()
	return ctx
}

// SignalWithCleanup is like Signal but also returns a cleanup function that
// cancels the merged context and stops the background goroutine.
func (cw *CapacityWake) SignalWithCleanup() (context.Context, context.CancelFunc) {
	return cw.signalInternal()
}

func (cw *CapacityWake) signalInternal() (context.Context, context.CancelFunc) {
	merged, cancel := context.WithCancel(cw.outer)

	// Short-circuit: if outer is already done, return immediately.
	if cw.outer.Err() != nil {
		cancel()
		return merged, func() {}
	}

	cw.mu.Lock()
	wakeCh := cw.wake
	cw.mu.Unlock()

	go func() {
		select {
		case <-merged.Done():
			// outer cancelled or cleanup called
		case <-wakeCh:
			cancel()
		}
	}()

	return merged, cancel
}
