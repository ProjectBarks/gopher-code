package query

import "testing"

// Source: utils/QueryGuard.ts

func TestQueryGuard(t *testing.T) {

	t.Run("starts_idle", func(t *testing.T) {
		g := NewQueryGuard()
		if g.Status() != GuardIdle {
			t.Errorf("expected idle, got %s", g.Status())
		}
		if g.IsActive() {
			t.Error("should not be active when idle")
		}
	})

	t.Run("reserve_idle_to_dispatching", func(t *testing.T) {
		// Source: QueryGuard.ts:38-43
		g := NewQueryGuard()
		if !g.Reserve() {
			t.Error("reserve from idle should succeed")
		}
		if g.Status() != GuardDispatching {
			t.Errorf("expected dispatching, got %s", g.Status())
		}
		if !g.IsActive() {
			t.Error("should be active when dispatching")
		}
	})

	t.Run("reserve_fails_when_not_idle", func(t *testing.T) {
		// Source: QueryGuard.ts:39
		g := NewQueryGuard()
		g.Reserve()
		if g.Reserve() {
			t.Error("second reserve should fail")
		}
	})

	t.Run("cancel_reservation", func(t *testing.T) {
		// Source: QueryGuard.ts:49-53
		g := NewQueryGuard()
		g.Reserve()
		g.CancelReservation()
		if g.Status() != GuardIdle {
			t.Errorf("expected idle after cancel, got %s", g.Status())
		}
	})

	t.Run("trystart_from_idle", func(t *testing.T) {
		// Source: QueryGuard.ts:61-67
		g := NewQueryGuard()
		gen := g.TryStart()
		if gen == -1 {
			t.Error("trystart from idle should succeed")
		}
		if g.Status() != GuardRunning {
			t.Errorf("expected running, got %s", g.Status())
		}
		if gen != 1 {
			t.Errorf("first generation should be 1, got %d", gen)
		}
	})

	t.Run("trystart_from_dispatching", func(t *testing.T) {
		// Source: QueryGuard.ts:59-60
		g := NewQueryGuard()
		g.Reserve()
		gen := g.TryStart()
		if gen == -1 {
			t.Error("trystart from dispatching should succeed")
		}
		if g.Status() != GuardRunning {
			t.Errorf("expected running, got %s", g.Status())
		}
	})

	t.Run("trystart_fails_when_running", func(t *testing.T) {
		// Source: QueryGuard.ts:62
		g := NewQueryGuard()
		g.TryStart()
		gen := g.TryStart()
		if gen != -1 {
			t.Error("trystart should fail when already running")
		}
	})

	t.Run("end_with_matching_generation", func(t *testing.T) {
		// Source: QueryGuard.ts:74-80
		g := NewQueryGuard()
		gen := g.TryStart()
		if !g.End(gen) {
			t.Error("end with current generation should succeed")
		}
		if g.Status() != GuardIdle {
			t.Errorf("expected idle after end, got %s", g.Status())
		}
	})

	t.Run("end_with_stale_generation", func(t *testing.T) {
		// Source: QueryGuard.ts:75
		g := NewQueryGuard()
		gen1 := g.TryStart()
		g.ForceEnd()
		g.TryStart()
		if g.End(gen1) {
			t.Error("end with stale generation should return false")
		}
	})

	t.Run("force_end", func(t *testing.T) {
		// Source: QueryGuard.ts:88-93
		g := NewQueryGuard()
		g.TryStart()
		g.ForceEnd()
		if g.Status() != GuardIdle {
			t.Errorf("expected idle after force end, got %s", g.Status())
		}
		// Generation should be incremented
		if g.Generation() != 2 {
			t.Errorf("expected generation 2 after force end, got %d", g.Generation())
		}
	})

	t.Run("force_end_noop_when_idle", func(t *testing.T) {
		// Source: QueryGuard.ts:89
		g := NewQueryGuard()
		g.ForceEnd()
		if g.Status() != GuardIdle {
			t.Error("force end when idle should be noop")
		}
		if g.Generation() != 0 {
			t.Errorf("generation should be 0, got %d", g.Generation())
		}
	})

	t.Run("subscribe_notified", func(t *testing.T) {
		g := NewQueryGuard()
		notified := 0
		g.Subscribe(func() { notified++ })

		g.Reserve()
		if notified != 1 {
			t.Errorf("expected 1 notification, got %d", notified)
		}

		g.TryStart()
		if notified != 2 {
			t.Errorf("expected 2 notifications, got %d", notified)
		}
	})

	t.Run("full_lifecycle", func(t *testing.T) {
		g := NewQueryGuard()

		// idle → dispatching
		g.Reserve()
		if g.Status() != GuardDispatching {
			t.Fatal("should be dispatching")
		}

		// dispatching → running
		gen := g.TryStart()
		if g.Status() != GuardRunning {
			t.Fatal("should be running")
		}

		// running → idle
		if !g.End(gen) {
			t.Fatal("end should succeed")
		}
		if g.Status() != GuardIdle {
			t.Fatal("should be idle")
		}

		// Can start again
		gen2 := g.TryStart()
		if gen2 == -1 {
			t.Fatal("should be able to start again")
		}
		if gen2 != gen+1 {
			t.Fatalf("generation should increment: %d != %d", gen2, gen+1)
		}
	})
}
