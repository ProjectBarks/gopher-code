package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestCommandPaletteCreation(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	if cp == nil {
		t.Fatal("CommandPalette should not be nil")
	}

	if cp.visible {
		t.Error("Palette should be hidden initially")
	}

	if cp.CommandCount() != 0 {
		t.Error("Palette should start with no commands")
	}

	if cp.searchText != "" {
		t.Error("Search should be empty initially")
	}
}

func TestCommandPaletteAddCommand(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Test Command", "A test command", "test", func() {})

	if cp.CommandCount() != 1 {
		t.Errorf("Expected 1 command, got %d", cp.CommandCount())
	}

	cmd := cp.GetCommand("cmd-1")
	if cmd == nil || cmd.Title != "Test Command" {
		t.Error("Command should be added")
	}
}

func TestCommandPaletteRemoveCommand(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "First", "", "test", nil)
	cp.AddCommand("cmd-2", "Second", "", "test", nil)

	success := cp.RemoveCommand("cmd-1")
	if !success {
		t.Error("RemoveCommand should return true")
	}

	if cp.CommandCount() != 1 {
		t.Errorf("Expected 1 command, got %d", cp.CommandCount())
	}

	if cp.GetCommand("cmd-1") != nil {
		t.Error("Removed command should not exist")
	}
}

func TestCommandPaletteOpen(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	if cp.IsOpen() {
		t.Error("Palette should be closed initially")
	}

	cp.Open()
	if !cp.IsOpen() {
		t.Error("Palette should be open after Open()")
	}
}

func TestCommandPaletteClose(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.Open()
	cp.Close()

	if cp.IsOpen() {
		t.Error("Palette should be closed after Close()")
	}
}

func TestCommandPaletteSearch(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Help", "Show help", "docs", nil)
	cp.AddCommand("cmd-2", "History", "Command history", "nav", nil)
	cp.AddCommand("cmd-3", "Hide", "Hide panel", "view", nil)

	cp.Open()
	cp.SetSearchText("h")

	// Should match "Help", "History", and "Hide"
	if cp.FilteredCount() != 3 {
		t.Errorf("Expected 3 matches for 'h', got %d", cp.FilteredCount())
	}
}

func TestCommandPaletteSearchEmpty(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "First", "desc", "test", nil)
	cp.AddCommand("cmd-2", "Second", "desc", "test", nil)

	cp.Open()
	cp.SetSearchText("")

	// Empty search should match all
	if cp.FilteredCount() != 2 {
		t.Errorf("Expected 2 matches for empty search, got %d", cp.FilteredCount())
	}
}

func TestCommandPaletteFuzzyMatch(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Open File", "Open a file", "file", nil)
	cp.AddCommand("cmd-2", "Close File", "Close a file", "file", nil)
	cp.AddCommand("cmd-3", "Save All", "Save all files", "file", nil)

	cp.Open()
	cp.SetSearchText("of") // Should match "Open File"

	// Fuzzy match "of" should match "Open File"
	if cp.FilteredCount() < 1 {
		t.Error("Should match 'Open File' for fuzzy 'of'")
	}
}

func TestCommandPaletteNavigateUp(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "First", "", "test", nil)
	cp.AddCommand("cmd-2", "Second", "", "test", nil)
	cp.AddCommand("cmd-3", "Third", "", "test", nil)

	cp.Open()
	cp.selectedIdx = 2

	msg := tea.KeyPressMsg{Code: tea.KeyUp}
	cp.Update(msg)

	if cp.selectedIdx != 1 {
		t.Errorf("Expected index 1, got %d", cp.selectedIdx)
	}
}

func TestCommandPaletteNavigateDown(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "First", "", "test", nil)
	cp.AddCommand("cmd-2", "Second", "", "test", nil)
	cp.AddCommand("cmd-3", "Third", "", "test", nil)

	cp.Open()
	cp.selectedIdx = 0

	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	cp.Update(msg)

	if cp.selectedIdx != 1 {
		t.Errorf("Expected index 1, got %d", cp.selectedIdx)
	}
}

func TestCommandPaletteNavigateHome(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "First", "", "test", nil)
	cp.AddCommand("cmd-2", "Second", "", "test", nil)
	cp.AddCommand("cmd-3", "Third", "", "test", nil)

	cp.Open()
	cp.selectedIdx = 2

	msg := tea.KeyPressMsg{Code: tea.KeyHome}
	cp.Update(msg)

	if cp.selectedIdx != 0 {
		t.Errorf("Expected index 0, got %d", cp.selectedIdx)
	}
}

func TestCommandPaletteNavigateEnd(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "First", "", "test", nil)
	cp.AddCommand("cmd-2", "Second", "", "test", nil)
	cp.AddCommand("cmd-3", "Third", "", "test", nil)

	cp.Open()
	cp.selectedIdx = 0

	msg := tea.KeyPressMsg{Code: tea.KeyEnd}
	cp.Update(msg)

	if cp.selectedIdx != 2 {
		t.Errorf("Expected index 2, got %d", cp.selectedIdx)
	}
}

func TestCommandPaletteExecute(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	executed := false
	cp.AddCommand("cmd-1", "Test", "", "test", func() {
		executed = true
	})

	cp.Open()
	cp.selectedIdx = 0

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	cp.Update(msg)

	if !executed {
		t.Error("Command should be executed")
	}

	if cp.IsOpen() {
		t.Error("Palette should close after execution")
	}
}

func TestCommandPaletteExecuteCallback(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	var executedID string
	cp.SetOnExecute(func(cmdID string) {
		executedID = cmdID
	})

	cp.AddCommand("cmd-1", "Test", "", "test", nil)

	cp.Open()
	cp.selectedIdx = 0

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	cp.Update(msg)

	if executedID != "cmd-1" {
		t.Errorf("Expected cmd-1, got %s", executedID)
	}
}

func TestCommandPaletteEscape(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.Open()
	if !cp.IsOpen() {
		t.Fatal("Palette should be open")
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	cp.Update(msg)

	if cp.IsOpen() {
		t.Error("Palette should close on Escape")
	}
}

func TestCommandPaletteCharacterSearch(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "File", "File operations", "file", nil)
	cp.AddCommand("cmd-2", "Edit", "Edit operations", "edit", nil)
	cp.AddCommand("cmd-3", "Find", "Search files", "search", nil)

	cp.Open()

	// Type "f"
	msg := tea.KeyPressMsg{Code: 'f'}
	cp.Update(msg)

	// Should match File and Find (and Edit since search is in description)
	if cp.searchText != "f" {
		t.Errorf("Expected search 'f', got '%s'", cp.searchText)
	}
}

func TestCommandPaletteBackspace(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Help", "", "test", nil)
	cp.AddCommand("cmd-2", "History", "", "test", nil)

	cp.Open()
	cp.SetSearchText("hel")

	msg := tea.KeyPressMsg{Code: tea.KeyBackspace}
	cp.Update(msg)

	if cp.searchText != "he" {
		t.Errorf("Expected 'he', got '%s'", cp.searchText)
	}
}

func TestCommandPaletteBackspaceEmpty(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.Open()
	cp.SetSearchText("")

	msg := tea.KeyPressMsg{Code: tea.KeyBackspace}
	cp.Update(msg)

	if cp.searchText != "" {
		t.Error("Backspace on empty should do nothing")
	}
}

func TestCommandPaletteView(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Help", "Show help", "docs", nil)
	cp.SetSize(60, 20)
	cp.Open()

	view := cp.View()

	if view.Content == "" {
		t.Error("View should not be empty when open")
	}

	if !strings.Contains(view.Content, "Help") {
		t.Error("View should contain command title")
	}
}

func TestCommandPaletteViewClosed(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.Close()
	view := cp.View()

	if view.Content != "" {
		t.Error("View should be empty when closed")
	}
}

func TestCommandPaletteViewEmpty(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.Open()
	cp.SetSearchText("nonexistent")
	cp.SetSize(60, 20)

	view := cp.View()

	if !strings.Contains(view.Content, "No commands found") {
		t.Error("View should show no results message")
	}
}

func TestCommandPaletteSetSize(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.SetSize(80, 30)

	if cp.width != 80 {
		t.Errorf("Width should be 80, got %d", cp.width)
	}

	if cp.height != 30 {
		t.Errorf("Height should be 30, got %d", cp.height)
	}
}

func TestCommandPaletteInit(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cmd := cp.Init()

	if cmd != nil {
		t.Error("Init should return nil")
	}
}

func TestCommandPaletteGetSelectedCommand(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "First", "", "test", nil)
	cp.AddCommand("cmd-2", "Second", "", "test", nil)

	cp.Open()
	cp.selectedIdx = 1

	selected := cp.GetSelectedCommand()
	if selected == nil || selected.ID != "cmd-2" {
		t.Error("Should get correct selected command")
	}
}

func TestCommandPaletteGetSelectedCommandNone(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.Open()

	selected := cp.GetSelectedCommand()
	if selected != nil {
		t.Error("Should return nil when no commands")
	}
}

func TestCommandPaletteSetSearchText(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Test", "", "test", nil)

	cp.Open()
	cp.SetSearchText("test")

	if cp.GetSearchText() != "test" {
		t.Errorf("Expected 'test', got '%s'", cp.GetSearchText())
	}

	if cp.FilteredCount() != 1 {
		t.Errorf("Expected 1 match, got %d", cp.FilteredCount())
	}
}

func TestCommandPaletteRemoveNonexistent(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Test", "", "test", nil)

	success := cp.RemoveCommand("nonexistent")
	if success {
		t.Error("RemoveCommand should return false for nonexistent")
	}
}

func TestCommandPaletteCloseCallback(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	closeCalled := false
	cp.SetOnClose(func() {
		closeCalled = true
	})

	cp.Open()
	cp.Close()

	if !closeCalled {
		t.Error("OnClose callback should be called")
	}
}

func TestCommandPaletteGetCommandNonexistent(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cmd := cp.GetCommand("nonexistent")
	if cmd != nil {
		t.Error("GetCommand should return nil for nonexistent")
	}
}

func TestCommandPaletteNavigationBoundaries(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "First", "", "test", nil)
	cp.AddCommand("cmd-2", "Second", "", "test", nil)

	cp.Open()

	// Try to go up from first
	cp.selectedIdx = 0
	msg := tea.KeyPressMsg{Code: tea.KeyUp}
	cp.Update(msg)

	if cp.selectedIdx != 0 {
		t.Error("Should not go up from first")
	}

	// Try to go down from last
	cp.selectedIdx = 1
	msg = tea.KeyPressMsg{Code: tea.KeyDown}
	cp.Update(msg)

	if cp.selectedIdx != 1 {
		t.Error("Should not go down from last")
	}
}

func TestCommandPaletteMultipleCharacterSearch(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Open File", "", "file", nil)
	cp.AddCommand("cmd-2", "Close File", "", "file", nil)
	cp.AddCommand("cmd-3", "Save All", "", "file", nil)

	cp.Open()

	// Type "o"
	msg := tea.KeyPressMsg{Code: 'o'}
	cp.Update(msg)

	// Type "p"
	msg = tea.KeyPressMsg{Code: 'p'}
	cp.Update(msg)

	if cp.searchText != "op" {
		t.Errorf("Expected 'op', got '%s'", cp.searchText)
	}
}

func TestCommandPaletteSearchOnAddCommand(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.Open()
	cp.SetSearchText("test")

	cp.AddCommand("cmd-1", "Test Command", "", "test", nil)

	// Adding command should update filter
	if cp.FilteredCount() == 0 {
		t.Error("Should match added command")
	}
}

func TestCommandPaletteSearchCaseSensitive(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Open", "", "test", nil)
	cp.AddCommand("cmd-2", "CLOSE", "", "test", nil)

	cp.Open()
	cp.SetSearchText("open")

	// Should match "Open" case-insensitive
	if cp.FilteredCount() < 1 {
		t.Error("Should match case-insensitive")
	}
}

func TestCommandPaletteUpdateWhenClosed(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Test", "", "test", nil)
	cp.Close()

	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	cp.Update(msg)

	// Should not crash or change state
	if cp.selectedIdx != 0 {
		t.Error("Should not process input when closed")
	}
}

func TestCommandPaletteExecuteNilFunction(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Test", "", "test", nil)

	cp.Open()
	cp.selectedIdx = 0

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	cp.Update(msg)

	// Should not crash even with nil execute function
	if cp.IsOpen() {
		t.Error("Palette should close after execution")
	}
}

func TestCommandPaletteCommandCount(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	if cp.CommandCount() != 0 {
		t.Errorf("Expected 0 commands, got %d", cp.CommandCount())
	}

	cp.AddCommand("cmd-1", "Test", "", "test", nil)
	if cp.CommandCount() != 1 {
		t.Errorf("Expected 1 command, got %d", cp.CommandCount())
	}
}

func TestCommandPaletteFilteredCount(t *testing.T) {
	th := theme.Current()
	cp := NewCommandPalette(th)

	cp.AddCommand("cmd-1", "Help", "", "test", nil)
	cp.AddCommand("cmd-2", "History", "", "test", nil)

	cp.Open()

	if cp.FilteredCount() != 2 {
		t.Errorf("Expected 2 filtered, got %d", cp.FilteredCount())
	}

	cp.SetSearchText("history")
	if cp.FilteredCount() != 1 {
		t.Errorf("Expected 1 filtered for 'history', got %d", cp.FilteredCount())
	}
}
