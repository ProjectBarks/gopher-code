package screens

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
)

func TestResumeModel_Init(t *testing.T) {
	m := NewResumeModel(nil)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil cmd")
	}
}

func TestResumeModel_EmptySessions(t *testing.T) {
	m := NewResumeModel(nil)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	v := m.View()
	if !strings.Contains(v.Content, "No previous sessions found") {
		t.Error("expected no-sessions message")
	}
}

func TestResumeModel_RendersSessions(t *testing.T) {
	sessions := []session.SessionMetadata{
		{
			ID:        "abc12345-6789-0000-0000-000000000001",
			Name:      "My Session",
			CWD:       "/home/user/project",
			UpdatedAt: time.Now().Add(-2 * time.Hour),
			TurnCount: 5,
		},
		{
			ID:        "def12345-6789-0000-0000-000000000002",
			CWD:       "/tmp/other",
			UpdatedAt: time.Now().Add(-48 * time.Hour),
			TurnCount: 12,
		},
	}
	m := NewResumeModel(sessions)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	v := m.View()

	if !strings.Contains(v.Content, "Resume Conversation") {
		t.Error("expected header")
	}
	if !strings.Contains(v.Content, "My Session") {
		t.Error("expected session name")
	}
	if !strings.Contains(v.Content, "/home/user/project") {
		t.Error("expected CWD")
	}
	if !strings.Contains(v.Content, "2 session(s) available") {
		t.Error("expected session count")
	}
}

func TestResumeModel_Navigation(t *testing.T) {
	sessions := []session.SessionMetadata{
		{ID: "aaa", Name: "First"},
		{ID: "bbb", Name: "Second"},
		{ID: "ccc", Name: "Third"},
	}
	m := NewResumeModel(sessions)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})

	// Start at 0
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	// Down
	m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	// Down again
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.cursor)
	}

	// Down past end should clamp
	m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", m.cursor)
	}

	// Up
	m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	// Up via arrow
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	// Up past start should clamp
	m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.cursor)
	}
}

func TestResumeModel_EnterSelects(t *testing.T) {
	sessions := []session.SessionMetadata{
		{ID: "session-abc"},
		{ID: "session-def"},
	}
	m := NewResumeModel(sessions)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})

	// Move to second item
	m.Update(tea.KeyPressMsg{Code: 'j'})

	// Press Enter
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a cmd")
	}
	msg := cmd()
	sel, ok := msg.(ResumeSelectMsg)
	if !ok {
		t.Fatalf("expected ResumeSelectMsg, got %T", msg)
	}
	if sel.SessionID != "session-def" {
		t.Errorf("expected session-def, got %q", sel.SessionID)
	}
}

func TestResumeModel_EscapeDismisses(t *testing.T) {
	m := NewResumeModel(nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Escape should produce a cmd")
	}
	msg := cmd()
	if _, ok := msg.(ResumeDoneMsg); !ok {
		t.Errorf("expected ResumeDoneMsg, got %T", msg)
	}
}

func TestResumeModel_CtrlCDismisses(t *testing.T) {
	m := NewResumeModel(nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C should produce a cmd")
	}
	msg := cmd()
	if _, ok := msg.(ResumeDoneMsg); !ok {
		t.Errorf("expected ResumeDoneMsg, got %T", msg)
	}
}

func TestResumeModel_QDismisses(t *testing.T) {
	m := NewResumeModel(nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil {
		t.Fatal("q should produce a cmd")
	}
	msg := cmd()
	if _, ok := msg.(ResumeDoneMsg); !ok {
		t.Errorf("expected ResumeDoneMsg, got %T", msg)
	}
}

func TestResumeModel_ZeroSize(t *testing.T) {
	m := NewResumeModel(nil)
	v := m.View()
	if !strings.Contains(v.Content, "Loading sessions") {
		t.Error("zero-size should show loading message")
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5 minutes ago"},
		{now.Add(-1 * time.Minute), "1 minute ago"},
		{now.Add(-3 * time.Hour), "3 hours ago"},
		{now.Add(-1 * time.Hour), "1 hour ago"},
		{now.Add(-2 * 24 * time.Hour), "2 days ago"},
		{now.Add(-1 * 24 * time.Hour), "1 day ago"},
		{now.Add(-14 * 24 * time.Hour), "2 weeks ago"},
		{now.Add(-60 * 24 * time.Hour), "2 months ago"},
		{time.Time{}, "unknown"},
	}
	for _, tt := range tests {
		got := formatAge(tt.t)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestShortID(t *testing.T) {
	if got := shortID("abc12345-6789-0000"); got != "abc12345" {
		t.Errorf("expected abc12345, got %q", got)
	}
	if got := shortID("short"); got != "short" {
		t.Errorf("expected short, got %q", got)
	}
}
