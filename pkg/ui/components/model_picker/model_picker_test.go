package model_picker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDefaultModelOptions(t *testing.T) {
	opts := DefaultModelOptions()
	if len(opts) < 3 {
		t.Fatalf("expected at least 3 options, got %d", len(opts))
	}
	// First should be "Default"
	if opts[0].Value != NoPreference {
		t.Errorf("first option should be NoPreference, got %q", opts[0].Value)
	}
	if !strings.Contains(opts[0].Label, "Default") {
		t.Errorf("first label = %q", opts[0].Label)
	}
}

func TestNew_CursorOnCurrent(t *testing.T) {
	opts := []ModelOption{
		{Value: "a", Label: "Model A"},
		{Value: "b", Label: "Model B"},
		{Value: "c", Label: "Model C"},
	}
	m := New(opts, "b")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (current = b)", m.cursor)
	}
}

func TestNew_CursorDefaultsToFirst(t *testing.T) {
	opts := []ModelOption{
		{Value: "a", Label: "Model A"},
		{Value: "b", Label: "Model B"},
	}
	m := New(opts, "nonexistent")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (fallback)", m.cursor)
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New([]ModelOption{
		{Value: "a", Label: "A"},
		{Value: "b", Label: "B"},
		{Value: "c", Label: "C"},
	}, "a")

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("after down: cursor = %d", m.cursor)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("after down+down: cursor = %d", m.cursor)
	}

	// Can't go past end
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Error("should not exceed bounds")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 1 {
		t.Errorf("after up: cursor = %d", m.cursor)
	}
}

func TestModel_VimKeys(t *testing.T) {
	m := New([]ModelOption{{Value: "a"}, {Value: "b"}}, "a")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Error("k should move up")
	}
}

func TestModel_Select(t *testing.T) {
	m := New([]ModelOption{
		{Value: "a", Label: "A"},
		{Value: "b", Label: "B"},
	}, "a")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // cursor on b

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return cmd")
	}
	msg := cmd()
	sel, ok := msg.(ModelSelectedMsg)
	if !ok {
		t.Fatalf("expected ModelSelectedMsg, got %T", msg)
	}
	if sel.Model != "b" {
		t.Errorf("selected = %q, want b", sel.Model)
	}
}

func TestModel_SelectWithEffort(t *testing.T) {
	m := New([]ModelOption{{Value: "a"}}, "a")
	m.SetEffort(EffortHigh)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(ModelSelectedMsg)
	if msg.Effort != EffortHigh {
		t.Errorf("effort = %q, want high", msg.Effort)
	}
}

func TestModel_Cancel(t *testing.T) {
	m := New([]ModelOption{{Value: "a"}}, "a")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(CancelledMsg); !ok {
		t.Fatalf("expected CancelledMsg, got %T", msg)
	}
}

func TestModel_View(t *testing.T) {
	m := New([]ModelOption{
		{Value: "", Label: "Default", Description: "Use default model"},
		{Value: "opus", Label: "Opus", Description: "Most capable"},
	}, "opus")
	v := m.View()

	if !strings.Contains(v, "Select model") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "Default") {
		t.Error("should show Default option")
	}
	if !strings.Contains(v, "Opus") {
		t.Error("should show Opus option")
	}
	if !strings.Contains(v, "(current)") {
		t.Error("should mark current model")
	}
	if !strings.Contains(v, "Most capable") {
		t.Error("should show description for cursor item (Opus)")
	}
}

func TestModel_ViewWithEffort(t *testing.T) {
	m := New([]ModelOption{{Value: "a", Label: "A"}}, "a")
	m.SetEffort(EffortMax)
	v := m.View()
	if !strings.Contains(v, "Effort") {
		t.Error("should show effort indicator")
	}
	if !strings.Contains(v, "max") {
		t.Error("should show effort level")
	}
}

func TestSelectedOption(t *testing.T) {
	m := New([]ModelOption{
		{Value: "a", Label: "A"},
		{Value: "b", Label: "B"},
	}, "a")
	sel := m.SelectedOption()
	if sel == nil || sel.Value != "a" {
		t.Error("should return current selected option")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	sel = m.SelectedOption()
	if sel == nil || sel.Value != "b" {
		t.Error("should return new selected option")
	}
}

func TestEffortSymbol(t *testing.T) {
	if effortSymbol(EffortLow) == "" {
		t.Error("low should have symbol")
	}
	if effortSymbol(EffortMax) == "" {
		t.Error("max should have symbol")
	}
	if effortSymbol("") != "" {
		t.Error("empty should have no symbol")
	}
}

func TestEffortLevel_Constants(t *testing.T) {
	if EffortLow != "low" {
		t.Error("wrong")
	}
	if EffortHigh != "high" {
		t.Error("wrong")
	}
}
