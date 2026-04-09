package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testServer() ServerInfo {
	return ServerInfo{
		Name:         "github",
		Status:       StatusConnected,
		ToolCount:    3,
		Transport:    "stdio",
		Command:      "github-mcp-server",
		Capabilities: []string{"tools", "resources"},
	}
}

func testTools() []ToolInfo {
	return []ToolInfo{
		{Name: "search_code", DisplayName: "Search Code", Description: "Search repository code", IsReadOnly: true},
		{Name: "create_issue", DisplayName: "Create Issue", Description: "Create a new issue"},
		{Name: "delete_branch", DisplayName: "Delete Branch", IsDestructive: true, InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
}

func TestToolListModel_View(t *testing.T) {
	m := NewToolListModel(testServer(), testTools())
	v := m.View()

	if !strings.Contains(v, "github") {
		t.Error("should show server name")
	}
	if !strings.Contains(v, "3") {
		t.Error("should show tool count")
	}
	if !strings.Contains(v, "Search Code") {
		t.Error("should show tool names")
	}
	if !strings.Contains(v, "read-only") {
		t.Error("should show read-only annotation")
	}
	if !strings.Contains(v, "destructive") {
		t.Error("should show destructive annotation")
	}
}

func TestToolListModel_EmptyTools(t *testing.T) {
	m := NewToolListModel(testServer(), nil)
	v := m.View()
	if !strings.Contains(v, "No tools available") {
		t.Error("empty tools should show message")
	}
}

func TestToolListModel_Navigation(t *testing.T) {
	m := NewToolListModel(testServer(), testTools())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Error("down should move cursor")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Error("up should move cursor back")
	}
}

func TestToolListModel_Select(t *testing.T) {
	m := NewToolListModel(testServer(), testTools())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // index 1
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return cmd")
	}
	msg := cmd()
	sel, ok := msg.(ToolSelectedMsg)
	if !ok {
		t.Fatalf("expected ToolSelectedMsg, got %T", msg)
	}
	if sel.Index != 1 {
		t.Errorf("index = %d, want 1", sel.Index)
	}
}

func TestToolListModel_Back(t *testing.T) {
	m := NewToolListModel(testServer(), testTools())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Fatalf("expected BackMsg, got %T", msg)
	}
}

func TestToolDetailModel_View(t *testing.T) {
	tool := testTools()[0] // Search Code
	m := ToolDetailModel{Tool: tool, Server: testServer()}
	v := m.View()

	if !strings.Contains(v, "Search Code") {
		t.Error("should show display name")
	}
	if !strings.Contains(v, "github") {
		t.Error("should show server name")
	}
	if !strings.Contains(v, "read-only") {
		t.Error("should show read-only access")
	}
	if !strings.Contains(v, "Search repository code") {
		t.Error("should show description")
	}
}

func TestToolDetailModel_WithSchema(t *testing.T) {
	tool := testTools()[2] // Delete Branch with schema
	m := ToolDetailModel{Tool: tool, Server: testServer()}
	v := m.View()

	if !strings.Contains(v, "Input Schema") {
		t.Error("should show input schema section")
	}
	if !strings.Contains(v, "destructive") {
		t.Error("should show destructive access")
	}
}

func TestReconnectModel_View(t *testing.T) {
	server := ServerInfo{
		Name:      "github",
		Status:    StatusDisconnected,
		Transport: "stdio",
		Command:   "github-mcp-server",
	}
	m := NewReconnectModel(server)
	v := m.View()

	if !strings.Contains(v, "disconnected") {
		t.Error("should mention disconnected")
	}
	if !strings.Contains(v, "github") {
		t.Error("should show server name")
	}
	if !strings.Contains(v, "Reconnect") {
		t.Error("should show Reconnect option")
	}
	if !strings.Contains(v, "Cancel") {
		t.Error("should show Cancel option")
	}
}

func TestReconnectModel_Reconnect(t *testing.T) {
	m := NewReconnectModel(ServerInfo{Name: "srv"})
	m.cursor = 0
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("should return cmd")
	}
	msg := cmd()
	rc, ok := msg.(ReconnectMsg)
	if !ok {
		t.Fatalf("expected ReconnectMsg, got %T", msg)
	}
	if rc.ServerName != "srv" {
		t.Errorf("name = %q", rc.ServerName)
	}
}

func TestReconnectModel_Cancel(t *testing.T) {
	m := NewReconnectModel(ServerInfo{Name: "srv"})
	m.cursor = 1
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Fatalf("expected BackMsg, got %T", msg)
	}
}

func TestRenderCapabilities(t *testing.T) {
	caps := []string{"tools", "resources", "prompts"}
	result := RenderCapabilities(caps)
	if !strings.Contains(result, "Capabilities") {
		t.Error("should show title")
	}
	if !strings.Contains(result, "tools") {
		t.Error("should list tools capability")
	}
	if !strings.Contains(result, "✓") {
		t.Error("should show checkmarks")
	}
}

func TestRenderCapabilities_Empty(t *testing.T) {
	result := RenderCapabilities(nil)
	if result != "" {
		t.Error("empty caps should return empty string")
	}
}

func TestToolAnnotations(t *testing.T) {
	if toolAnnotations(ToolInfo{}) != "" {
		t.Error("no annotations should be empty")
	}
	if !strings.Contains(toolAnnotations(ToolInfo{IsReadOnly: true}), "read-only") {
		t.Error("should include read-only")
	}
	if !strings.Contains(toolAnnotations(ToolInfo{IsDestructive: true}), "destructive") {
		t.Error("should include destructive")
	}
	if !strings.Contains(toolAnnotations(ToolInfo{IsReadOnly: true, IsDestructive: true}), ", ") {
		t.Error("multiple annotations should be comma-separated")
	}
}

func TestServerStatus_Constants(t *testing.T) {
	if StatusConnected != "connected" {
		t.Error("wrong")
	}
	if StatusDisconnected != "disconnected" {
		t.Error("wrong")
	}
	if StatusError != "error" {
		t.Error("wrong")
	}
}
