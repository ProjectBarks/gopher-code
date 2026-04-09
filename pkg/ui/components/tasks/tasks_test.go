package tasks

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func testTasks() []Task {
	return []Task{
		{ID: "t1", Type: TaskTypeAgent, Label: "Explore codebase", Status: TaskRunning, Activity: "searching files", StartedAt: time.Now().Add(-30 * time.Second)},
		{ID: "t2", Type: TaskTypeShell, Label: "npm test", Status: TaskCompleted, StartedAt: time.Now().Add(-2 * time.Minute)},
		{ID: "t3", Type: TaskTypeAgent, Label: "Fix bug", Status: TaskFailed, HasError: true},
	}
}

func TestTaskStatus_IsTerminal(t *testing.T) {
	if TaskRunning.IsTerminal() {
		t.Error("running should not be terminal")
	}
	if !TaskCompleted.IsTerminal() {
		t.Error("completed should be terminal")
	}
	if !TaskFailed.IsTerminal() {
		t.Error("failed should be terminal")
	}
	if !TaskKilled.IsTerminal() {
		t.Error("killed should be terminal")
	}
}

func TestStatusIcon(t *testing.T) {
	if StatusIcon(TaskRunning, false, false) != "▶" {
		t.Error("running should be play")
	}
	if StatusIcon(TaskRunning, true, false) != "…" {
		t.Error("idle should be ellipsis")
	}
	if StatusIcon(TaskCompleted, false, false) != "✓" {
		t.Error("completed should be checkmark")
	}
	if StatusIcon(TaskFailed, false, false) != "✗" {
		t.Error("failed should be cross")
	}
	if StatusIcon(TaskRunning, false, true) != "✗" {
		t.Error("error should be cross regardless of status")
	}
}

func TestNew(t *testing.T) {
	m := New(testTasks())
	if len(m.tasks) != 3 {
		t.Errorf("tasks = %d", len(m.tasks))
	}
}

func TestModel_View_List(t *testing.T) {
	m := New(testTasks())
	v := m.View()
	if !strings.Contains(v, "Background Tasks") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "Explore codebase") {
		t.Error("should show first task")
	}
	if !strings.Contains(v, "npm test") {
		t.Error("should show second task")
	}
	if !strings.Contains(v, "1 running") {
		t.Error("should show running count")
	}
}

func TestModel_View_Empty(t *testing.T) {
	m := New(nil)
	v := m.View()
	if !strings.Contains(v, "No background tasks") {
		t.Error("empty should show message")
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New(testTasks())
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
	m := New(testTasks())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.level != 1 {
		t.Error("Enter should drill to detail")
	}
	if m.selected == nil || m.selected.ID != "t1" {
		t.Error("should select first task")
	}

	v := m.View()
	if !strings.Contains(v, "Explore codebase") {
		t.Error("detail should show task label")
	}
	if !strings.Contains(v, "running") {
		t.Error("detail should show status")
	}
}

func TestModel_Back(t *testing.T) {
	m := New(testTasks())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.level != 0 {
		t.Error("Esc should go back to list")
	}
}

func TestModel_Stop(t *testing.T) {
	m := New(testTasks()) // first task is running
	_, cmd := m.Update(tea.KeyPressMsg{Code: 's'})
	if cmd == nil {
		t.Fatal("s on running task should return cmd")
	}
	msg := cmd()
	stop, ok := msg.(StopTaskMsg)
	if !ok {
		t.Fatalf("expected StopTaskMsg, got %T", msg)
	}
	if stop.TaskID != "t1" {
		t.Errorf("taskID = %q", stop.TaskID)
	}
}

func TestModel_Close(t *testing.T) {
	m := New(testTasks())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should close")
	}
	if _, ok := cmd().(DoneMsg); !ok {
		t.Error("expected DoneMsg")
	}
}

func TestModel_ViewTask(t *testing.T) {
	m := New(testTasks())
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // drill down
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'v'})
	if cmd == nil {
		t.Fatal("v should view task")
	}
	msg := cmd()
	view, ok := msg.(ViewTaskMsg)
	if !ok {
		t.Fatalf("expected ViewTaskMsg, got %T", msg)
	}
	if view.TaskID != "t1" {
		t.Errorf("taskID = %q", view.TaskID)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
