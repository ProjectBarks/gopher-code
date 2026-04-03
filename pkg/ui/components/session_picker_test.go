package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func testSessions() []SessionInfo {
	return []SessionInfo{
		{ID: "sess-1", Name: "Debug auth", Model: "opus", MessageCount: 10},
		{ID: "sess-2", Name: "Fix tests", Model: "sonnet", MessageCount: 5},
		{ID: "sess-3", Name: "Refactor UI", Model: "opus", MessageCount: 20},
	}
}

func TestSessionPickerCreation(t *testing.T) {
	sp := NewSessionPicker(theme.Current())
	if sp == nil {
		t.Fatal("SessionPicker should not be nil")
	}
}

func TestSessionPickerSetSessions(t *testing.T) {
	sp := NewSessionPicker(theme.Current())
	sp.SetSessions(testSessions())
	if len(sp.filtered) != 3 {
		t.Errorf("Expected 3 filtered sessions, got %d", len(sp.filtered))
	}
}

func TestSessionPickerSearch(t *testing.T) {
	sp := NewSessionPicker(theme.Current())
	sp.SetSessions(testSessions())
	// Type "auth"
	sp.Update(tea.KeyPressMsg{Text: "a"})
	sp.Update(tea.KeyPressMsg{Text: "u"})
	sp.Update(tea.KeyPressMsg{Text: "t"})
	sp.Update(tea.KeyPressMsg{Text: "h"})
	if len(sp.filtered) != 1 {
		t.Errorf("Expected 1 filtered session for 'auth', got %d", len(sp.filtered))
	}
}

func TestSessionPickerBackspace(t *testing.T) {
	sp := NewSessionPicker(theme.Current())
	sp.SetSessions(testSessions())
	sp.Update(tea.KeyPressMsg{Text: "x"})
	sp.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if sp.SearchText() != "" {
		t.Error("Backspace should clear search")
	}
	if len(sp.filtered) != 3 {
		t.Error("All sessions should show after clearing search")
	}
}

func TestSessionPickerNavigation(t *testing.T) {
	sp := NewSessionPicker(theme.Current())
	sp.SetSessions(testSessions())
	sp.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	selected := sp.Selected()
	if selected == nil || selected.ID != "sess-2" {
		t.Error("Down should select second session")
	}
}

func TestSessionPickerView(t *testing.T) {
	sp := NewSessionPicker(theme.Current())
	sp.SetSessions(testSessions())
	sp.SetSize(80, 20)
	view := sp.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "Select Session") {
		t.Error("Expected title")
	}
	if !strings.Contains(plain, "Debug auth") {
		t.Error("Expected session name")
	}
}

func TestSessionPickerEmptySessions(t *testing.T) {
	sp := NewSessionPicker(theme.Current())
	sp.SetSize(80, 20)
	view := sp.View()
	plain := stripANSI(view.Content)
	if !strings.Contains(plain, "No sessions") {
		t.Error("Expected 'No sessions' message")
	}
}
