package permissions

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestResolveOnce_SingleClaim(t *testing.T) {
	r := NewResolveOnce(func(v int) {})

	if r.IsResolved() {
		t.Fatal("should not be resolved initially")
	}

	ok := r.Claim()
	if !ok {
		t.Fatal("first claim should succeed")
	}
	if !r.IsResolved() {
		t.Fatal("should be resolved after claim")
	}

	// Second claim must fail.
	ok = r.Claim()
	if ok {
		t.Fatal("second claim should fail")
	}
}

func TestResolveOnce_ResolveDelivers(t *testing.T) {
	var got int
	r := NewResolveOnce(func(v int) { got = v })

	r.Resolve(42)
	if got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}

	// Second resolve is a no-op.
	r.Resolve(99)
	if got != 42 {
		t.Fatalf("expected 42 (unchanged), got %d", got)
	}
}

func TestResolveOnce_ClaimThenResolve(t *testing.T) {
	var got int
	r := NewResolveOnce(func(v int) { got = v })

	ok := r.Claim()
	if !ok {
		t.Fatal("claim should succeed")
	}

	// Resolve should still deliver after claim.
	r.Resolve(7)
	if got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestResolveOnce_ConcurrentRace(t *testing.T) {
	// Source: PermissionContext.ts — createResolveOnce (race guard)
	// Multiple goroutines race to claim; exactly one should win.

	var callCount atomic.Int32
	r := NewResolveOnce(func(v int) {
		callCount.Add(1)
	})

	const racers = 100
	var wg sync.WaitGroup
	wg.Add(racers)
	winners := atomic.Int32{}

	for i := 0; i < racers; i++ {
		go func(val int) {
			defer wg.Done()
			if r.Claim() {
				winners.Add(1)
				r.Resolve(val)
			}
		}(i)
	}

	wg.Wait()

	if w := winners.Load(); w != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", w)
	}
	if c := callCount.Load(); c != 1 {
		t.Fatalf("expected exactly 1 resolve callback, got %d", c)
	}
}

func TestResolveOnce_ResolveWithoutClaim(t *testing.T) {
	// Resolve() implicitly claims.
	var got string
	r := NewResolveOnce(func(v string) { got = v })

	r.Resolve("hello")
	if got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
	if !r.IsResolved() {
		t.Fatal("should be resolved after Resolve()")
	}
	if r.Claim() {
		t.Fatal("claim after Resolve() should fail")
	}
}
