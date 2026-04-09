package hooks

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	pkghooks "github.com/projectbarks/gopher-code/pkg/hooks"
)

func testHooks() []pkghooks.IndividualHookConfig {
	return []pkghooks.IndividualHookConfig{
		{
			Event:   pkghooks.PreToolUse,
			Config:  pkghooks.HookCommand{Type: pkghooks.HookCommandTypeBash, Command: "echo pre-tool"},
			Matcher: "Bash",
			Source:  pkghooks.HookSourceUserSettings,
		},
		{
			Event:   pkghooks.PreToolUse,
			Config:  pkghooks.HookCommand{Type: pkghooks.HookCommandTypeBash, Command: "lint.sh"},
			Matcher: "Write",
			Source:  pkghooks.HookSourceProjectSettings,
		},
		{
			Event:  pkghooks.Notification,
			Config: pkghooks.HookCommand{Type: pkghooks.HookCommandTypeBash, Command: "notify-send"},
			Source: pkghooks.HookSourceUserSettings,
		},
	}
}

func TestNew(t *testing.T) {
	m := New(testHooks(), []string{"Bash", "Write"})
	if m.level != levelEvents {
		t.Error("should start at events level")
	}
	if len(m.events) == 0 {
		t.Error("should have events with hooks")
	}
}

func TestNew_Empty(t *testing.T) {
	m := New(nil, nil)
	if len(m.events) != 0 {
		t.Error("no hooks = no events")
	}
}

func TestModel_View_Events(t *testing.T) {
	m := New(testHooks(), []string{"Bash", "Write"})
	v := m.View()

	if !strings.Contains(v, "Hook Events") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "PreToolUse") {
		t.Error("should show PreToolUse event")
	}
	if !strings.Contains(v, "Notification") {
		t.Error("should show Notification event")
	}
}

func TestModel_View_EmptyHooks(t *testing.T) {
	m := New(nil, nil)
	v := m.View()
	if !strings.Contains(v, "No hooks configured") {
		t.Error("empty should show message")
	}
}

func TestModel_DrillDown(t *testing.T) {
	m := New(testHooks(), []string{"Bash", "Write"})

	// Select first event (PreToolUse)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Should be at matchers or hooks level now
	if m.level != levelMatchers && m.level != levelHooks {
		t.Errorf("level = %d, expected matchers or hooks", m.level)
	}
}

func TestModel_BackFromHooks(t *testing.T) {
	m := New(testHooks(), []string{"Bash", "Write"})

	// Drill down
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	prevLevel := m.level

	// Back
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.level >= prevLevel {
		t.Error("Escape should go back")
	}
}

func TestModel_ExitFromEvents(t *testing.T) {
	m := New(testHooks(), nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc at events should close")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
}

func TestModel_QuitFromEvents(t *testing.T) {
	m := New(testHooks(), nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil {
		t.Fatal("q at events should close")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New(testHooks(), []string{"Bash", "Write"})
	if m.cursor != 0 {
		t.Error("should start at 0")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Error("down should move cursor")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Error("up should move cursor back")
	}
}

func TestModel_VimKeys(t *testing.T) {
	m := New(testHooks(), nil)
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Error("k should move up")
	}
}

func TestHookDisplayText(t *testing.T) {
	tests := []struct {
		hook pkghooks.IndividualHookConfig
		want string
	}{
		{pkghooks.IndividualHookConfig{Config: pkghooks.HookCommand{Command: "echo hi"}}, "echo hi"},
		{pkghooks.IndividualHookConfig{Config: pkghooks.HookCommand{Type: "prompt"}}, "prompt hook"},
		{pkghooks.IndividualHookConfig{}, "hook"},
	}
	for _, tt := range tests {
		got := hookDisplayText(tt.hook)
		if got != tt.want {
			t.Errorf("hookDisplayText = %q, want %q", got, tt.want)
		}
	}
}

func TestPlural(t *testing.T) {
	if plural(1) != "" {
		t.Error("1 should be empty")
	}
	if plural(0) != "s" {
		t.Error("0 should be 's'")
	}
	if plural(5) != "s" {
		t.Error("5 should be 's'")
	}
}
