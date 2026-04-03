package core

import (
	"strings"
	"testing"
)

func TestNewKeyMap(t *testing.T) {
	km := NewKeyMap()
	if km == nil {
		t.Fatal("KeyMap should not be nil")
	}
	if len(km.Actions()) != 0 {
		t.Error("New KeyMap should have no actions")
	}
}

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()
	if km == nil {
		t.Fatal("DefaultKeyMap should not be nil")
	}
	// Should have navigation keys
	if !km.Has("next") {
		t.Error("DefaultKeyMap should have 'next' action")
	}
	if !km.Has("prev") {
		t.Error("DefaultKeyMap should have 'prev' action")
	}
	if !km.Has("submit") {
		t.Error("DefaultKeyMap should have 'submit' action")
	}
	if !km.Has("quit") {
		t.Error("DefaultKeyMap should have 'quit' action")
	}
}

func TestKeyMapSet(t *testing.T) {
	km := NewKeyMap()
	km.Set("test", "a", "b")
	keys := km.Get("test")
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("Expected [a, b], got %v", keys)
	}
}

func TestKeyMapGet(t *testing.T) {
	km := NewKeyMap()
	keys := km.Get("nonexistent")
	if keys != nil {
		t.Error("Get for nonexistent action should return nil")
	}
}

func TestKeyMapHas(t *testing.T) {
	km := NewKeyMap()
	if km.Has("test") {
		t.Error("Has should return false for nonexistent action")
	}
	km.Set("test", "x")
	if !km.Has("test") {
		t.Error("Has should return true after Set")
	}
}

func TestKeyMapLookup(t *testing.T) {
	km := NewKeyMap()
	km.Set("submit", "enter")
	km.Set("quit", "ctrl+c")

	action := km.Lookup("enter")
	if action != "submit" {
		t.Errorf("Expected 'submit', got %q", action)
	}

	action = km.Lookup("ctrl+c")
	if action != "quit" {
		t.Errorf("Expected 'quit', got %q", action)
	}

	action = km.Lookup("unknown")
	if action != "" {
		t.Errorf("Expected empty string for unknown key, got %q", action)
	}
}

func TestKeyMapLookupCaseInsensitive(t *testing.T) {
	km := NewKeyMap()
	km.Set("test", "Ctrl+C")
	action := km.Lookup("ctrl+c")
	if action != "test" {
		t.Errorf("Lookup should be case-insensitive, got %q", action)
	}
}

func TestKeyMapActions(t *testing.T) {
	km := NewKeyMap()
	km.Set("a", "1")
	km.Set("b", "2")
	km.Set("c", "3")
	actions := km.Actions()
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions))
	}
}

func TestKeyMapString(t *testing.T) {
	km := NewKeyMap()
	km.Set("quit", "ctrl+c")
	s := km.String()
	if !strings.Contains(s, "quit") || !strings.Contains(s, "ctrl+c") {
		t.Errorf("String should contain action and key, got %q", s)
	}
}

func TestKeyMapOverwrite(t *testing.T) {
	km := NewKeyMap()
	km.Set("test", "a")
	km.Set("test", "b", "c")
	keys := km.Get("test")
	if len(keys) != 2 {
		t.Error("Set should overwrite previous bindings")
	}
}

func TestDefaultKeyMapContents(t *testing.T) {
	km := DefaultKeyMap()
	// Verify specific bindings
	nextKeys := km.Get("next")
	if len(nextKeys) == 0 || nextKeys[0] != "tab" {
		t.Error("'next' should be bound to 'tab'")
	}

	quitKeys := km.Get("quit")
	if len(quitKeys) == 0 || quitKeys[0] != "ctrl+c" {
		t.Error("'quit' should be bound to 'ctrl+c'")
	}
}
