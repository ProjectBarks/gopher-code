package rules

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRules_TabSwitch(t *testing.T) {
	m := New(
		[]Rule{{Pattern: "Bash(git:*)", Source: "userSettings"}},
		nil,
		[]Rule{{Pattern: "Edit(*)", Source: "projectSettings"}},
		nil,
		80,
	)

	if m.tab != TabRecent {
		t.Error("should start on Recent tab")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != TabAllow {
		t.Errorf("tab = %d, want TabAllow", m.tab)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != TabAsk {
		t.Error("should be on Ask tab")
	}
}

func TestRules_Navigation(t *testing.T) {
	allow := []Rule{
		{Pattern: "Bash(git:*)", Source: "user"},
		{Pattern: "Edit(*)", Source: "user"},
		{Pattern: "Read(*)", Source: "project"},
	}
	m := New(allow, nil, nil, nil, 80)
	m.tab = TabAllow

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selected != 1 {
		t.Errorf("selected = %d, want 1", m.selected)
	}
}

func TestRules_Dismiss(t *testing.T) {
	m := New(nil, nil, nil, nil, 80)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("escape should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(RuleDismissedMsg); !ok {
		t.Fatalf("expected RuleDismissedMsg, got %T", msg)
	}
}

func TestRules_ViewRecent(t *testing.T) {
	m := New(nil, nil, nil, nil, 80)
	v := m.View()
	if v == "" {
		t.Error("should produce output")
	}
	if !strings.Contains(v, "denials") {
		t.Error("empty recent should show 'no recent denials' message")
	}
}

func TestRules_ViewWithRules(t *testing.T) {
	allow := []Rule{{Pattern: "Bash(git:*)", Source: "userSettings"}}
	m := New(allow, nil, nil, nil, 80)
	m.tab = TabAllow
	v := m.View()
	if !strings.Contains(v, "Bash(git:*)") {
		t.Error("should show rule pattern")
	}
	if !strings.Contains(v, "userSettings") {
		t.Error("should show rule source")
	}
}

func TestRules_TabCounts(t *testing.T) {
	m := New(
		[]Rule{{Pattern: "a"}, {Pattern: "b"}},
		[]Rule{{Pattern: "c"}},
		nil,
		[]Rule{{Pattern: "d"}},
		80,
	)
	v := m.View()
	if !strings.Contains(v, "Allow (2)") {
		t.Error("should show allow count")
	}
	if !strings.Contains(v, "Ask (1)") {
		t.Error("should show ask count")
	}
}
