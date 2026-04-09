package selectcomp

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testOptions() []Option {
	return []Option{
		{Label: "Read", Value: "read", Description: "Read files"},
		{Label: "Write", Value: "write", Description: "Write files"},
		{Label: "Edit", Value: "edit", Description: "Edit files"},
		{Label: "Bash", Value: "bash", Description: "Run commands"},
		{Label: "Grep", Value: "grep", Description: "Search content"},
	}
}

func TestSelect_Navigation(t *testing.T) {
	m := New("Pick a tool", testOptions())
	if m.selected != 0 {
		t.Error("should start at 0")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selected != 1 {
		t.Error("down should move to 1")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.selected != 0 {
		t.Error("up should move back to 0")
	}
}

func TestSelect_Enter(t *testing.T) {
	m := New("", testOptions())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should return cmd")
	}
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("expected SelectMsg, got %T", msg)
	}
	if sel.Value != "read" {
		t.Errorf("value = %q, want read", sel.Value)
	}
}

func TestSelect_Cancel(t *testing.T) {
	m := New("", testOptions())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("escape should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(CancelMsg); !ok {
		t.Fatalf("expected CancelMsg, got %T", msg)
	}
}

func TestSelect_Filter(t *testing.T) {
	m := New("", testOptions())

	// Enter filter mode
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.filtering {
		t.Fatal("/ should enter filter mode")
	}

	// Type filter text
	m, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	if m.OptionCount() != 1 {
		t.Errorf("filter 'ba' should match 1 option (Bash), got %d", m.OptionCount())
	}

	// Clear filter with Escape
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.filtering {
		t.Error("escape should exit filter mode")
	}
	if m.OptionCount() != 5 {
		t.Errorf("after clear, should show all 5 options, got %d", m.OptionCount())
	}
}

func TestSelect_DisabledOption(t *testing.T) {
	opts := []Option{
		{Label: "Active", Value: "a"},
		{Label: "Disabled", Value: "d", Disabled: true},
	}
	m := New("", opts)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // move to disabled
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	// Should not select disabled option
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(SelectMsg); ok {
			t.Error("should not select disabled option")
		}
	}
}

func TestSelect_SelectedOption(t *testing.T) {
	m := New("", testOptions())
	opt := m.SelectedOption()
	if opt == nil {
		t.Fatal("should return selected option")
	}
	if opt.Value != "read" {
		t.Errorf("value = %q, want read", opt.Value)
	}
}

func TestSelect_View(t *testing.T) {
	m := New("Choose", testOptions())
	v := m.View()
	if !strings.Contains(v, "Choose") {
		t.Error("should contain title")
	}
	if !strings.Contains(v, "Read") {
		t.Error("should contain first option")
	}
}

func TestSelect_EmptyOptions(t *testing.T) {
	m := New("Empty", nil)
	v := m.View()
	if !strings.Contains(v, "No matching") {
		t.Error("should show no options message")
	}
}
