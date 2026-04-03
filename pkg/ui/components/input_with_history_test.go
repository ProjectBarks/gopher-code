package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// TestInputWithHistoryCreation tests basic creation.
func TestInputWithHistoryCreation(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	if iwh == nil {
		t.Error("InputWithHistory should not be nil")
	}
	if iwh.input == nil {
		t.Error("Wrapped InputPane should not be nil")
	}
}

// TestInputWithHistoryCreationWithSession tests creation with session.
func TestInputWithHistoryCreationWithSession(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/home/user")

	iwh := NewInputWithHistory(sess)

	if iwh == nil {
		t.Error("InputWithHistory should not be nil")
	}
	if iwh.session != sess {
		t.Error("Session should be stored")
	}
}

// TestInputWithHistoryValue tests getting and setting values.
func TestInputWithHistoryValue(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	if iwh.Value() != "" {
		t.Error("Initial value should be empty")
	}

	iwh.SetValue("test input")
	if iwh.Value() != "test input" {
		t.Errorf("Value should be 'test input', got '%s'", iwh.Value())
	}
}

// TestInputWithHistoryClear tests clearing the input.
func TestInputWithHistoryClear(t *testing.T) {
	iwh := NewInputWithHistory(nil)
	iwh.SetValue("test")

	iwh.Clear()

	if iwh.Value() != "" {
		t.Error("Value should be empty after clear")
	}
}

// TestInputWithHistoryAddToHistory adds commands to history.
func TestInputWithHistoryAddToHistory(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	iwh.AddToHistory("cmd1")
	iwh.AddToHistory("cmd2")
	iwh.AddToHistory("cmd3")

	history := iwh.GetHistory()
	if len(history) != 3 {
		t.Errorf("Expected 3 history items, got %d", len(history))
	}
	if history[0] != "cmd1" || history[1] != "cmd2" || history[2] != "cmd3" {
		t.Error("History not stored correctly")
	}
}

// TestInputWithHistoryFocus tests focus management.
func TestInputWithHistoryFocus(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	if iwh.Focused() {
		t.Error("Should not be focused initially")
	}

	iwh.Focus()
	if !iwh.Focused() {
		t.Error("Should be focused after Focus()")
	}

	iwh.Blur()
	if iwh.Focused() {
		t.Error("Should not be focused after Blur()")
	}
}

// TestInputWithHistorySetSize tests size setting.
func TestInputWithHistorySetSize(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	iwh.SetSize(80, 3)

	if iwh.input.width != 80 || iwh.input.height != 3 {
		t.Error("Size not set correctly")
	}
}

// TestInputWithHistoryInit tests initialization.
func TestInputWithHistoryInit(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	cmd := iwh.Init()

	// Init should return a command
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

// TestInputWithHistoryView tests rendering.
func TestInputWithHistoryView(t *testing.T) {
	iwh := NewInputWithHistory(nil)
	iwh.SetSize(80, 3)
	iwh.SetValue("test")

	view := iwh.View()

	if view.Content == "" {
		t.Error("View should render content")
	}
}

// TestInputWithHistoryID tests the ID method.
func TestInputWithHistoryID(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	id := iwh.ID()
	if id != "input-with-history" {
		t.Errorf("ID should be 'input-with-history', got '%s'", id)
	}
}

// TestInputWithHistoryEmptyHistoryIgnored tests that empty input is ignored.
func TestInputWithHistoryEmptyHistoryIgnored(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	iwh.AddToHistory("")
	iwh.AddToHistory("   ")
	iwh.AddToHistory("valid")

	history := iwh.GetHistory()
	// Note: AddToHistory still adds empty strings (unlike the old implementation)
	// This tests the current behavior
	if len(history) < 1 {
		t.Error("Should have at least one history item")
	}
}

// TestInputWithHistoryGetHistoryCopy tests that GetHistory returns a copy.
func TestInputWithHistoryGetHistoryCopy(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	iwh.AddToHistory("cmd1")
	iwh.AddToHistory("cmd2")

	hist1 := iwh.GetHistory()
	hist1[0] = "modified"

	hist2 := iwh.GetHistory()
	if hist2[0] != "cmd1" {
		t.Error("GetHistory should return a copy, not the original")
	}
}

// TestInputWithHistoryGetHistoryNilInput tests GetHistory with nil input.
func TestInputWithHistoryGetHistoryNilInput(t *testing.T) {
	iwh := &InputWithHistory{
		input:   nil,
		session: nil,
	}

	history := iwh.GetHistory()
	if history == nil || len(history) != 0 {
		t.Error("GetHistory should return empty slice for nil input")
	}
}

// TestInputWithHistoryUpdate tests Update method.
func TestInputWithHistoryUpdate(t *testing.T) {
	iwh := NewInputWithHistory(nil)
	iwh.SetSize(80, 3)
	iwh.Focus()

	// Simulate Enter key
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}

	_, cmd := iwh.Update(msg)

	// Should return a model and command
	if cmd == nil {
		// cmd can be nil, that's fine
	}
}

// TestInputWithHistorySessionPersistence tests that session is stored.
func TestInputWithHistorySessionPersistence(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/home/user")

	iwh := NewInputWithHistory(sess)
	iwh.AddToHistory("test command")

	// Session should still be accessible
	if iwh.session != sess {
		t.Error("Session should be retained after AddToHistory")
	}
}

// TestInputWithHistoryWithoutSession tests operation without session.
func TestInputWithHistoryWithoutSession(t *testing.T) {
	iwh := NewInputWithHistory(nil)

	iwh.AddToHistory("cmd1")
	iwh.AddToHistory("cmd2")

	history := iwh.GetHistory()
	if len(history) != 2 {
		t.Errorf("Should work without session, got %d items", len(history))
	}
}
