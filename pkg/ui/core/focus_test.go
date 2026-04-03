package core

import (
	"testing"
)

// TestFocusManagerBasic tests focus initialization.
func TestFocusManagerBasic(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	c3 := &MockComponent{}

	fm := NewFocusManager(c1, c2, c3)

	if fm.Focused() != c1 {
		t.Error("FM should return c1 as focused")
	}
	if !c1.Focused() {
		t.Error("First child should be focused initially")
	}
	if c2.Focused() || c3.Focused() {
		t.Error("Other children should not be focused")
	}
}

// TestFocusManagerNext tests focus cycling.
func TestFocusManagerNext(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	c3 := &MockComponent{}

	fm := NewFocusManager(c1, c2, c3)

	// Initial: c1 focused
	if !c1.Focused() {
		t.Error("c1 should be focused")
	}

	// Move to c2
	fm.Next()
	if c1.Focused() || !c2.Focused() {
		t.Error("c2 should be focused after Next")
	}

	// Move to c3
	fm.Next()
	if c2.Focused() || !c3.Focused() {
		t.Error("c3 should be focused after Next")
	}

	// Wrap to c1
	fm.Next()
	if c3.Focused() || !c1.Focused() {
		t.Error("c1 should be focused after wrap")
	}
}

// TestFocusManagerPrev tests reverse focus cycling.
func TestFocusManagerPrev(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	c3 := &MockComponent{}

	fm := NewFocusManager(c1, c2, c3)
	fm.Next()
	fm.Next() // Now at c3

	// Move back to c2
	fm.Prev()
	if c3.Focused() || !c2.Focused() {
		t.Error("c2 should be focused after Prev")
	}

	// Move back to c1
	fm.Prev()
	if c2.Focused() || !c1.Focused() {
		t.Error("c1 should be focused after Prev")
	}

	// Wrap to c3
	fm.Prev()
	if c1.Focused() || !c3.Focused() {
		t.Error("c3 should be focused after wrap")
	}
}

// TestFocusManagerModal tests modal override.
func TestFocusManagerModal(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	modal := &MockComponent{}

	fm := NewFocusManager(c1, c2)

	if !c1.Focused() {
		t.Error("c1 should be focused")
	}

	// Push modal
	fm.PushModal(modal)
	if c1.Focused() || !modal.Focused() {
		t.Error("Modal should be focused after PushModal")
	}

	// Modal active, Next should not cycle
	fm.Next()
	if !modal.Focused() {
		t.Error("Modal should remain focused during Next")
	}

	// Pop modal
	fm.PopModal()
	if modal.Focused() || !c1.Focused() {
		t.Error("c1 should be focused after PopModal")
	}
}

// TestFocusManagerModalStack tests nested modals.
func TestFocusManagerModalStack(t *testing.T) {
	c1 := &MockComponent{}
	modal1 := &MockComponent{}
	modal2 := &MockComponent{}

	fm := NewFocusManager(c1)

	fm.PushModal(modal1)
	if !modal1.Focused() {
		t.Error("modal1 should be focused")
	}

	fm.PushModal(modal2)
	if modal1.Focused() || !modal2.Focused() {
		t.Error("modal2 should be focused")
	}

	fm.PopModal()
	if modal2.Focused() || !modal1.Focused() {
		t.Error("modal1 should be focused after popping modal2")
	}

	fm.PopModal()
	if modal1.Focused() || !c1.Focused() {
		t.Error("c1 should be focused after popping all modals")
	}
}

// TestFocusManagerFocused returns the current focused element.
func TestFocusManagerFocused(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}
	modal := &MockComponent{}

	fm := NewFocusManager(c1, c2)

	// Regular focus
	focused := fm.Focused()
	if focused != c1 {
		t.Error("Focused should return c1")
	}

	fm.Next()
	focused = fm.Focused()
	if focused != c2 {
		t.Error("Focused should return c2")
	}

	// Modal focus
	fm.PushModal(modal)
	focused = fm.Focused()
	if focused != modal {
		t.Error("Focused should return modal")
	}
}

// TestFocusManagerRoute routes messages correctly.
func TestFocusManagerRoute(t *testing.T) {
	c1 := &MockComponent{}
	c2 := &MockComponent{}

	fm := NewFocusManager(c1, c2)
	fm.Route(nil)

	if c1.updates != 1 {
		t.Error("Message should be routed to focused component")
	}

	fm.Next()
	fm.Route(nil)

	if c2.updates != 1 {
		t.Error("Message should be routed to new focused component")
	}
}

// TestFocusManagerEmpty handles empty manager gracefully.
func TestFocusManagerEmpty(t *testing.T) {
	fm := &FocusManager{
		children:   []Focusable{},
		modalStack: []Focusable{},
	}

	// Should not panic
	fm.Next()
	fm.Prev()
	fm.Route(nil)

	if fm.Focused() != nil {
		t.Error("Focused should return nil when empty")
	}
}

// TestFocusManagerAdd tests adding children.
func TestFocusManagerAdd(t *testing.T) {
	fm := NewFocusManager()
	c1 := &MockComponent{}

	fm.Add(c1)

	if !c1.Focused() {
		t.Error("First added child should be focused")
	}

	c2 := &MockComponent{}
	fm.Add(c2)

	if !c1.Focused() {
		t.Error("c1 should remain focused when c2 is added")
	}
}
