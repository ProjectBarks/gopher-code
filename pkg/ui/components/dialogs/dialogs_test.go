package dialogs

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestConfirm_Yes(t *testing.T) {
	m := NewConfirm("Delete?", "Are you sure?", "delete-1")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	msg := cmd().(ConfirmMsg)
	if !msg.Confirmed {
		t.Error("y should confirm")
	}
	if msg.ID != "delete-1" {
		t.Errorf("ID = %q", msg.ID)
	}
}

func TestConfirm_No(t *testing.T) {
	m := NewConfirm("Delete?", "Are you sure?", "")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n'})
	msg := cmd().(ConfirmMsg)
	if msg.Confirmed {
		t.Error("n should not confirm")
	}
}

func TestConfirm_Escape(t *testing.T) {
	m := NewConfirm("", "", "")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := cmd().(ConfirmMsg)
	if msg.Confirmed {
		t.Error("escape should not confirm")
	}
}

func TestConfirm_EnterSelect(t *testing.T) {
	m := NewConfirm("Title", "Msg", "")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // move to No
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(ConfirmMsg)
	if msg.Confirmed {
		t.Error("enter on No should not confirm")
	}
}

func TestConfirm_View(t *testing.T) {
	m := NewConfirm("Reset?", "This cannot be undone", "")
	v := m.View()
	if !strings.Contains(v, "Reset") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "undone") {
		t.Error("should show message")
	}
}

func TestRenderToast(t *testing.T) {
	tests := []struct {
		level ToastLevel
		icon  string
	}{
		{ToastSuccess, "✓"},
		{ToastWarning, "⚠"},
		{ToastError, "✗"},
		{ToastInfo, "ℹ"},
	}
	for _, tt := range tests {
		got := RenderToast(Toast{Message: "Test", Level: tt.level})
		if !strings.Contains(got, tt.icon) {
			t.Errorf("level %q should contain %q", tt.level, tt.icon)
		}
	}
}

func TestRenderErrorAlert(t *testing.T) {
	got := RenderErrorAlert("Connection failed", "timeout after 10s")
	if !strings.Contains(got, "Connection failed") {
		t.Error("should contain title")
	}
	if !strings.Contains(got, "timeout") {
		t.Error("should contain detail")
	}
}

func TestRenderConfigError(t *testing.T) {
	got := RenderConfigError("settings.json", "invalid JSON at line 5")
	if !strings.Contains(got, "settings.json") {
		t.Error("should contain file path")
	}
}
