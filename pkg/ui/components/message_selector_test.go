package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func makeTestEntries() []MessageEntry {
	return []MessageEntry{
		{ID: "1", Preview: "Fix the login bug", TurnNum: 1, IsUser: true},
		{ID: "2", Preview: "Add tests for auth", TurnNum: 2, IsUser: true},
		{ID: "3", Preview: "Refactor the API", TurnNum: 3, IsUser: true},
	}
}

func TestMessageSelector_Navigation(t *testing.T) {
	m := NewMessageSelector(makeTestEntries())
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

func TestMessageSelector_SelectAndChooseAction(t *testing.T) {
	m := NewMessageSelector(makeTestEntries())

	// Select first message
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.phase != 1 {
		t.Fatal("enter should advance to action phase")
	}

	// Choose "Restore both" (first option)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("should return a cmd")
	}
	msg := cmd()
	done, ok := msg.(MessageSelectorDoneMsg)
	if !ok {
		t.Fatalf("expected MessageSelectorDoneMsg, got %T", msg)
	}
	if done.MessageID != "1" {
		t.Errorf("messageID = %q, want 1", done.MessageID)
	}
	if done.Action != RestoreBoth {
		t.Errorf("action = %q, want both", done.Action)
	}
}

func TestMessageSelector_Cancel(t *testing.T) {
	m := NewMessageSelector(makeTestEntries())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("escape should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(MessageSelectorCancelMsg); !ok {
		t.Fatalf("expected cancel msg, got %T", msg)
	}
}

func TestMessageSelector_BackFromAction(t *testing.T) {
	m := NewMessageSelector(makeTestEntries())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // phase 1
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // back
	if m.phase != 0 {
		t.Error("escape in action phase should go back to message list")
	}
}

func TestMessageSelector_ViewPhase0(t *testing.T) {
	m := NewMessageSelector(makeTestEntries())
	v := m.View()
	if !strings.Contains(v, "rewind") {
		t.Error("should mention rewind")
	}
	if !strings.Contains(v, "Turn 1") {
		t.Error("should show turn numbers")
	}
}

func TestMessageSelector_ViewPhase1(t *testing.T) {
	m := NewMessageSelector(makeTestEntries())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v := m.View()
	if !strings.Contains(v, "restore") {
		t.Error("action phase should mention restore")
	}
}

func TestMessageSelector_Empty(t *testing.T) {
	m := NewMessageSelector(nil)
	v := m.View()
	if !strings.Contains(v, "No messages") {
		t.Error("empty should show no messages")
	}
}
