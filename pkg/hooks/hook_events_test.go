package hooks

import (
	"sync"
	"testing"
)

// Source: utils/hooks/hookEvents.ts

func TestAlwaysEmittedHookEvents(t *testing.T) {
	// Source: hookEvents.ts:18
	if len(AlwaysEmittedHookEvents) != 2 {
		t.Errorf("expected 2 always-emitted events, got %d", len(AlwaysEmittedHookEvents))
	}
	if AlwaysEmittedHookEvents[0] != SessionStart {
		t.Errorf("expected SessionStart, got %s", AlwaysEmittedHookEvents[0])
	}
	if AlwaysEmittedHookEvents[1] != Setup {
		t.Errorf("expected Setup, got %s", AlwaysEmittedHookEvents[1])
	}
}

func TestMaxPendingEvents(t *testing.T) {
	// Source: hookEvents.ts:20
	if MaxPendingEvents != 100 {
		t.Errorf("expected 100, got %d", MaxPendingEvents)
	}
}

func TestHookEventEmitter_QueueBeforeHandler(t *testing.T) {
	// Source: hookEvents.ts:72-80 — events queue before handler registration
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	e.EmitHookStarted("h1", "test-hook", "PreToolUse")
	e.EmitHookStarted("h2", "test-hook-2", "PostToolUse")

	if e.PendingCount() != 2 {
		t.Errorf("expected 2 pending, got %d", e.PendingCount())
	}
}

func TestHookEventEmitter_FlushOnRegister(t *testing.T) {
	// Source: hookEvents.ts:63-69 — flush pending events on handler registration
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	e.EmitHookStarted("h1", "hook-a", "PreToolUse")
	e.EmitHookStarted("h2", "hook-b", "PostToolUse")

	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	if len(received) != 2 {
		t.Errorf("expected 2 flushed events, got %d", len(received))
	}
	if e.PendingCount() != 0 {
		t.Errorf("expected 0 pending after flush, got %d", e.PendingCount())
	}
}

func TestHookEventEmitter_DirectDelivery(t *testing.T) {
	// Source: hookEvents.ts:73-74 — deliver directly when handler registered
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	e.EmitHookStarted("h1", "hook", "PreToolUse")

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Started == nil {
		t.Fatal("expected Started event")
	}
	if received[0].Started.HookID != "h1" {
		t.Errorf("hookId = %q", received[0].Started.HookID)
	}
	if received[0].Started.Type != "started" {
		t.Errorf("type = %q", received[0].Started.Type)
	}
}

func TestHookEventEmitter_MaxPendingEviction(t *testing.T) {
	// Source: hookEvents.ts:77-79 — evict oldest when over max
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	for i := 0; i < MaxPendingEvents+10; i++ {
		e.EmitHookStarted(itoa(i), "hook", "PreToolUse")
	}

	if e.PendingCount() != MaxPendingEvents {
		t.Errorf("expected %d pending (evicted oldest), got %d", MaxPendingEvents, e.PendingCount())
	}

	// Verify the oldest events were dropped
	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	if len(received) != MaxPendingEvents {
		t.Fatalf("expected %d flushed, got %d", MaxPendingEvents, len(received))
	}
	// First received should be event #10 (0-9 were evicted)
	if received[0].Started.HookID != "10" {
		t.Errorf("expected first flushed event to have hookId '10', got %q", received[0].Started.HookID)
	}
}

func TestHookEventEmitter_AlwaysEmittedEvents(t *testing.T) {
	// Source: hookEvents.ts:83-91 — SessionStart and Setup always emitted
	e := NewHookEventEmitter()
	// allHookEventsEnabled = false (default)

	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	e.EmitHookStarted("h1", "hook", "SessionStart") // always emitted
	e.EmitHookStarted("h2", "hook", "Setup")         // always emitted
	e.EmitHookStarted("h3", "hook", "PreToolUse")    // NOT emitted (not always-emitted + disabled)

	if len(received) != 2 {
		t.Errorf("expected 2 events (only always-emitted), got %d", len(received))
	}
}

func TestHookEventEmitter_AllEventsEnabled(t *testing.T) {
	// Source: hookEvents.ts:184-186 — enable all hook events
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	e.EmitHookStarted("h1", "hook", "PreToolUse")
	e.EmitHookStarted("h2", "hook", "Stop")

	if len(received) != 2 {
		t.Errorf("expected 2 events when all enabled, got %d", len(received))
	}
}

func TestHookEventEmitter_InvalidEventNotEmitted(t *testing.T) {
	// Source: hookEvents.ts:88-90 — invalid events not emitted even when enabled
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	e.EmitHookStarted("h1", "hook", "NotAHookEvent")

	if len(received) != 0 {
		t.Errorf("expected 0 events for invalid event, got %d", len(received))
	}
}

func TestHookEventEmitter_ProgressEvent(t *testing.T) {
	// Source: hookEvents.ts:108-122
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	e.EmitHookProgress("h1", "hook", "PreToolUse", "out", "err", "combined")

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Progress == nil {
		t.Fatal("expected Progress event")
	}
	p := received[0].Progress
	if p.Type != "progress" {
		t.Errorf("type = %q", p.Type)
	}
	if p.Stdout != "out" {
		t.Errorf("stdout = %q", p.Stdout)
	}
	if p.Stderr != "err" {
		t.Errorf("stderr = %q", p.Stderr)
	}
	if p.Output != "combined" {
		t.Errorf("output = %q", p.Output)
	}
}

func TestHookEventEmitter_ResponseEvent(t *testing.T) {
	// Source: hookEvents.ts:153-177
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	exitCode := 0
	e.EmitHookResponse("h1", "hook", "PostToolUse", "output", "stdout", "stderr", &exitCode, "success")

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Response == nil {
		t.Fatal("expected Response event")
	}
	r := received[0].Response
	if r.Type != "response" {
		t.Errorf("type = %q", r.Type)
	}
	if r.Outcome != "success" {
		t.Errorf("outcome = %q", r.Outcome)
	}
	if r.ExitCode == nil || *r.ExitCode != 0 {
		t.Errorf("exitCode wrong")
	}
}

func TestHookEventEmitter_Clear(t *testing.T) {
	// Source: hookEvents.ts:188-192
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	e.EmitHookStarted("h1", "hook", "PreToolUse")
	e.RegisterHandler(func(ev HookExecutionEvent) {})
	e.SetAllHookEventsEnabled(true)

	e.Clear()

	if e.PendingCount() != 0 {
		t.Errorf("expected 0 pending after clear")
	}

	// After clear, events should queue again (no handler)
	e.EmitHookStarted("h2", "hook", "SessionStart") // always-emitted
	if e.PendingCount() != 1 {
		t.Errorf("expected 1 pending after clear + emit, got %d", e.PendingCount())
	}
}

func TestHookEventEmitter_ConcurrentSafety(t *testing.T) {
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	var mu sync.Mutex
	count := 0
	e.RegisterHandler(func(ev HookExecutionEvent) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			e.EmitHookStarted(itoa(n), "hook", "PreToolUse")
		}(i)
	}
	wg.Wait()

	mu.Lock()
	if count != 50 {
		t.Errorf("expected 50 events, got %d", count)
	}
	mu.Unlock()
}

func TestHookEventEmitter_NilExitCode(t *testing.T) {
	// Source: hookEvents.ts:48 — exitCode is optional
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	var received []HookExecutionEvent
	e.RegisterHandler(func(ev HookExecutionEvent) {
		received = append(received, ev)
	})

	e.EmitHookResponse("h1", "hook", "Stop", "", "", "", nil, "cancelled")

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Response.ExitCode != nil {
		t.Error("expected nil exitCode")
	}
	if received[0].Response.Outcome != "cancelled" {
		t.Errorf("outcome = %q", received[0].Response.Outcome)
	}
}

func TestHookEventEmitter_UnregisterHandler(t *testing.T) {
	// Source: hookEvents.ts:62 — pass nil to unregister
	e := NewHookEventEmitter()
	e.SetAllHookEventsEnabled(true)

	called := 0
	e.RegisterHandler(func(ev HookExecutionEvent) {
		called++
	})

	e.EmitHookStarted("h1", "hook", "PreToolUse")
	if called != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}

	// Unregister
	e.RegisterHandler(nil)
	e.EmitHookStarted("h2", "hook", "PreToolUse")

	// Should not have been called again — event should be queued
	if called != 1 {
		t.Errorf("expected still 1 call after unregister, got %d", called)
	}
	if e.PendingCount() != 1 {
		t.Errorf("expected 1 pending after unregister + emit, got %d", e.PendingCount())
	}
}
