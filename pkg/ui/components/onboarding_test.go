package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOnboarding_InitialState(t *testing.T) {
	m := NewOnboardingModel()
	if m.Step() != StepWelcome {
		t.Errorf("initial step = %d, want StepWelcome", m.Step())
	}
	if m.IsDone() {
		t.Error("should not be done initially")
	}
}

func TestOnboarding_StepProgression(t *testing.T) {
	m := NewOnboardingModel()

	// Welcome → Theme
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Step() != StepTheme {
		t.Errorf("after first enter: step = %d, want StepTheme", m.Step())
	}

	// Theme → Security
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Step() != StepSecurity {
		t.Errorf("after second enter: step = %d, want StepSecurity", m.Step())
	}

	// Security → Done
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.IsDone() {
		t.Error("should be done after security step")
	}
	if cmd == nil {
		t.Error("should return OnboardingDoneMsg cmd")
	}
}

func TestOnboarding_EscapeSkips(t *testing.T) {
	m := NewOnboardingModel()
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.IsDone() {
		t.Error("escape should skip to done")
	}
	if cmd == nil {
		t.Error("should return done cmd on escape")
	}
}

func TestOnboarding_ViewWelcome(t *testing.T) {
	m := NewOnboardingModel()
	v := m.View()
	if !strings.Contains(v, "Welcome") {
		t.Error("welcome view should contain 'Welcome'")
	}
}

func TestOnboarding_ViewTheme(t *testing.T) {
	m := NewOnboardingModel()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v := m.View()
	if !strings.Contains(v, "theme") || !strings.Contains(v, "Dark") {
		t.Error("theme view should mention theme options")
	}
}

func TestOnboarding_ViewSecurity(t *testing.T) {
	m := NewOnboardingModel()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v := m.View()
	if !strings.Contains(v, "Security") {
		t.Error("security view should contain 'Security'")
	}
}
