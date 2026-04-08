package keybindings

import (
	"sync/atomic"
	"testing"
)

func TestContextStack_PushPopLifecycle(t *testing.T) {
	s := NewContextStack()

	s.Push("Global")
	s.Push("Input")

	if !s.IsActive("Global") {
		t.Error("Global should be active")
	}
	if !s.IsActive("Input") {
		t.Error("Input should be active")
	}
	if s.IsActive("ThemePicker") {
		t.Error("ThemePicker should not be active")
	}

	active := s.ActiveContexts()
	if len(active) != 2 {
		t.Errorf("expected 2 active contexts, got %d", len(active))
	}

	s.Pop("Input")
	if s.IsActive("Input") {
		t.Error("Input should be inactive after pop")
	}
	if !s.IsActive("Global") {
		t.Error("Global should still be active")
	}
}

func TestContextStack_RefCounting(t *testing.T) {
	s := NewContextStack()

	// Two components both register "Global"
	s.Push("Global")
	s.Push("Global")

	if !s.IsActive("Global") {
		t.Error("Global should be active")
	}

	// First pop — still active (refcount=1)
	s.Pop("Global")
	if !s.IsActive("Global") {
		t.Error("Global should still be active (refcount=1)")
	}

	// Second pop — now inactive
	s.Pop("Global")
	if s.IsActive("Global") {
		t.Error("Global should be inactive (refcount=0)")
	}
}

func TestContextStack_RegisterAndInvokeHandler(t *testing.T) {
	s := NewContextStack()
	s.Push("Global")

	var called int32
	unregister := s.RegisterHandler(HandlerRegistration{
		Action:  "app:toggleTranscript",
		Context: "Global",
		Handler: func() { atomic.AddInt32(&called, 1) },
	})

	// Invoke — should call handler since Global is active
	if !s.InvokeAction("app:toggleTranscript") {
		t.Error("InvokeAction should return true")
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("handler called %d times, want 1", called)
	}

	// Unregister — invoke should return false
	unregister()
	if s.InvokeAction("app:toggleTranscript") {
		t.Error("InvokeAction should return false after unregister")
	}
}

func TestContextStack_InvokeOnlyActiveContexts(t *testing.T) {
	s := NewContextStack()
	s.Push("Global")
	// Note: "ThemePicker" is NOT pushed

	var globalCalled, themeCalled int32
	s.RegisterHandler(HandlerRegistration{
		Action:  "app:quit",
		Context: "Global",
		Handler: func() { atomic.AddInt32(&globalCalled, 1) },
	})
	s.RegisterHandler(HandlerRegistration{
		Action:  "app:quit",
		Context: "ThemePicker",
		Handler: func() { atomic.AddInt32(&themeCalled, 1) },
	})

	s.InvokeAction("app:quit")

	if atomic.LoadInt32(&globalCalled) != 1 {
		t.Error("Global handler should be called")
	}
	if atomic.LoadInt32(&themeCalled) != 0 {
		t.Error("ThemePicker handler should NOT be called (context inactive)")
	}
}

func TestContextStack_PopNonexistent(t *testing.T) {
	s := NewContextStack()
	// Should not panic
	s.Pop("DoesNotExist")
	if s.IsActive("DoesNotExist") {
		t.Error("should not be active")
	}
}
