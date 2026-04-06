package bridge

import (
	"context"
	"testing"
	"time"
)

func TestCapacityWake_SignalWakesWaiter(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cw := NewCapacityWake(ctx)

	sig := cw.Signal()

	// Wake should unblock the signal context.
	done := make(chan struct{})
	go func() {
		<-sig.Done()
		close(done)
	}()

	cw.Wake()

	select {
	case <-done:
		// success
	case <-time.After(time.Second):
		t.Fatal("signal was not woken within timeout")
	}
}

func TestCapacityWake_OuterCancelWakesWaiter(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	cw := NewCapacityWake(ctx)
	sig := cw.Signal()

	done := make(chan struct{})
	go func() {
		<-sig.Done()
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// success — outer cancellation propagated
	case <-time.After(time.Second):
		t.Fatal("outer cancel did not propagate to signal")
	}
}

func TestCapacityWake_SignalWithoutWaiterIsNoop(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cw := NewCapacityWake(ctx)

	// Wake with no outstanding signal must not panic.
	cw.Wake()
	cw.Wake()

	// A subsequent Signal should still work normally.
	sig := cw.Signal()
	go func() {
		time.Sleep(10 * time.Millisecond)
		cw.Wake()
	}()

	select {
	case <-sig.Done():
		// success
	case <-time.After(time.Second):
		t.Fatal("signal after no-op wakes should still work")
	}
}

func TestCapacityWake_ResetClearsPendingSignal(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cw := NewCapacityWake(ctx)

	// Wake fires but nobody is listening — the wake is "pending".
	cw.Wake()

	// The next Signal should NOT be pre-fired; it should require a new Wake.
	sig := cw.Signal()

	select {
	case <-sig.Done():
		t.Fatal("signal should not be immediately done after wake with no waiter")
	case <-time.After(50 * time.Millisecond):
		// good — not pre-fired
	}

	// Now actually wake it.
	cw.Wake()

	select {
	case <-sig.Done():
		// success
	case <-time.After(time.Second):
		t.Fatal("signal was not woken after explicit wake")
	}
}

func TestCapacityWake_PreCancelledOuter(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	cw := NewCapacityWake(ctx)
	sig := cw.Signal()

	select {
	case <-sig.Done():
		// success — immediately done because outer is already cancelled
	case <-time.After(100 * time.Millisecond):
		t.Fatal("signal from pre-cancelled outer should be immediately done")
	}
}

func TestCapacityWake_CleanupStopsGoroutine(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cw := NewCapacityWake(ctx)
	sig, cleanup := cw.SignalWithCleanup()

	// Cleanup before any wake — the context should be cancelled (cleanup cancels the merged ctx).
	cleanup()

	select {
	case <-sig.Done():
		// success — cleanup cancelled the signal
	case <-time.After(100 * time.Millisecond):
		t.Fatal("cleanup should cancel the signal context")
	}
}
