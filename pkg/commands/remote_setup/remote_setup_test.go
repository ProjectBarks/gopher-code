package remote_setup

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New()
	if m.step != StepChecking {
		t.Errorf("step = %q, want checking", m.step)
	}
}

func TestModel_CheckResult_NoGhCLI(t *testing.T) {
	m := New()
	m, _ = m.Update(CheckResultMsg{Result: CheckResult{
		Step:    StepNoGhCLI,
		Message: "gh not found",
	}})
	if m.step != StepNoGhCLI {
		t.Errorf("step = %q", m.step)
	}
	v := m.View()
	if !strings.Contains(v, "not found") {
		t.Error("should show gh not found")
	}
}

func TestModel_CheckResult_NoGhAuth(t *testing.T) {
	m := New()
	m, _ = m.Update(CheckResultMsg{Result: CheckResult{
		Step:    StepNoGhAuth,
		Message: "not authenticated",
	}})
	if m.step != StepNoGhAuth {
		t.Errorf("step = %q", m.step)
	}
	v := m.View()
	if !strings.Contains(v, "not authenticated") {
		t.Error("should show auth message")
	}
}

func TestModel_CheckResult_HasToken(t *testing.T) {
	m := New()
	m, _ = m.Update(CheckResultMsg{Result: CheckResult{
		Step:  StepConfirm,
		Token: "ghp_abc123",
	}})
	if m.step != StepConfirm {
		t.Errorf("step = %q", m.step)
	}
	v := m.View()
	if !strings.Contains(v, "Continue") {
		t.Error("confirm should show Continue prompt")
	}
	if !strings.Contains(v, "Import your GitHub token") {
		t.Error("should explain what will happen")
	}
}

func TestModel_Confirm_Yes(t *testing.T) {
	m := New()
	m, _ = m.Update(CheckResultMsg{Result: CheckResult{Step: StepConfirm, Token: "token"}})

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	if m.step != StepUploading {
		t.Errorf("step = %q, want uploading", m.step)
	}
	if cmd == nil {
		t.Fatal("should return upload cmd")
	}
}

func TestModel_Confirm_No(t *testing.T) {
	m := New()
	m, _ = m.Update(CheckResultMsg{Result: CheckResult{Step: StepConfirm}})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n'})
	if cmd == nil {
		t.Fatal("n should return done cmd")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
}

func TestModel_UploadSuccess(t *testing.T) {
	m := New()
	m.step = StepUploading
	m, _ = m.Update(UploadResultMsg{Success: true, Message: "Token imported"})
	if m.step != StepSuccess {
		t.Errorf("step = %q", m.step)
	}
	v := m.View()
	if !strings.Contains(v, "complete") {
		t.Error("should show success")
	}
	if !strings.Contains(v, CodeWebURL) {
		t.Error("should show web URL")
	}
}

func TestModel_UploadError(t *testing.T) {
	m := New()
	m.step = StepUploading
	m, _ = m.Update(UploadResultMsg{Success: false, Message: "server error"})
	if m.step != StepError {
		t.Errorf("step = %q", m.step)
	}
	v := m.View()
	if !strings.Contains(v, "failed") {
		t.Error("should show failure")
	}
}

func TestModel_DismissOnAnyKey(t *testing.T) {
	for _, step := range []Step{StepSuccess, StepError, StepNoGhCLI, StepNoGhAuth} {
		m := New()
		m.step = step
		m.message = "test"
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Errorf("step %q: any key should dismiss", step)
			continue
		}
		if _, ok := cmd().(DoneMsg); !ok {
			t.Errorf("step %q: expected DoneMsg", step)
		}
	}
}

func TestModel_View_Checking(t *testing.T) {
	m := New()
	v := m.View()
	if !strings.Contains(v, "Checking") {
		t.Error("checking should show progress")
	}
}

func TestModel_View_Uploading(t *testing.T) {
	m := New()
	m.step = StepUploading
	v := m.View()
	if !strings.Contains(v, "Importing") {
		t.Error("uploading should show progress")
	}
}

func TestStep_Constants(t *testing.T) {
	if StepChecking != "checking" {
		t.Error("wrong")
	}
	if StepSuccess != "success" {
		t.Error("wrong")
	}
}

func TestCodeWebURL(t *testing.T) {
	if CodeWebURL != "https://claude.ai/code" {
		t.Error("wrong URL")
	}
}
