package plugin

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testPlugins() []PluginInfo {
	return []PluginInfo{
		{Name: "thinkback", Version: "1.0.0", Source: "marketplace", Enabled: true, Description: "Year in review animation", Skills: 1},
		{Name: "code-review", Version: "0.5.0", Source: "git", Enabled: true, Description: "AI-powered code review", Skills: 2, Agents: 1},
		{Name: "testing-helper", Version: "2.0.0", Source: "local", Enabled: false, Description: "Test generation helper"},
	}
}

func TestNew(t *testing.T) {
	m := New(testPlugins())
	if len(m.plugins) != 3 {
		t.Errorf("plugins = %d", len(m.plugins))
	}
}

func TestModel_View_List(t *testing.T) {
	m := New(testPlugins())
	v := m.View()
	if !strings.Contains(v, "Plugins") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "thinkback") {
		t.Error("should show first plugin")
	}
	if !strings.Contains(v, "3 installed") {
		t.Error("should show count")
	}
}

func TestModel_View_Empty(t *testing.T) {
	m := New(nil)
	v := m.View()
	if !strings.Contains(v, "No plugins installed") {
		t.Error("empty should show message")
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New(testPlugins())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d", m.cursor)
	}
}

func TestModel_DrillDown(t *testing.T) {
	m := New(testPlugins())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.level != 1 {
		t.Error("Enter should drill down")
	}
	if m.selected == nil || m.selected.Name != "thinkback" {
		t.Error("should select first plugin")
	}

	v := m.View()
	if !strings.Contains(v, "thinkback") {
		t.Error("detail should show name")
	}
	if !strings.Contains(v, "1.0.0") {
		t.Error("detail should show version")
	}
	if !strings.Contains(v, "enabled") {
		t.Error("detail should show status")
	}
}

func TestModel_Back(t *testing.T) {
	m := New(testPlugins())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.level != 0 {
		t.Error("Esc should go back")
	}
}

func TestModel_Close(t *testing.T) {
	m := New(testPlugins())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should close")
	}
	if _, ok := cmd().(DoneMsg); !ok {
		t.Error("should be DoneMsg")
	}
}

func TestModel_ToggleEnable(t *testing.T) {
	m := New(testPlugins())
	// Drill into first plugin (enabled)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.selected.Enabled {
		t.Error("first plugin should be enabled")
	}

	// Toggle
	m, _ = m.Update(tea.KeyPressMsg{Code: 'e'})
	if m.selected.Enabled {
		t.Error("should be disabled after toggle")
	}
	// Verify in list too
	if m.plugins[0].Enabled {
		t.Error("list should reflect toggle")
	}
}

func TestModel_VimKeys(t *testing.T) {
	m := New(testPlugins())
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Error("k should move up")
	}
}

func TestPluginInfo(t *testing.T) {
	p := PluginInfo{
		Name:        "test",
		Version:     "1.0",
		Source:      "local",
		Enabled:     true,
		Description: "A test plugin",
		Skills:      2,
		Agents:      1,
	}
	if p.Name != "test" {
		t.Error("wrong name")
	}
}
