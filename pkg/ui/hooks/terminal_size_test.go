package hooks

import (
	"sync"
	"testing"
)

func TestTerminalSizeTracker_InitialSize(t *testing.T) {
	tr := NewTerminalSizeTracker(80, 24)
	if tr.Width() != 80 {
		t.Fatalf("width = %d, want 80", tr.Width())
	}
	if tr.Height() != 24 {
		t.Fatalf("height = %d, want 24", tr.Height())
	}
	s := tr.Size()
	if s.Width != 80 || s.Height != 24 {
		t.Fatalf("size = %+v, want {80, 24}", s)
	}
}

func TestTerminalSizeTracker_UpdateChangesSize(t *testing.T) {
	tr := NewTerminalSizeTracker(80, 24)
	changed := tr.Update(120, 40)
	if !changed {
		t.Fatal("Update should return true when size changes")
	}
	if tr.Width() != 120 || tr.Height() != 40 {
		t.Fatalf("size = {%d, %d}, want {120, 40}", tr.Width(), tr.Height())
	}
}

func TestTerminalSizeTracker_UpdateNoChange(t *testing.T) {
	tr := NewTerminalSizeTracker(80, 24)
	changed := tr.Update(80, 24)
	if changed {
		t.Fatal("Update should return false when size unchanged")
	}
}

func TestTerminalSizeTracker_OnResizeCallback(t *testing.T) {
	tr := NewTerminalSizeTracker(80, 24)
	var got TerminalSize
	tr.OnResize(func(s TerminalSize) { got = s })

	tr.Update(100, 30)
	if got.Width != 100 || got.Height != 30 {
		t.Fatalf("callback got %+v, want {100, 30}", got)
	}
}

func TestTerminalSizeTracker_OnResizeNotCalledIfNoChange(t *testing.T) {
	tr := NewTerminalSizeTracker(80, 24)
	called := false
	tr.OnResize(func(TerminalSize) { called = true })

	tr.Update(80, 24)
	if called {
		t.Fatal("callback should not be called when size unchanged")
	}
}

func TestTerminalSizeTracker_Unsubscribe(t *testing.T) {
	tr := NewTerminalSizeTracker(80, 24)
	callCount := 0
	unsub := tr.OnResize(func(TerminalSize) { callCount++ })

	tr.Update(100, 30)
	if callCount != 1 {
		t.Fatalf("callCount = %d, want 1", callCount)
	}

	unsub()
	tr.Update(120, 40)
	if callCount != 1 {
		t.Fatalf("callCount = %d after unsub, want 1", callCount)
	}
}

func TestTerminalSizeTracker_MultipleListeners(t *testing.T) {
	tr := NewTerminalSizeTracker(80, 24)
	var counts [3]int

	for i := range counts {
		i := i
		tr.OnResize(func(TerminalSize) { counts[i]++ })
	}

	tr.Update(100, 30)
	for i, c := range counts {
		if c != 1 {
			t.Fatalf("listener %d: count = %d, want 1", i, c)
		}
	}
}

func TestTerminalSizeTracker_ConcurrentAccess(t *testing.T) {
	tr := NewTerminalSizeTracker(80, 24)
	tr.OnResize(func(TerminalSize) {})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(w, h int) {
			defer wg.Done()
			tr.Update(w, h)
			_ = tr.Size()
			_ = tr.Width()
			_ = tr.Height()
		}(80+i, 24+i)
	}
	wg.Wait()
}
