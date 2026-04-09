package teams

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func testTeammates() []Teammate {
	return []Teammate{
		{Name: "worker-1", AgentType: "general-purpose", Status: StatusRunning, Color: "blue", Model: "sonnet"},
		{Name: "explorer", AgentType: "Explore", Status: StatusIdle, Color: "green", IdleSince: time.Now().Add(-5 * time.Minute)},
		{Name: "planner", AgentType: "Plan", Status: StatusUnknown, IsHidden: true},
	}
}

func TestNew(t *testing.T) {
	m := New("my-team", testTeammates())
	if m.teamName != "my-team" {
		t.Errorf("teamName = %q", m.teamName)
	}
	if len(m.teammates) != 3 {
		t.Errorf("teammates = %d", len(m.teammates))
	}
	if m.level != levelList {
		t.Error("should start at list level")
	}
}

func TestModel_View_List(t *testing.T) {
	m := New("dev-team", testTeammates())
	v := m.View()

	if !strings.Contains(v, "dev-team") {
		t.Error("should show team name")
	}
	if !strings.Contains(v, "worker-1") {
		t.Error("should show first teammate")
	}
	if !strings.Contains(v, "explorer") {
		t.Error("should show second teammate")
	}
	if !strings.Contains(v, "[hidden]") {
		t.Error("should show hidden marker")
	}
	if !strings.Contains(v, "3 members") {
		t.Error("should show member count")
	}
}

func TestModel_View_Empty(t *testing.T) {
	m := New("empty", nil)
	v := m.View()
	if !strings.Contains(v, "No teammates") {
		t.Error("empty team should show message")
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New("t", testTeammates())

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestModel_DrillDown(t *testing.T) {
	m := New("t", testTeammates())

	// Select first teammate
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.level != levelDetail {
		t.Error("Enter should drill down to detail")
	}
	if m.selected == nil || m.selected.Name != "worker-1" {
		t.Error("should select first teammate")
	}

	// View should show detail
	v := m.View()
	if !strings.Contains(v, "worker-1") {
		t.Error("detail should show name")
	}
	if !strings.Contains(v, "running") {
		t.Error("detail should show status")
	}
	if !strings.Contains(v, "sonnet") {
		t.Error("detail should show model")
	}
}

func TestModel_BackFromDetail(t *testing.T) {
	m := New("t", testTeammates())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // drill down

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // back
	if m.level != levelList {
		t.Error("Esc should go back to list")
	}
	if m.selected != nil {
		t.Error("selected should be cleared")
	}
}

func TestModel_Close(t *testing.T) {
	m := New("t", testTeammates())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc at list should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
}

func TestModel_VimKeys(t *testing.T) {
	m := New("t", testTeammates())
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Error("k should move up")
	}
}

func TestModel_DetailWithIdleSince(t *testing.T) {
	teammates := []Teammate{
		{Name: "idle-one", Status: StatusIdle, IdleSince: time.Now().Add(-10 * time.Minute)},
	}
	m := New("t", teammates)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v := m.View()
	if !strings.Contains(v, "Idle since") {
		t.Error("should show idle since")
	}
}

func TestTeammateStatus_Constants(t *testing.T) {
	if StatusRunning != "running" {
		t.Error("wrong")
	}
	if StatusIdle != "idle" {
		t.Error("wrong")
	}
	if StatusUnknown != "unknown" {
		t.Error("wrong")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
