package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOAuthFlow_InitialState(t *testing.T) {
	m := NewOAuthFlowModel("login", "")
	if m.State() != OAuthIdle {
		t.Errorf("initial state = %d, want OAuthIdle", m.State())
	}
	if m.IsDone() {
		t.Error("should not be done initially")
	}
}

func TestOAuthFlow_StepProgression(t *testing.T) {
	m := NewOAuthFlowModel("login", "")

	// Idle → ReadyToStart
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.State() != OAuthReadyToStart {
		t.Errorf("after enter: state = %d, want ReadyToStart", m.State())
	}

	// ReadyToStart → WaitingForLogin
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.State() != OAuthWaitingForLogin {
		t.Errorf("after second enter: state = %d, want WaitingForLogin", m.State())
	}
}

func TestOAuthFlow_EscapeCancels(t *testing.T) {
	m := NewOAuthFlowModel("login", "")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("escape should return a done cmd")
	}
	msg := cmd()
	done, ok := msg.(OAuthFlowDoneMsg)
	if !ok {
		t.Fatalf("expected OAuthFlowDoneMsg, got %T", msg)
	}
	if done.Success {
		t.Error("canceled flow should not be successful")
	}
}

func TestOAuthFlow_ViewIdle(t *testing.T) {
	m := NewOAuthFlowModel("login", "")
	v := m.View()
	if !strings.Contains(v, "Login") {
		t.Error("idle view should mention Login")
	}
}

func TestOAuthFlow_ViewWaiting(t *testing.T) {
	m := NewOAuthFlowModel("login", "")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v := m.View()
	if !strings.Contains(v, "Waiting") {
		t.Error("waiting view should mention Waiting")
	}
}

func TestOAuthFlow_StartMessage(t *testing.T) {
	m := NewOAuthFlowModel("login", "Starting fresh login...")
	v := m.View()
	if !strings.Contains(v, "Starting fresh login") {
		t.Error("should show start message")
	}
}
