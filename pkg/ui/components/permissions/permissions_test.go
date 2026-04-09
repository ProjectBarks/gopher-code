package permissions

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPermission_Allow(t *testing.T) {
	m := New("tool-1", Request{Type: RequestBash, ToolName: "Bash", Command: "git status"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	if cmd == nil {
		t.Fatal("y should return cmd")
	}
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionAllow {
		t.Errorf("decision = %q, want allow", msg.Decision)
	}
	if msg.ToolUseID != "tool-1" {
		t.Errorf("toolUseID = %q", msg.ToolUseID)
	}
}

func TestPermission_Deny(t *testing.T) {
	m := New("tool-2", Request{Type: RequestEdit, FilePath: "main.go"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n'})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Errorf("decision = %q, want deny", msg.Decision)
	}
}

func TestPermission_AlwaysAllow(t *testing.T) {
	m := New("tool-3", Request{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'a'})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionAlwaysAllow {
		t.Errorf("decision = %q, want always_allow", msg.Decision)
	}
}

func TestPermission_EnterSelect(t *testing.T) {
	m := New("tool-4", Request{})
	// Default selected is 0 (Allow)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionAllow {
		t.Error("enter on first option should allow")
	}

	// Move to deny and enter
	m2 := New("tool-5", Request{})
	m2, _ = m2.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd = m2.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg = cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Error("enter on second option should deny")
	}
}

func TestPermission_EscapeDenies(t *testing.T) {
	m := New("tool-6", Request{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Error("escape should deny")
	}
}

func TestPermission_ViewBash(t *testing.T) {
	m := New("x", Request{Type: RequestBash, ToolName: "Bash", Command: "rm -rf /", Description: "Run a command", IsDangerous: true})
	v := m.View()
	if !strings.Contains(v, "rm -rf") {
		t.Error("should show command")
	}
	if !strings.Contains(v, "Bash") {
		t.Error("should show tool name")
	}
}

func TestPermission_ViewWebFetch(t *testing.T) {
	m := New("x", Request{Type: RequestWebFetch, URL: "https://example.com", Description: "Fetch a page"})
	v := m.View()
	if !strings.Contains(v, "example.com") {
		t.Error("should show URL")
	}
}

func TestRenderCompactPermission(t *testing.T) {
	got := RenderCompactPermission(Request{Type: RequestBash, Command: "ls"})
	if got != "Run: ls" {
		t.Errorf("got %q", got)
	}
}
