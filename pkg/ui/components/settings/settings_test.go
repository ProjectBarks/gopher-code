package settings

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testStatusData() StatusData {
	return StatusData{
		Primary: []Property{
			{Label: "Version", Value: "1.0.0"},
			{Label: "Session ID", Value: "abc-123"},
		},
		Secondary: []Property{
			{Label: "Model", Value: "Claude Sonnet 4.6"},
		},
		Diagnostics: []Diagnostic{
			{Name: "Git", Status: "ok", Message: "v2.42.0"},
			{Name: "Node", Status: "warning", Message: "v16 (v18+ recommended)"},
			{Name: "Sandbox", Status: "error", Message: "bwrap not found"},
		},
	}
}

func testConfigs() []ConfigEntry {
	return []ConfigEntry{
		{Key: "model", Value: "claude-sonnet-4-6-20250514", Source: "user", Description: "The default model to use"},
		{Key: "theme", Value: "dark", Source: "default"},
	}
}

func testUsage() *UsageInfo {
	return &UsageInfo{
		PlanName:      "Pro",
		UsedPercent:   42.5,
		ResetDate:     "2026-04-16",
		TotalTokens:   1_500_000,
		SessionTokens: 25_000,
	}
}

func TestNew_DefaultTab(t *testing.T) {
	m := New(TabStatus, StatusData{}, nil, nil)
	if m.tab != TabStatus {
		t.Errorf("tab = %q, want Status", m.tab)
	}

	m = New(TabConfig, StatusData{}, nil, nil)
	if m.tab != TabConfig {
		t.Errorf("tab = %q, want Config", m.tab)
	}

	m = New(TabUsage, StatusData{}, nil, nil)
	if m.tab != TabUsage {
		t.Errorf("tab = %q, want Usage", m.tab)
	}
}

func TestModel_TabSwitch(t *testing.T) {
	m := New(TabStatus, StatusData{}, nil, nil)
	if m.tabIndex != 0 {
		t.Errorf("tabIndex = %d, want 0", m.tabIndex)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != TabConfig {
		t.Errorf("tab = %q after Tab, want Config", m.tab)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != TabUsage {
		t.Errorf("tab = %q, want Usage", m.tab)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.tab != TabStatus {
		t.Error("should wrap back to Status")
	}
}

func TestModel_Close(t *testing.T) {
	m := New(TabStatus, StatusData{}, nil, nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Escape should return cmd")
	}
	msg := cmd()
	closed, ok := msg.(ClosedMsg)
	if !ok {
		t.Fatalf("expected ClosedMsg, got %T", msg)
	}
	if closed.Result == "" {
		t.Error("result should not be empty")
	}
}

func TestModel_View_StatusTab(t *testing.T) {
	m := New(TabStatus, testStatusData(), nil, nil)
	v := m.View()

	if !strings.Contains(v, "[Status]") {
		t.Error("should highlight active Status tab")
	}
	if !strings.Contains(v, "Version") {
		t.Error("should show Version property")
	}
	if !strings.Contains(v, "1.0.0") {
		t.Error("should show version value")
	}
	if !strings.Contains(v, "Git") {
		t.Error("should show Git diagnostic")
	}
	if !strings.Contains(v, "bwrap not found") {
		t.Error("should show error diagnostic message")
	}
}

func TestModel_View_ConfigTab(t *testing.T) {
	m := New(TabConfig, StatusData{}, testConfigs(), nil)
	v := m.View()

	if !strings.Contains(v, "[Config]") {
		t.Error("should highlight active Config tab")
	}
	if !strings.Contains(v, "model") {
		t.Error("should show model setting")
	}
	if !strings.Contains(v, "[user]") {
		t.Error("should show source badge for non-default")
	}
	if !strings.Contains(v, "The default model") {
		t.Error("should show description")
	}
}

func TestModel_View_ConfigTabEmpty(t *testing.T) {
	m := New(TabConfig, StatusData{}, nil, nil)
	v := m.View()
	if !strings.Contains(v, "No configuration") {
		t.Error("empty config should show message")
	}
}

func TestModel_View_UsageTab(t *testing.T) {
	m := New(TabUsage, StatusData{}, nil, testUsage())
	v := m.View()

	if !strings.Contains(v, "[Usage]") {
		t.Error("should highlight active Usage tab")
	}
	if !strings.Contains(v, "Pro") {
		t.Error("should show plan name")
	}
	if !strings.Contains(v, "42.5%") {
		t.Error("should show usage percentage")
	}
	if !strings.Contains(v, "█") {
		t.Error("should show usage bar")
	}
	if !strings.Contains(v, "25.0K") {
		t.Error("should show session tokens")
	}
}

func TestModel_View_UsageTabNoData(t *testing.T) {
	m := New(TabUsage, StatusData{}, nil, nil)
	v := m.View()
	if !strings.Contains(v, "No usage data") {
		t.Error("nil usage should show message")
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{500, "500"},
		{1_500, "1.5K"},
		{25_000, "25.0K"},
		{1_500_000, "1.5M"},
		{0, "0"},
	}
	for _, tt := range tests {
		got := formatTokens(tt.n)
		if got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestCurrentTab(t *testing.T) {
	m := New(TabConfig, StatusData{}, nil, nil)
	if m.CurrentTab() != TabConfig {
		t.Errorf("CurrentTab = %q", m.CurrentTab())
	}
}

func TestModel_View_StatusEmpty(t *testing.T) {
	m := New(TabStatus, StatusData{}, nil, nil)
	v := m.View()
	if !strings.Contains(v, "No status data") {
		t.Error("empty status should show message")
	}
}
