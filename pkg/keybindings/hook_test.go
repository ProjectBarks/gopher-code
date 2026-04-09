package keybindings

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestKeybindingHandler_Handle(t *testing.T) {
	// Set up bindings: ctrl+s → "app:save" in Global context
	bindings := []ParsedBinding{
		{
			Chord:   []ParsedKeystroke{{Key: "s", Ctrl: true}},
			Action:  "app:save",
			Context: ContextGlobal,
		},
	}

	resolver := NewChordResolver(bindings)
	stack := NewContextStack()
	stack.Push(ContextGlobal)

	saveCalled := false
	stack.RegisterHandler(HandlerRegistration{
		Action:  "app:save",
		Context: ContextGlobal,
		Handler: func() { saveCalled = true },
	})

	handler := NewKeybindingHandler(resolver, stack)

	// Ctrl+S should match and invoke the handler
	result := handler.Handle(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if result != HandleMatched {
		t.Errorf("expected HandleMatched, got %d", result)
	}
	if !saveCalled {
		t.Error("save handler was not invoked")
	}
}

func TestKeybindingHandler_NoMatch(t *testing.T) {
	bindings := []ParsedBinding{
		{
			Chord:   []ParsedKeystroke{{Key: "s", Ctrl: true}},
			Action:  "app:save",
			Context: ContextGlobal,
		},
	}

	resolver := NewChordResolver(bindings)
	stack := NewContextStack()
	stack.Push(ContextGlobal)

	handler := NewKeybindingHandler(resolver, stack)

	// Regular 'a' should not match
	result := handler.Handle(tea.KeyPressMsg{Code: 'a'})
	if result != HandleNone {
		t.Errorf("expected HandleNone, got %d", result)
	}
}

func TestKeybindingHandler_ChordSequence(t *testing.T) {
	// ctrl+k ctrl+s → "app:saveAll" (two-key chord)
	bindings := []ParsedBinding{
		{
			Chord: []ParsedKeystroke{
				{Key: "k", Ctrl: true},
				{Key: "s", Ctrl: true},
			},
			Action:  "app:saveAll",
			Context: ContextGlobal,
		},
	}

	resolver := NewChordResolver(bindings)
	stack := NewContextStack()
	stack.Push(ContextGlobal)

	saveAllCalled := false
	stack.RegisterHandler(HandlerRegistration{
		Action:  "app:saveAll",
		Context: ContextGlobal,
		Handler: func() { saveAllCalled = true },
	})

	handler := NewKeybindingHandler(resolver, stack)

	// First key: chord started
	r1 := handler.Handle(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if r1 != HandleChordStarted {
		t.Errorf("expected HandleChordStarted, got %d", r1)
	}
	if handler.PendingChord() == nil {
		t.Error("should have pending chord")
	}

	// Second key: chord completed
	r2 := handler.Handle(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if r2 != HandleMatched {
		t.Errorf("expected HandleMatched, got %d", r2)
	}
	if !saveAllCalled {
		t.Error("saveAll handler was not invoked")
	}
	if handler.PendingChord() != nil {
		t.Error("pending chord should be cleared after match")
	}
}

func TestKeybindingHandler_ChordCancelled(t *testing.T) {
	bindings := []ParsedBinding{
		{
			Chord: []ParsedKeystroke{
				{Key: "k", Ctrl: true},
				{Key: "s", Ctrl: true},
			},
			Action:  "app:saveAll",
			Context: ContextGlobal,
		},
	}

	resolver := NewChordResolver(bindings)
	stack := NewContextStack()
	stack.Push(ContextGlobal)

	handler := NewKeybindingHandler(resolver, stack)

	// Start chord
	handler.Handle(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})

	// Cancel with escape
	r := handler.Handle(tea.KeyPressMsg{Code: tea.KeyEscape})
	if r != HandleChordCancelled {
		t.Errorf("expected HandleChordCancelled, got %d", r)
	}
}

func TestKeybindingHandler_InactiveContext(t *testing.T) {
	bindings := []ParsedBinding{
		{
			Chord:   []ParsedKeystroke{{Key: "s", Ctrl: true}},
			Action:  "dialog:close",
			Context: "Dialog",
		},
	}

	resolver := NewChordResolver(bindings)
	stack := NewContextStack()
	stack.Push(ContextGlobal) // Dialog is NOT active

	handler := NewKeybindingHandler(resolver, stack)

	// Should not match because Dialog context is not active
	result := handler.Handle(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if result != HandleNone {
		t.Errorf("expected HandleNone (context inactive), got %d", result)
	}
}

func TestKeybindingHandler_ClearPending(t *testing.T) {
	bindings := []ParsedBinding{
		{
			Chord: []ParsedKeystroke{
				{Key: "k", Ctrl: true},
				{Key: "s", Ctrl: true},
			},
			Action:  "app:saveAll",
			Context: ContextGlobal,
		},
	}

	resolver := NewChordResolver(bindings)
	stack := NewContextStack()
	stack.Push(ContextGlobal)

	handler := NewKeybindingHandler(resolver, stack)

	// Start chord
	handler.Handle(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if handler.PendingChord() == nil {
		t.Fatal("should have pending chord")
	}

	// Programmatic clear
	handler.ClearPendingChord()
	if handler.PendingChord() != nil {
		t.Error("pending chord should be nil after ClearPendingChord")
	}
}
