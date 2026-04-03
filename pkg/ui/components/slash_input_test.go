package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

func TestSlashCommandInputCreation(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	if sci == nil {
		t.Fatal("SlashCommandInput should not be nil")
	}
	if sci.IsActive() {
		t.Error("Should not be active initially")
	}
}

func TestSlashCommandInputActivate(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.Activate("/m")
	if !sci.IsActive() {
		t.Error("Should be active after Activate")
	}
	if len(sci.Suggestions()) == 0 {
		t.Error("Should have suggestions for '/m'")
	}
}

func TestSlashCommandInputFilterModel(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.Activate("/model")
	found := false
	for _, s := range sci.Suggestions() {
		if s.Name == "/model" {
			found = true
		}
	}
	if !found {
		t.Error("Should suggest /model for '/model' prefix")
	}
}

func TestSlashCommandInputFilterAll(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.Activate("/")
	if len(sci.Suggestions()) != len(DefaultSlashCommands()) {
		t.Errorf("Expected all commands for '/', got %d", len(sci.Suggestions()))
	}
}

func TestSlashCommandInputDeactivate(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.Activate("/m")
	sci.Deactivate()
	if sci.IsActive() {
		t.Error("Should not be active after Deactivate")
	}
}

func TestSlashCommandInputEscapeDeactivates(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.Activate("/")
	sci.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if sci.IsActive() {
		t.Error("Escape should deactivate")
	}
}

func TestSlashCommandInputNavigation(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.Activate("/")
	sci.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if sci.selected != 1 {
		t.Error("Down should move selection")
	}
	sci.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if sci.selected != 0 {
		t.Error("Up should move selection back")
	}
}

func TestSlashCommandInputViewEmpty(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	view := sci.View()
	if view.Content != "" {
		t.Error("Inactive input should render empty")
	}
}

func TestSlashCommandInputViewActive(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.Activate("/")
	view := sci.View()
	if view.Content == "" {
		t.Error("Active input should render suggestions")
	}
}
