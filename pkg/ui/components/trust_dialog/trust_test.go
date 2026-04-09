package trust_dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTrustDialog_Accept(t *testing.T) {
	m := New(ProjectConfig{CWD: "/project", HasHooks: true})

	// Default is "trust" (selected=0), press Enter
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("should return a cmd")
	}
	msg := cmd()
	dm, ok := msg.(TrustDecisionMsg)
	if !ok {
		t.Fatalf("expected TrustDecisionMsg, got %T", msg)
	}
	if dm.Decision != TrustAccepted {
		t.Error("should be accepted")
	}
	if dm.Project != "/project" {
		t.Errorf("project = %q", dm.Project)
	}
}

func TestTrustDialog_Reject(t *testing.T) {
	m := New(ProjectConfig{CWD: "/project"})

	// Move down to "reject"
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("should return a cmd")
	}
	msg := cmd()
	dm := msg.(TrustDecisionMsg)
	if dm.Decision != TrustRejected {
		t.Error("should be rejected")
	}
}

func TestTrustDialog_EscapeRejects(t *testing.T) {
	m := New(ProjectConfig{CWD: "/project"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := cmd()
	dm := msg.(TrustDecisionMsg)
	if dm.Decision != TrustRejected {
		t.Error("escape should reject")
	}
}

func TestTrustDialog_ViewShowsRisks(t *testing.T) {
	m := New(ProjectConfig{
		CWD:           "/my/project",
		HasHooks:      true,
		HasMCPServers: true,
		MCPServerCount: 3,
	})
	v := m.View()
	if !strings.Contains(v, "Hooks") {
		t.Error("should show hooks warning")
	}
	if !strings.Contains(v, "MCP servers") {
		t.Error("should show MCP warning")
	}
	if !strings.Contains(v, "3") {
		t.Error("should show server count")
	}
}

func TestProjectConfig_HasRisks(t *testing.T) {
	if (ProjectConfig{}).HasRisks() {
		t.Error("empty config should have no risks")
	}
	if !(ProjectConfig{HasHooks: true}).HasRisks() {
		t.Error("hooks should be a risk")
	}
	if !(ProjectConfig{HasMCPServers: true}).HasRisks() {
		t.Error("MCP servers should be a risk")
	}
}
