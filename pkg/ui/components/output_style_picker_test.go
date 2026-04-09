package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOutputStylePicker_HasOptions(t *testing.T) {
	m := NewOutputStylePicker(t.TempDir())
	if len(m.styles) == 0 {
		t.Fatal("should have at least built-in styles")
	}
	// Default should be first
	if m.styles[0].name != "Default" {
		t.Errorf("first style = %q, want Default", m.styles[0].name)
	}
}

func TestOutputStylePicker_Navigation(t *testing.T) {
	m := NewOutputStylePicker(t.TempDir())
	if m.selected != 0 {
		t.Error("should start at 0")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selected != 1 {
		t.Errorf("after down: selected = %d, want 1", m.selected)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.selected != 0 {
		t.Errorf("after up: selected = %d, want 0", m.selected)
	}
}

func TestOutputStylePicker_Select(t *testing.T) {
	m := NewOutputStylePicker(t.TempDir())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should return a cmd")
	}
	msg := cmd()
	sel, ok := msg.(OutputStyleSelectedMsg)
	if !ok {
		t.Fatalf("expected OutputStyleSelectedMsg, got %T", msg)
	}
	if sel.StyleName != "Default" {
		t.Errorf("selected = %q, want Default", sel.StyleName)
	}
}

func TestOutputStylePicker_Cancel(t *testing.T) {
	m := NewOutputStylePicker(t.TempDir())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("escape should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(OutputStyleCanceledMsg); !ok {
		t.Fatalf("expected OutputStyleCanceledMsg, got %T", msg)
	}
}

func TestOutputStylePicker_View(t *testing.T) {
	m := NewOutputStylePicker(t.TempDir())
	v := m.View()
	if !strings.Contains(v, "output style") {
		t.Error("view should contain title")
	}
	if !strings.Contains(v, "Default") {
		t.Error("view should show Default option")
	}
}
