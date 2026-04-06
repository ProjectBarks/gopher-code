package compact

import (
	"sync"
	"testing"
)

// Source: services/compact/compactWarningState.ts

func TestCompactWarningState_InitiallyNotSuppressed(t *testing.T) {
	s := NewCompactWarningState()
	if s.IsSuppressed() {
		t.Error("expected new state to not be suppressed")
	}
}

func TestCompactWarningState_SuppressAndClear(t *testing.T) {
	s := NewCompactWarningState()

	s.Suppress()
	if !s.IsSuppressed() {
		t.Error("expected suppressed after Suppress()")
	}

	s.ClearSuppression()
	if s.IsSuppressed() {
		t.Error("expected not suppressed after ClearSuppression()")
	}
}

func TestCompactWarningState_DoubleSuppress(t *testing.T) {
	s := NewCompactWarningState()
	s.Suppress()
	s.Suppress()
	if !s.IsSuppressed() {
		t.Error("expected suppressed after double Suppress()")
	}
}

func TestCompactWarningState_ClearWithoutSuppress(t *testing.T) {
	s := NewCompactWarningState()
	s.ClearSuppression() // no-op on fresh state
	if s.IsSuppressed() {
		t.Error("expected not suppressed after ClearSuppression() on fresh state")
	}
}

func TestCompactWarningState_ConcurrentAccess(t *testing.T) {
	s := NewCompactWarningState()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Suppress()
		}()
		go func() {
			defer wg.Done()
			s.ClearSuppression()
		}()
	}
	wg.Wait()
	// No race — final value is indeterminate but must not panic.
	_ = s.IsSuppressed()
}
