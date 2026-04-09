package sandbox

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestConfig_CurrentMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want SandboxMode
	}{
		{"disabled", Config{Enabled: false}, ModeDisabled},
		{"regular", Config{Enabled: true}, ModeRegular},
		{"auto-allow", Config{Enabled: true, AutoAllow: true}, ModeAutoAllow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.CurrentMode(); got != tt.want {
				t.Errorf("CurrentMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModeLabel(t *testing.T) {
	tests := []struct {
		mode SandboxMode
		want string
	}{
		{ModeAutoAllow, "Sandbox BashTool, with auto-allow"},
		{ModeRegular, "Sandbox BashTool, with regular permissions"},
		{ModeDisabled, "Disable sandboxing"},
	}
	for _, tt := range tests {
		if got := ModeLabel(tt.mode); got != tt.want {
			t.Errorf("ModeLabel(%q) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestNew_CursorOnCurrent(t *testing.T) {
	cfg := Config{Enabled: true} // regular mode
	m := New(cfg)
	// Cursor should be on "regular" which is index 1
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (regular)", m.cursor)
	}
}

func TestNew_CursorOnDisabled(t *testing.T) {
	cfg := Config{Enabled: false}
	m := New(cfg)
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (disabled)", m.cursor)
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New(Config{Enabled: true})

	// Move up from regular (1) to auto-allow (0)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d after up, want 0", m.cursor)
	}

	// Can't go above 0
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, should stay at 0", m.cursor)
	}

	// Move down
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}

	// Can't go past 2
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, should stay at 2", m.cursor)
	}
}

func TestModel_SelectMode(t *testing.T) {
	m := New(Config{Enabled: false})
	// Move to auto-allow (index 0)
	m.cursor = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return cmd")
	}
	msg := cmd()
	modeMsg, ok := msg.(ModeChangedMsg)
	if !ok {
		t.Fatalf("expected ModeChangedMsg, got %T", msg)
	}
	if modeMsg.Mode != ModeAutoAllow {
		t.Errorf("mode = %q, want auto-allow", modeMsg.Mode)
	}
}

func TestModel_Cancel(t *testing.T) {
	m := New(Config{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Escape should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
}

func TestModel_TabSwitch(t *testing.T) {
	m := New(Config{Enabled: true})
	if m.tab != 0 {
		t.Error("should start on mode tab")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != 1 {
		t.Errorf("tab = %d, want 1 (config)", m.tab)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != 2 {
		t.Errorf("tab = %d, want 2 (deps)", m.tab)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != 0 {
		t.Error("should wrap back to tab 0")
	}
}

func TestModel_View_ModeTab(t *testing.T) {
	m := New(Config{Enabled: true})
	v := m.View()

	if !strings.Contains(v, "Mode") {
		t.Error("should show Mode tab")
	}
	if !strings.Contains(v, "auto-allow") {
		t.Error("should show auto-allow option")
	}
	if !strings.Contains(v, "regular permissions") {
		t.Error("should show regular option")
	}
	if !strings.Contains(v, "(current)") {
		t.Error("should mark current mode")
	}
}

func TestModel_View_ConfigTab(t *testing.T) {
	m := New(Config{
		Enabled:          true,
		ExcludedCommands: []string{"rm", "dd"},
		ReadDeny:         []string{"/etc/shadow"},
		WriteAllow:       []string{"/tmp"},
		NetworkAllow:     []string{"api.anthropic.com"},
	})
	m.tab = 1
	v := m.View()

	if !strings.Contains(v, "Excluded Commands") {
		t.Error("should show excluded commands section")
	}
	if !strings.Contains(v, "rm, dd") {
		t.Error("should list excluded commands")
	}
	if !strings.Contains(v, "Read Restrictions") {
		t.Error("should show read restrictions")
	}
}

func TestModel_View_ConfigTabDisabled(t *testing.T) {
	m := New(Config{Enabled: false})
	m.tab = 1
	v := m.View()
	if !strings.Contains(v, "not enabled") {
		t.Error("disabled config should say 'not enabled'")
	}
}

func TestModel_View_DepsTab(t *testing.T) {
	m := New(Config{Warnings: []string{"bwrap not found"}})
	m.tab = 2
	v := m.View()

	if !strings.Contains(v, "Dependencies") {
		t.Error("should show Dependencies title")
	}
	if !strings.Contains(v, "bwrap not found") {
		t.Error("should show warnings")
	}
}

func TestModel_View_DepsTabNoWarnings(t *testing.T) {
	m := New(Config{})
	m.tab = 2
	v := m.View()
	if !strings.Contains(v, "No dependency issues") {
		t.Error("should say no issues when no warnings")
	}
}

func TestModel_VimKeys(t *testing.T) {
	m := New(Config{Enabled: true, AutoAllow: true}) // cursor at 0
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Error("k should move up")
	}
}
