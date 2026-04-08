package keybindings

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func makeBindings() []ParsedBinding {
	return []ParsedBinding{
		// Single-keystroke bindings
		{Chord: Chord{ParseKeystroke("ctrl+t")}, Action: "app:tasks", Context: "Global"},
		{Chord: Chord{ParseKeystroke("ctrl+o")}, Action: "app:transcript", Context: "Global"},
		{Chord: Chord{ParseKeystroke("escape")}, Action: "theme:cancel", Context: "ThemePicker"},
		// Multi-keystroke chord binding
		{Chord: Chord{ParseKeystroke("ctrl+k"), ParseKeystroke("ctrl+s")}, Action: "app:save", Context: "Global"},
		{Chord: Chord{ParseKeystroke("ctrl+k"), ParseKeystroke("ctrl+d")}, Action: "app:diff", Context: "Global"},
	}
}

func TestChordResolver_SingleKeystroke(t *testing.T) {
	r := NewChordResolver(makeBindings())
	msg := tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}

	result := r.Resolve(msg, []Context{"Global"})
	if result.Type != ResolveMatch {
		t.Fatalf("expected Match, got %d", result.Type)
	}
	if result.Action != "app:tasks" {
		t.Errorf("action = %q, want app:tasks", result.Action)
	}
}

func TestChordResolver_NoMatch(t *testing.T) {
	r := NewChordResolver(makeBindings())
	msg := tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}

	result := r.Resolve(msg, []Context{"Global"})
	if result.Type != ResolveNone {
		t.Errorf("expected None, got %d", result.Type)
	}
}

func TestChordResolver_ContextFiltering(t *testing.T) {
	r := NewChordResolver(makeBindings())

	// escape with only Global → none (theme:cancel is ThemePicker only)
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	result := r.Resolve(esc, []Context{"Global"})
	if result.Type != ResolveNone {
		t.Errorf("expected None, got %d", result.Type)
	}

	// escape with ThemePicker → match
	result = r.Resolve(esc, []Context{"Global", "ThemePicker"})
	if result.Type != ResolveMatch || result.Action != "theme:cancel" {
		t.Errorf("expected Match(theme:cancel), got %d/%q", result.Type, result.Action)
	}
}

func TestChordResolver_ChordSequence(t *testing.T) {
	r := NewChordResolver(makeBindings())

	// First key: ctrl+k → should start chord
	k1 := tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	result := r.Resolve(k1, []Context{"Global"})
	if result.Type != ResolveChordStarted {
		t.Fatalf("expected ChordStarted, got %d", result.Type)
	}
	if len(result.Pending) != 1 {
		t.Fatalf("pending should have 1 keystroke, got %d", len(result.Pending))
	}

	// Second key: ctrl+s → should complete chord
	k2 := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	result = r.Resolve(k2, []Context{"Global"})
	if result.Type != ResolveMatch {
		t.Fatalf("expected Match, got %d", result.Type)
	}
	if result.Action != "app:save" {
		t.Errorf("action = %q, want app:save", result.Action)
	}

	// Pending should be cleared
	if r.Pending() != nil {
		t.Error("pending should be nil after match")
	}
}

func TestChordResolver_ChordCancelledByEscape(t *testing.T) {
	r := NewChordResolver(makeBindings())

	// Start chord
	k1 := tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	r.Resolve(k1, []Context{"Global"})

	// Cancel with escape
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	result := r.Resolve(esc, []Context{"Global"})
	if result.Type != ResolveChordCancelled {
		t.Errorf("expected ChordCancelled, got %d", result.Type)
	}
	if r.Pending() != nil {
		t.Error("pending should be cleared after cancel")
	}
}

func TestChordResolver_ChordCancelledByWrongKey(t *testing.T) {
	r := NewChordResolver(makeBindings())

	// Start chord with ctrl+k
	k1 := tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	r.Resolve(k1, []Context{"Global"})

	// Wrong second key: ctrl+x (not ctrl+s or ctrl+d)
	wrong := tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}
	result := r.Resolve(wrong, []Context{"Global"})
	if result.Type != ResolveChordCancelled {
		t.Errorf("expected ChordCancelled, got %d", result.Type)
	}
}

func TestKeystrokesEqual(t *testing.T) {
	a := ParseKeystroke("alt+f")
	b := ParseKeystroke("meta+f")
	if !keystrokesEqual(a, b) {
		t.Error("alt+f and meta+f should be equal (terminal conflates them)")
	}

	c := ParseKeystroke("ctrl+f")
	if keystrokesEqual(a, c) {
		t.Error("alt+f and ctrl+f should not be equal")
	}
}
