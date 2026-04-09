package mcp

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testServers() []ServerInfo {
	return []ServerInfo{
		{Name: "github", Status: StatusConnected, ToolCount: 15, Transport: "stdio", Command: "github-mcp"},
		{Name: "slack", Status: StatusConnected, ToolCount: 8, Transport: "sse", URL: "https://mcp.slack.com"},
		{Name: "broken", Status: StatusError, ToolCount: 0, Transport: "stdio", Command: "broken-server"},
	}
}

func TestNewSettingsModel(t *testing.T) {
	m := NewSettingsModel(testServers())
	if m.level != settingsLevelList {
		t.Error("should start at list level")
	}
	if len(m.servers) != 3 {
		t.Errorf("servers = %d", len(m.servers))
	}
}

func TestSettings_View_ServerList(t *testing.T) {
	m := NewSettingsModel(testServers())
	v := m.View()
	if !strings.Contains(v, "MCP Servers") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "github") {
		t.Error("should show github server")
	}
	if !strings.Contains(v, "slack") {
		t.Error("should show slack server")
	}
	if !strings.Contains(v, "3 servers") {
		t.Error("should show server count")
	}
}

func TestSettings_View_Empty(t *testing.T) {
	m := NewSettingsModel(nil)
	v := m.View()
	if !strings.Contains(v, "No MCP servers") {
		t.Error("empty should show message")
	}
}

func TestSettings_Navigation(t *testing.T) {
	m := NewSettingsModel(testServers())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d", m.cursor)
	}
}

func TestSettings_DrillDown_Server(t *testing.T) {
	m := NewSettingsModel(testServers())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.level != settingsLevelServer {
		t.Errorf("level = %d, want server detail", m.level)
	}
	if m.selectedServer == nil || m.selectedServer.Name != "github" {
		t.Error("should select first server")
	}

	v := m.View()
	if !strings.Contains(v, "github") {
		t.Error("detail should show server name")
	}
	if !strings.Contains(v, "connected") {
		t.Error("detail should show status")
	}
}

func TestSettings_DrillDown_ToolList(t *testing.T) {
	m := NewSettingsModel(testServers())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // to server detail
	m.SetTools([]ToolInfo{
		{Name: "search_code", DisplayName: "Search Code"},
		{Name: "create_issue", DisplayName: "Create Issue"},
	})
	m, _ = m.Update(tea.KeyPressMsg{Code: 't'}) // to tool list
	if m.level != settingsLevelToolList {
		t.Errorf("level = %d, want tool list", m.level)
	}

	v := m.View()
	if !strings.Contains(v, "Search Code") {
		t.Error("should show tool names")
	}
}

func TestSettings_Back_FromServer(t *testing.T) {
	m := NewSettingsModel(testServers())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // to server
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // back
	if m.level != settingsLevelList {
		t.Errorf("level = %d, should be list", m.level)
	}
}

func TestSettings_Back_FromToolList(t *testing.T) {
	m := NewSettingsModel(testServers())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // to server
	m, _ = m.Update(tea.KeyPressMsg{Code: 't'})          // to tools
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // back to server
	if m.level != settingsLevelServer {
		t.Errorf("level = %d, should be server", m.level)
	}
}

func TestSettings_Close(t *testing.T) {
	m := NewSettingsModel(testServers())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc at list should close")
	}
	if _, ok := cmd().(SettingsClosedMsg); !ok {
		t.Error("expected SettingsClosedMsg")
	}
}

func TestScopeLabel(t *testing.T) {
	tests := []struct {
		scope ConfigScope
		want  string
	}{
		{ScopeProject, "Project MCPs"},
		{ScopeUser, "User MCPs"},
		{ScopeLocal, "Local MCPs"},
		{ScopeEnterprise, "Enterprise MCPs"},
		{ScopeDynamic, "Built-in MCPs"},
	}
	for _, tt := range tests {
		if got := ScopeLabel(tt.scope); got != tt.want {
			t.Errorf("ScopeLabel(%q) = %q, want %q", tt.scope, got, tt.want)
		}
	}
}
