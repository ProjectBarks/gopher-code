package help

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testCommands() []CommandInfo {
	return []CommandInfo{
		{Name: "help", Description: "Show this help screen"},
		{Name: "doctor", Description: "Run diagnostics"},
		{Name: "compact", Description: "Compact conversation"},
		{Name: "hidden", Description: "Hidden command", IsHidden: true},
	}
}

func TestHelp_InitialState(t *testing.T) {
	m := New(testCommands(), 80, 40)
	if m.tab != TabGeneral {
		t.Error("should start on General tab")
	}
	// Hidden commands should be filtered
	if len(m.commands) != 3 {
		t.Errorf("expected 3 visible commands, got %d", len(m.commands))
	}
}

func TestHelp_TabSwitch(t *testing.T) {
	m := New(testCommands(), 80, 40)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != TabCommands {
		t.Error("Tab should switch to Commands")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != TabGeneral {
		t.Error("Tab again should switch back to General")
	}
}

func TestHelp_Dismiss(t *testing.T) {
	m := New(testCommands(), 80, 40)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("escape should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(HelpDismissedMsg); !ok {
		t.Fatalf("expected HelpDismissedMsg, got %T", msg)
	}
}

func TestHelp_ViewGeneral(t *testing.T) {
	m := New(testCommands(), 80, 40)
	v := m.View()
	if !strings.Contains(v, "Keyboard Shortcuts") {
		t.Error("general tab should show shortcuts")
	}
	if !strings.Contains(v, "Enter") {
		t.Error("should list Enter shortcut")
	}
}

func TestHelp_ViewCommands(t *testing.T) {
	m := New(testCommands(), 80, 40)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	v := m.View()
	if !strings.Contains(v, "/help") {
		t.Error("commands tab should show /help")
	}
	if !strings.Contains(v, "/doctor") {
		t.Error("commands tab should show /doctor")
	}
}

func TestHelp_Scroll(t *testing.T) {
	m := New(testCommands(), 80, 40)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.scroll != 1 {
		t.Errorf("scroll = %d, want 1", m.scroll)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.scroll != 0 {
		t.Errorf("scroll = %d, want 0", m.scroll)
	}
}
