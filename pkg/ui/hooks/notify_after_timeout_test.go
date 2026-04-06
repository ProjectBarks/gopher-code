package hooks

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestInteractionTracker_Touch(t *testing.T) {
	tr := NewInteractionTracker()
	time.Sleep(5 * time.Millisecond)
	d1 := tr.SinceLastInteraction()

	tr.Touch()
	d2 := tr.SinceLastInteraction()

	if d2 >= d1 {
		t.Fatalf("after Touch, duration should decrease: d1=%v, d2=%v", d1, d2)
	}
}

func TestNotifyAfterTimeout_FiresWhenIdle(t *testing.T) {
	tracker := NewInteractionTracker()
	var fired atomic.Int32

	threshold := 50 * time.Millisecond

	// Backdate the tracker so it looks idle.
	tracker.lastTime.Store(time.Now().Add(-2 * threshold).UnixNano())

	n := newNotifyAfterTimeout(tracker, "done", "completion", func(msg, typ string) {
		if msg != "done" {
			t.Errorf("message = %q, want done", msg)
		}
		if typ != "completion" {
			t.Errorf("type = %q, want completion", typ)
		}
		fired.Add(1)
	}, threshold, false)
	defer n.Stop()

	// Wait for the ticker to fire.
	deadline := time.After(500 * time.Millisecond)
	for {
		if fired.Load() > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("notification did not fire within deadline")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if !n.HasFired() {
		t.Fatal("HasFired should be true")
	}
}

func TestNotifyAfterTimeout_DoesNotFireIfActive(t *testing.T) {
	tracker := NewInteractionTracker()
	var fired atomic.Int32

	threshold := 50 * time.Millisecond

	n := newNotifyAfterTimeout(tracker, "msg", "type", func(string, string) {
		fired.Add(1)
	}, threshold, false)
	defer n.Stop()

	// Keep touching to stay "active".
	for i := 0; i < 5; i++ {
		tracker.Touch()
		time.Sleep(20 * time.Millisecond)
	}

	if fired.Load() != 0 {
		t.Fatal("should not fire while user is active")
	}
}

func TestNotifyAfterTimeout_TestModeSkips(t *testing.T) {
	tracker := NewInteractionTracker()
	tracker.lastTime.Store(time.Now().Add(-time.Hour).UnixNano())

	n := newNotifyAfterTimeout(tracker, "msg", "type", func(string, string) {
		t.Fatal("should not fire in test mode")
	}, 10*time.Millisecond, true)
	defer n.Stop()

	time.Sleep(50 * time.Millisecond)
	if n.HasFired() {
		t.Fatal("should not fire in test mode")
	}
}

func TestNotifyAfterTimeout_StopIsIdempotent(t *testing.T) {
	tracker := NewInteractionTracker()
	n := newNotifyAfterTimeout(tracker, "msg", "type", func(string, string) {}, 50*time.Millisecond, false)
	n.Stop()
	n.Stop() // should not panic
}

func TestNotifyAfterTimeout_OneShotGuard(t *testing.T) {
	tracker := NewInteractionTracker()
	var count atomic.Int32

	threshold := 30 * time.Millisecond
	tracker.lastTime.Store(time.Now().Add(-2 * threshold).UnixNano())

	n := newNotifyAfterTimeout(tracker, "msg", "type", func(string, string) {
		count.Add(1)
	}, threshold, false)
	defer n.Stop()

	time.Sleep(200 * time.Millisecond)
	if count.Load() != 1 {
		t.Fatalf("notification fired %d times, want exactly 1", count.Load())
	}
}

func TestNotifyAfterTimeout_ResetOnCreation(t *testing.T) {
	tracker := NewInteractionTracker()
	// Backdate to look stale.
	tracker.lastTime.Store(time.Now().Add(-time.Hour).UnixNano())

	// Creating the notifier should reset the tracker (mount-time reset).
	_ = newNotifyAfterTimeout(tracker, "msg", "type", func(string, string) {}, time.Hour, true)
	if tracker.SinceLastInteraction() > time.Second {
		t.Fatal("tracker should have been reset on creation")
	}
}
