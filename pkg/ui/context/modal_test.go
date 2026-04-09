package context

import (
	"sort"
	"testing"
)

func TestModalState_AvailableSize_Active(t *testing.T) {
	m := ModalState{Active: true, Rows: 20, Columns: 60}
	rows, cols := m.AvailableSize(40, 120)
	if rows != 20 || cols != 60 {
		t.Errorf("active modal should use modal size, got %d×%d", rows, cols)
	}
}

func TestModalState_AvailableSize_Inactive(t *testing.T) {
	m := ModalState{} // zero value = inactive
	rows, cols := m.AvailableSize(40, 120)
	if rows != 40 || cols != 120 {
		t.Errorf("inactive modal should use fallback, got %d×%d", rows, cols)
	}
}

func TestModalState_ZeroValue(t *testing.T) {
	var m ModalState
	if m.Active {
		t.Error("zero value should not be active")
	}
}

func TestOverlayTracker_RegisterUnregister(t *testing.T) {
	tr := NewOverlayTracker()
	if tr.IsAnyActive() {
		t.Error("should start empty")
	}

	tr.Register("select")
	if !tr.IsAnyActive() {
		t.Error("should be active after register")
	}
	if tr.Count() != 1 {
		t.Errorf("count = %d, want 1", tr.Count())
	}

	tr.Register("multi-select")
	if tr.Count() != 2 {
		t.Errorf("count = %d, want 2", tr.Count())
	}

	tr.Unregister("select")
	if tr.Count() != 1 {
		t.Errorf("count = %d, want 1 after unregister", tr.Count())
	}

	tr.Unregister("multi-select")
	if tr.IsAnyActive() {
		t.Error("should be empty after unregistering all")
	}
}

func TestOverlayTracker_UnregisterNonexistent(t *testing.T) {
	tr := NewOverlayTracker()
	tr.Unregister("nonexistent") // should not panic
	if tr.Count() != 0 {
		t.Error("should still be empty")
	}
}

func TestOverlayTracker_DuplicateRegister(t *testing.T) {
	tr := NewOverlayTracker()
	tr.Register("select")
	tr.Register("select") // duplicate
	if tr.Count() != 1 {
		t.Errorf("duplicate register should not increase count, got %d", tr.Count())
	}
}

func TestOverlayTracker_IsModalActive(t *testing.T) {
	tr := NewOverlayTracker()

	// No overlays → not modal
	if tr.IsModalActive() {
		t.Error("should not be modal when empty")
	}

	// Autocomplete is non-modal
	tr.Register("autocomplete")
	if tr.IsModalActive() {
		t.Error("autocomplete should not be modal")
	}
	if !tr.IsAnyActive() {
		t.Error("autocomplete should still count as active overlay")
	}

	// Adding a modal overlay
	tr.Register("select")
	if !tr.IsModalActive() {
		t.Error("select should be modal")
	}

	// Remove modal, keep autocomplete
	tr.Unregister("select")
	if tr.IsModalActive() {
		t.Error("should not be modal with only autocomplete")
	}
}

func TestOverlayTracker_ActiveIDs(t *testing.T) {
	tr := NewOverlayTracker()
	tr.Register("b")
	tr.Register("a")
	tr.Register("c")

	ids := tr.ActiveIDs()
	sort.Strings(ids)
	if len(ids) != 3 || ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Errorf("ActiveIDs = %v", ids)
	}
}

func TestNonModalOverlays(t *testing.T) {
	if !NonModalOverlays["autocomplete"] {
		t.Error("autocomplete should be non-modal")
	}
	if NonModalOverlays["select"] {
		t.Error("select should not be non-modal")
	}
}
