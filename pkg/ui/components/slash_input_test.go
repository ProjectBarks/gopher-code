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

// ---------------------------------------------------------------------------
// T174: /remote-control in DefaultSlashCommands
// ---------------------------------------------------------------------------

func TestDefaultSlashCommands_ContainsRemoteControl(t *testing.T) {
	cmds := DefaultSlashCommands()
	found := false
	for _, c := range cmds {
		if c.Name == "/remote-control" {
			found = true
			if c.Handler != "remote-control" {
				t.Errorf("Handler = %q, want %q", c.Handler, "remote-control")
			}
			if c.Source != "builtin" {
				t.Errorf("Source = %q, want %q", c.Source, "builtin")
			}
		}
	}
	if !found {
		t.Error("/remote-control command not found in DefaultSlashCommands")
	}
}

// ---------------------------------------------------------------------------
// T222: SlashCommand struct extensions
// ---------------------------------------------------------------------------

func TestSlashCommand_AliasesField(t *testing.T) {
	cmd := SlashCommand{
		Name:    "/quit",
		Aliases: []string{"/q", "/exit"},
	}
	if len(cmd.Aliases) != 2 {
		t.Errorf("Aliases length = %d, want 2", len(cmd.Aliases))
	}
}

func TestSlashCommand_IsHiddenFiltersFromSuggestions(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.SetCommands([]SlashCommand{
		{Name: "/visible", Description: "shown"},
		{Name: "/hidden", Description: "hidden", IsHidden: true},
	})
	sci.Activate("/")
	for _, s := range sci.Suggestions() {
		if s.Name == "/hidden" {
			t.Error("Hidden commands should not appear in suggestions")
		}
	}
	if len(sci.Suggestions()) != 1 {
		t.Errorf("Suggestions count = %d, want 1", len(sci.Suggestions()))
	}
}

func TestSlashCommand_IsEnabledFiltersFromSuggestions(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.SetCommands([]SlashCommand{
		{Name: "/enabled", Description: "on"},
		{Name: "/disabled", Description: "off", IsEnabled: func() bool { return false }},
		{Name: "/default", Description: "nil IsEnabled means enabled"},
	})
	sci.Activate("/")
	names := make(map[string]bool)
	for _, s := range sci.Suggestions() {
		names[s.Name] = true
	}
	if names["/disabled"] {
		t.Error("Disabled commands should not appear in suggestions")
	}
	if !names["/enabled"] {
		t.Error("/enabled should appear")
	}
	if !names["/default"] {
		t.Error("/default (nil IsEnabled) should appear")
	}
}

func TestSlashCommand_AliasMatchesSuggestions(t *testing.T) {
	sci := NewSlashCommandInput(theme.Current())
	sci.SetCommands([]SlashCommand{
		{Name: "/quit", Description: "Exit", Aliases: []string{"/q"}},
		{Name: "/help", Description: "Help"},
	})
	sci.Activate("/q")
	found := false
	for _, s := range sci.Suggestions() {
		if s.Name == "/quit" {
			found = true
		}
	}
	if !found {
		t.Error("/quit should match via alias /q")
	}
}

func TestSlashCommand_ExtendedFieldsExist(t *testing.T) {
	// Verify all T222 fields compile and can be set.
	cmd := SlashCommand{
		Name:         "/test",
		Aliases:      []string{"/t"},
		ArgumentHint: "<file>",
		IsHidden:     false,
		IsEnabled:    func() bool { return true },
		Immediate:    true,
		Availability: []CommandAvailability{AvailabilityClaudeAI, AvailabilityConsole},
		Type:         CommandTypeLocal,
	}
	if cmd.ArgumentHint != "<file>" {
		t.Error("ArgumentHint not set")
	}
	if !cmd.Immediate {
		t.Error("Immediate not set")
	}
	if len(cmd.Availability) != 2 {
		t.Errorf("Availability length = %d, want 2", len(cmd.Availability))
	}
	if cmd.Type != CommandTypeLocal {
		t.Errorf("Type = %q, want %q", cmd.Type, CommandTypeLocal)
	}
}
