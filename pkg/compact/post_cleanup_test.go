package compact

import "testing"

// Source: services/compact/postCompactCleanup.ts

func TestIsMainThreadCompact_EmptyIsMainThread(t *testing.T) {
	// undefined querySource = main thread (for /compact, /clear)
	if !IsMainThreadCompact("") {
		t.Error("empty querySource should be main thread")
	}
}

func TestIsMainThreadCompact_SDK(t *testing.T) {
	if !IsMainThreadCompact("sdk") {
		t.Error("sdk should be main thread")
	}
}

func TestIsMainThreadCompact_ReplMainThread(t *testing.T) {
	if !IsMainThreadCompact("repl_main_thread") {
		t.Error("repl_main_thread should be main thread")
	}
	if !IsMainThreadCompact("repl_main_thread_foo") {
		t.Error("repl_main_thread_foo should be main thread (startsWith)")
	}
}

func TestIsMainThreadCompact_SubagentIsNot(t *testing.T) {
	if IsMainThreadCompact("agent:sub1") {
		t.Error("agent:sub1 should NOT be main thread")
	}
	if IsMainThreadCompact("agent:deep:nested") {
		t.Error("agent:deep:nested should NOT be main thread")
	}
}

func TestPostCompactCleaner_AlwaysRunsForAll(t *testing.T) {
	c := NewPostCompactCleaner()
	called := false
	c.RegisterAlways(func() { called = true })

	c.Run("agent:sub1") // subagent
	if !called {
		t.Error("always-cleanup should run for subagent compact")
	}
}

func TestPostCompactCleaner_MainOnlySkippedForSubagent(t *testing.T) {
	c := NewPostCompactCleaner()
	mainCalled := false
	c.RegisterMainOnly(func() { mainCalled = true })

	c.Run("agent:sub1")
	if mainCalled {
		t.Error("main-only cleanup should NOT run for subagent compact")
	}
}

func TestPostCompactCleaner_MainOnlyRunsForMainThread(t *testing.T) {
	c := NewPostCompactCleaner()
	mainCalled := false
	c.RegisterMainOnly(func() { mainCalled = true })

	c.Run("") // empty = main thread
	if !mainCalled {
		t.Error("main-only cleanup should run for main thread compact")
	}
}

func TestPostCompactCleaner_OrderPreserved(t *testing.T) {
	c := NewPostCompactCleaner()
	var order []int
	c.RegisterAlways(func() { order = append(order, 1) })
	c.RegisterAlways(func() { order = append(order, 2) })
	c.RegisterMainOnly(func() { order = append(order, 3) })
	c.RegisterMainOnly(func() { order = append(order, 4) })

	c.Run("repl_main_thread")
	if len(order) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(order))
	}
	// Always runs first, then mainOnly.
	expected := []int{1, 2, 3, 4}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %d, want %d", i, order[i], v)
		}
	}
}
