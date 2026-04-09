package interactive

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// --- LanguagePicker ---

func TestLanguagePicker_Submit(t *testing.T) {
	m := NewLanguagePicker("日本語")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(LanguageSelectedMsg)
	if msg.Language != "日本語" {
		t.Errorf("language = %q", msg.Language)
	}
}

func TestLanguagePicker_Cancel(t *testing.T) {
	m := NewLanguagePicker("")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := cmd().(LanguageCancelledMsg); !ok {
		t.Error("Esc should cancel")
	}
}

func TestLanguagePicker_Typing(t *testing.T) {
	m := NewLanguagePicker("")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'h'})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'i'})
	if m.value != "hi" {
		t.Errorf("value = %q", m.value)
	}
}

func TestLanguagePicker_View(t *testing.T) {
	m := NewLanguagePicker("")
	v := m.View()
	if !strings.Contains(v, "preferred response") {
		t.Error("should show prompt")
	}
}

// --- ExportDialog ---

func TestExportDialog_Select(t *testing.T) {
	m := NewExportDialog()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(ExportSelectedMsg)
	if msg.Format != ExportJSON {
		t.Errorf("format = %q, want json", msg.Format)
	}
}

func TestExportDialog_Navigation(t *testing.T) {
	m := NewExportDialog()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(ExportSelectedMsg)
	if msg.Format != ExportMarkdown {
		t.Errorf("format = %q, want markdown", msg.Format)
	}
}

func TestExportDialog_Cancel(t *testing.T) {
	m := NewExportDialog()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := cmd().(ExportCancelledMsg); !ok {
		t.Error("Esc should cancel")
	}
}

func TestExportDialog_View(t *testing.T) {
	m := NewExportDialog()
	v := m.View()
	if !strings.Contains(v, "Export") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "json") {
		t.Error("should show json format")
	}
}

// --- WorktreeExitDialog ---

func TestWorktreeExit_Keep(t *testing.T) {
	m := NewWorktreeExitDialog("my-wt", "feature-branch")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(WorktreeExitMsg)
	if msg.Action != WorktreeKeep {
		t.Errorf("action = %q", msg.Action)
	}
}

func TestWorktreeExit_Delete(t *testing.T) {
	m := NewWorktreeExitDialog("wt", "br")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(WorktreeExitMsg)
	if msg.Action != WorktreeDelete {
		t.Errorf("action = %q", msg.Action)
	}
}

func TestWorktreeExit_Cancel(t *testing.T) {
	m := NewWorktreeExitDialog("wt", "br")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := cmd().(WorktreeExitMsg)
	if msg.Action != WorktreeCancel {
		t.Error("Esc should cancel")
	}
}

func TestWorktreeExit_View(t *testing.T) {
	m := NewWorktreeExitDialog("my-worktree", "dev")
	v := m.View()
	if !strings.Contains(v, "my-worktree") {
		t.Error("should show worktree name")
	}
	if !strings.Contains(v, "dev") {
		t.Error("should show branch name")
	}
}

// --- ConfirmDialog ---

func TestConfirm_Yes(t *testing.T) {
	m := NewConfirmDialog("Delete?", "This cannot be undone", "del-1")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	msg := cmd().(ConfirmMsg)
	if !msg.Confirmed {
		t.Error("y should confirm")
	}
	if msg.ID != "del-1" {
		t.Errorf("id = %q", msg.ID)
	}
}

func TestConfirm_No(t *testing.T) {
	m := NewConfirmDialog("Delete?", "", "")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n'})
	msg := cmd().(ConfirmMsg)
	if msg.Confirmed {
		t.Error("n should not confirm")
	}
}

func TestConfirm_Enter(t *testing.T) {
	m := NewConfirmDialog("OK?", "", "")
	// cursor starts at 0 (Yes)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(ConfirmMsg)
	if !msg.Confirmed {
		t.Error("Enter on Yes should confirm")
	}

	// Move to No and enter
	m.cursor = 1
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg = cmd().(ConfirmMsg)
	if msg.Confirmed {
		t.Error("Enter on No should not confirm")
	}
}

func TestConfirm_Escape(t *testing.T) {
	m := NewConfirmDialog("?", "", "")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := cmd().(ConfirmMsg)
	if msg.Confirmed {
		t.Error("Esc should not confirm")
	}
}

func TestConfirm_View(t *testing.T) {
	m := NewConfirmDialog("Really delete?", "All data lost", "x")
	v := m.View()
	if !strings.Contains(v, "Really delete?") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "All data lost") {
		t.Error("should show message")
	}
	if !strings.Contains(v, "Yes") || !strings.Contains(v, "No") {
		t.Error("should show Yes/No")
	}
}
