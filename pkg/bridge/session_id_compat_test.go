package bridge

import (
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// ToCompatSessionID
// ---------------------------------------------------------------------------

func TestToCompatSessionID(t *testing.T) {
	resetCseShimGate()
	cases := []struct {
		in, want string
	}{
		{"cse_abc123", "session_abc123"},
		{"session_abc123", "session_abc123"}, // no-op
		{"other_abc", "other_abc"},           // no-op
		{"", ""},
		{"cse_", "session_"},
	}
	for _, c := range cases {
		got := ToCompatSessionID(c.in)
		if got != c.want {
			t.Errorf("ToCompatSessionID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ToInfraSessionID
// ---------------------------------------------------------------------------

func TestToInfraSessionID(t *testing.T) {
	resetCseShimGate()
	cases := []struct {
		in, want string
	}{
		{"session_abc123", "cse_abc123"},
		{"cse_abc123", "cse_abc123"}, // no-op
		{"other_abc", "other_abc"},   // no-op
		{"", ""},
		{"session_", "cse_"},
	}
	for _, c := range cases {
		got := ToInfraSessionID(c.in)
		if got != c.want {
			t.Errorf("ToInfraSessionID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Round-trip
// ---------------------------------------------------------------------------

func TestSessionIDRoundTrip(t *testing.T) {
	resetCseShimGate()
	id := "cse_uuid-goes-here"
	compat := ToCompatSessionID(id)
	back := ToInfraSessionID(compat)
	if back != id {
		t.Errorf("round trip failed: %q -> %q -> %q", id, compat, back)
	}
}

// ---------------------------------------------------------------------------
// Kill-switch gate: disabled gate returns id unchanged
// ---------------------------------------------------------------------------

func TestToCompatSessionID_GateDisabled(t *testing.T) {
	SetCseShimGate(func() bool { return false })
	defer resetCseShimGate()

	id := "cse_abc123"
	got := ToCompatSessionID(id)
	if got != id {
		t.Errorf("disabled gate: ToCompatSessionID(%q) = %q, want unchanged", id, got)
	}
}

func TestToCompatSessionID_GateEnabled(t *testing.T) {
	SetCseShimGate(func() bool { return true })
	defer resetCseShimGate()

	got := ToCompatSessionID("cse_abc123")
	if got != "session_abc123" {
		t.Errorf("enabled gate: ToCompatSessionID = %q, want session_abc123", got)
	}
}

// ---------------------------------------------------------------------------
// Default-active: nil gate means shim is active
// ---------------------------------------------------------------------------

func TestToCompatSessionID_NilGateIsActive(t *testing.T) {
	resetCseShimGate()

	got := ToCompatSessionID("cse_abc123")
	if got != "session_abc123" {
		t.Errorf("nil gate should be active: got %q, want session_abc123", got)
	}
}

// ---------------------------------------------------------------------------
// Prefix constants
// ---------------------------------------------------------------------------

func TestPrefixConstants(t *testing.T) {
	if PrefixInfra != "cse_" {
		t.Errorf("PrefixInfra = %q", PrefixInfra)
	}
	if PrefixCompat != "session_" {
		t.Errorf("PrefixCompat = %q", PrefixCompat)
	}
}

// ---------------------------------------------------------------------------
// Concurrent access (race detector check)
// ---------------------------------------------------------------------------

func TestSessionIDCompat_ConcurrentAccess(t *testing.T) {
	resetCseShimGate()

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent reads + writes to the gate and conversions.
	wg.Add(goroutines * 3)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			SetCseShimGate(func() bool { return true })
		}()
		go func() {
			defer wg.Done()
			ToCompatSessionID("cse_concurrent")
		}()
		go func() {
			defer wg.Done()
			ToInfraSessionID("session_concurrent")
		}()
	}
	wg.Wait()
}
