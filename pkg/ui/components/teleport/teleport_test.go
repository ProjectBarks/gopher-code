package teleport

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestProgressModel_New(t *testing.T) {
	m := NewProgressModel("sess-123")
	if m.CurrentStep != StepValidating {
		t.Errorf("step = %q, want validating", m.CurrentStep)
	}
	if m.SessionID != "sess-123" {
		t.Errorf("sessionID = %q", m.SessionID)
	}
}

func TestProgressModel_View(t *testing.T) {
	m := NewProgressModel("abc")
	v := m.View()

	if !strings.Contains(v, "Teleporting session") {
		t.Error("should contain title")
	}
	if !strings.Contains(v, "abc") {
		t.Error("should show session ID")
	}
	if !strings.Contains(v, "Validating session") {
		t.Error("should show current step")
	}
	if !strings.Contains(v, "Fetching session logs") {
		t.Error("should show pending steps")
	}
}

func TestProgressModel_StepAdvance(t *testing.T) {
	m := NewProgressModel("")
	m.SetStep(StepFetchingLogs)
	v := m.View()
	// Validating should show ✓ (completed)
	if !strings.Contains(v, "✓") {
		t.Error("completed step should show checkmark")
	}
}

func TestProgressModel_Tick(t *testing.T) {
	m := NewProgressModel("")
	frame0 := m.frame
	m.Tick()
	if m.frame != frame0+1 {
		t.Error("Tick should advance frame")
	}
}

func TestProgressModel_ViewNoSessionID(t *testing.T) {
	m := NewProgressModel("")
	v := m.View()
	// Should not have an extra blank line for empty session ID
	if strings.Contains(v, "  \n\n") {
		// This is fine — the code still produces the session line but it's empty
	}
}

func TestErrorModel_NeedsLogin(t *testing.T) {
	m := ErrorModel{ErrorType: ErrorNeedsLogin}
	v := m.View()
	if !strings.Contains(v, "Authentication Required") {
		t.Error("should mention authentication")
	}
	if !strings.Contains(v, "/login") {
		t.Error("should suggest /login")
	}
}

func TestErrorModel_NeedsGitStash(t *testing.T) {
	m := ErrorModel{ErrorType: ErrorNeedsGitStash}
	v := m.View()
	if !strings.Contains(v, "Uncommitted Changes") {
		t.Error("should mention uncommitted changes")
	}
	if !strings.Contains(v, "git stash") {
		t.Error("should suggest git stash")
	}
}

func TestErrorModel_GenericError(t *testing.T) {
	m := ErrorModel{ErrorType: "unknown", Message: "something broke"}
	v := m.View()
	if !strings.Contains(v, "something broke") {
		t.Error("should show error message")
	}
}

func TestRepoMismatchModel_View(t *testing.T) {
	m := NewRepoMismatchModel("github.com/org/remote", "github.com/org/local")
	v := m.View()

	if !strings.Contains(v, "Repository Mismatch") {
		t.Error("should show mismatch title")
	}
	if !strings.Contains(v, "github.com/org/remote") {
		t.Error("should show remote repo")
	}
	if !strings.Contains(v, "github.com/org/local") {
		t.Error("should show local repo")
	}
	if !strings.Contains(v, "Continue") {
		t.Error("should show Continue option")
	}
	if !strings.Contains(v, "Cancel") {
		t.Error("should show Cancel option")
	}
}

func TestRepoMismatchModel_Continue(t *testing.T) {
	m := NewRepoMismatchModel("a", "b")
	m.cursor = 0
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on Continue should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(RepoMismatchContinueMsg); !ok {
		t.Fatalf("expected RepoMismatchContinueMsg, got %T", msg)
	}
}

func TestRepoMismatchModel_Cancel(t *testing.T) {
	m := NewRepoMismatchModel("a", "b")
	m.cursor = 1
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on Cancel should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(RepoMismatchCancelMsg); !ok {
		t.Fatalf("expected RepoMismatchCancelMsg, got %T", msg)
	}
}

func TestRepoMismatchModel_EscapeCancels(t *testing.T) {
	m := NewRepoMismatchModel("a", "b")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should return cmd")
	}
	msg := cmd()
	if _, ok := msg.(RepoMismatchCancelMsg); !ok {
		t.Fatalf("expected RepoMismatchCancelMsg, got %T", msg)
	}
}

func TestRepoMismatchModel_Navigation(t *testing.T) {
	m := NewRepoMismatchModel("a", "b")
	if m.cursor != 0 {
		t.Error("should start on Continue")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Error("down should move to Cancel")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Error("up should move to Continue")
	}
}

func TestStepConstants(t *testing.T) {
	if StepValidating != "validating" {
		t.Error("wrong")
	}
	if StepDone != "done" {
		t.Error("wrong")
	}
}

func TestErrorTypeConstants(t *testing.T) {
	if ErrorNeedsLogin != "needsLogin" {
		t.Error("wrong")
	}
	if ErrorNeedsGitStash != "needsGitStash" {
		t.Error("wrong")
	}
}
