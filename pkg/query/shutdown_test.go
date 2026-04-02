package query

import (
	"sync/atomic"
	"testing"
)

// Source: utils/cleanupRegistry.ts

func TestCleanupRegistry(t *testing.T) {

	t.Run("register_and_run", func(t *testing.T) {
		// Source: cleanupRegistry.ts:14-25
		r := NewCleanupRegistry()
		var ran int32

		r.Register(func() { atomic.AddInt32(&ran, 1) })
		r.Register(func() { atomic.AddInt32(&ran, 1) })

		r.RunAll()
		if atomic.LoadInt32(&ran) != 2 {
			t.Errorf("expected 2 functions to run, got %d", ran)
		}
	})

	t.Run("unregister", func(t *testing.T) {
		// Source: cleanupRegistry.ts:16
		r := NewCleanupRegistry()
		var ran int32

		unregister := r.Register(func() { atomic.AddInt32(&ran, 1) })
		r.Register(func() { atomic.AddInt32(&ran, 1) })

		unregister() // Remove first function

		r.RunAll()
		if atomic.LoadInt32(&ran) != 1 {
			t.Errorf("expected 1 function to run (1 unregistered), got %d", ran)
		}
	})

	t.Run("empty_registry_noop", func(t *testing.T) {
		r := NewCleanupRegistry()
		r.RunAll() // Should not panic
	})

	t.Run("concurrent_safe", func(t *testing.T) {
		r := NewCleanupRegistry()
		var count int32

		// Register from multiple goroutines
		done := make(chan struct{})
		for i := 0; i < 10; i++ {
			go func() {
				r.Register(func() { atomic.AddInt32(&count, 1) })
				done <- struct{}{}
			}()
		}
		for i := 0; i < 10; i++ {
			<-done
		}

		r.RunAll()
		if atomic.LoadInt32(&count) != 10 {
			t.Errorf("expected 10, got %d", count)
		}
	})

	t.Run("len_tracks_active", func(t *testing.T) {
		r := NewCleanupRegistry()
		r.Register(func() {})
		unreg := r.Register(func() {})
		r.Register(func() {})

		if r.Len() != 3 {
			t.Errorf("expected 3, got %d", r.Len())
		}

		unreg()
		if r.Len() != 2 {
			t.Errorf("expected 2 after unregister, got %d", r.Len())
		}
	})
}
