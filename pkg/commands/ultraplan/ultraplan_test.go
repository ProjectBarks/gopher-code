package ultraplan

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew_Empty(t *testing.T) {
	m := New("")
	if m.step != StepUsage {
		t.Errorf("step = %q, want usage", m.step)
	}
}

func TestNew_WithBlurb(t *testing.T) {
	m := New("build a REST API")
	if m.step != StepChecking {
		t.Errorf("step = %q, want checking", m.step)
	}
	if m.blurb != "build a REST API" {
		t.Errorf("blurb = %q", m.blurb)
	}
}

func TestModel_LaunchSuccess(t *testing.T) {
	m := New("plan something")
	m, _ = m.Update(LaunchResultMsg{
		Success:    true,
		SessionURL: "https://claude.ai/code/session/abc",
		SessionID:  "sess-001",
	})
	if m.step != StepPolling {
		t.Errorf("step = %q, want polling", m.step)
	}
	if m.sessionURL != "https://claude.ai/code/session/abc" {
		t.Errorf("url = %q", m.sessionURL)
	}
}

func TestModel_LaunchError(t *testing.T) {
	m := New("plan")
	m, _ = m.Update(LaunchResultMsg{Success: false, Error: "not eligible"})
	if m.step != StepError {
		t.Errorf("step = %q", m.step)
	}
	if m.message != "not eligible" {
		t.Errorf("message = %q", m.message)
	}
}

func TestModel_PlanReady(t *testing.T) {
	m := New("plan")
	m, _ = m.Update(LaunchResultMsg{Success: true, SessionURL: "url"})
	m, _ = m.Update(PlanReadyMsg{Plan: "1. Design\n2. Implement"})
	if m.step != StepChoosing {
		t.Errorf("step = %q, want choosing", m.step)
	}
	if m.plan != "1. Design\n2. Implement" {
		t.Errorf("plan = %q", m.plan)
	}
}

func TestModel_ChooseTeleport(t *testing.T) {
	m := New("plan")
	m.step = StepChoosing
	m.plan = "plan content"
	m.cursor = 0 // teleport

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return cmd")
	}
	msg := cmd()
	exec, ok := msg.(ExecuteMsg)
	if !ok {
		t.Fatalf("expected ExecuteMsg, got %T", msg)
	}
	if exec.Choice != ChoiceTeleport {
		t.Errorf("choice = %q", exec.Choice)
	}
}

func TestModel_ChooseRemote(t *testing.T) {
	m := New("plan")
	m.step = StepChoosing
	m.cursor = 1 // remote

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(ExecuteMsg)
	if msg.Choice != ChoiceRemote {
		t.Errorf("choice = %q", msg.Choice)
	}
}

func TestModel_StopPolling(t *testing.T) {
	m := New("plan")
	m.step = StepPolling

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should stop")
	}
	msg := cmd()
	done, ok := msg.(DoneMsg)
	if !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
	if !strings.Contains(done.Result, "stopped") {
		t.Errorf("result = %q", done.Result)
	}
}

func TestModel_DismissUsage(t *testing.T) {
	m := New("")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("should dismiss")
	}
	if _, ok := cmd().(DoneMsg); !ok {
		t.Error("expected DoneMsg")
	}
}

func TestModel_View_Usage(t *testing.T) {
	m := New("")
	v := m.View()
	if !strings.Contains(v, "Ultraplan") {
		t.Error("should show title")
	}
	if !strings.Contains(v, "Usage") {
		t.Error("should show usage")
	}
	if !strings.Contains(v, CCRTermsURL) {
		t.Error("should show terms URL")
	}
}

func TestModel_View_Polling(t *testing.T) {
	m := New("plan")
	m.step = StepPolling
	m.sessionURL = "https://example.com/session"
	v := m.View()
	if !strings.Contains(v, "launched") {
		t.Error("should show launched status")
	}
	if !strings.Contains(v, "example.com") {
		t.Error("should show session URL")
	}
	if !strings.Contains(v, "free") {
		t.Error("should mention terminal is free")
	}
}

func TestModel_View_Choosing(t *testing.T) {
	m := New("plan")
	m.step = StepChoosing
	v := m.View()
	if !strings.Contains(v, "Plan Ready") {
		t.Error("should show plan ready")
	}
	if !strings.Contains(v, "teleport") {
		t.Error("should show teleport option")
	}
	if !strings.Contains(v, "cloud") {
		t.Error("should show cloud option")
	}
}

func TestModel_View_Error(t *testing.T) {
	m := New("plan")
	m.step = StepError
	m.message = "not eligible"
	v := m.View()
	if !strings.Contains(v, "Error") {
		t.Error("should show error")
	}
	if !strings.Contains(v, "not eligible") {
		t.Error("should show message")
	}
}

func TestBuildPrompt(t *testing.T) {
	got := BuildPrompt("build an API", "")
	if got != "build an API" {
		t.Errorf("without seed: %q", got)
	}

	got = BuildPrompt("refine this", "1. Step one\n2. Step two")
	if !strings.Contains(got, "draft plan") {
		t.Error("with seed should mention draft plan")
	}
	if !strings.Contains(got, "Step one") {
		t.Error("should contain seed plan")
	}
	if !strings.Contains(got, "refine this") {
		t.Error("should contain blurb")
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New("plan")
	m.step = StepChoosing
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d", m.cursor)
	}
}

func TestStep_Constants(t *testing.T) {
	if StepUsage != "usage" {
		t.Error("wrong")
	}
	if StepPolling != "polling" {
		t.Error("wrong")
	}
}
