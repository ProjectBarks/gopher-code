package feedback_survey

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFeedback_InitialState(t *testing.T) {
	m := New("")
	if m.GetState() != StateOpen {
		t.Error("should start open")
	}
	if m.IsClosed() {
		t.Error("should not be closed")
	}
}

func TestFeedback_RateWithNumber(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: '4'})
	if m.GetState() != StateThanks {
		t.Errorf("state = %q, want thanks", m.GetState())
	}
	if m.selected != 3 {
		t.Errorf("selected = %d, want 3 (rating 4)", m.selected)
	}
}

func TestFeedback_Submit(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: '5'}) // rate 5
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // submit
	if m.GetState() != StateSubmitted {
		t.Errorf("state = %q, want submitted", m.GetState())
	}
	if cmd == nil {
		t.Fatal("should return submit cmd")
	}
	msg := cmd()
	sub, ok := msg.(FeedbackSubmittedMsg)
	if !ok {
		t.Fatalf("expected FeedbackSubmittedMsg, got %T", msg)
	}
	if sub.Response.Rating != 5 {
		t.Errorf("rating = %d, want 5", sub.Response.Rating)
	}
}

func TestFeedback_Dismiss(t *testing.T) {
	m := New("")
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.GetState() != StateClosed {
		t.Error("escape should close")
	}
	if cmd == nil {
		t.Fatal("should return dismiss cmd")
	}
	if _, ok := cmd().(FeedbackDismissedMsg); !ok {
		t.Error("should be FeedbackDismissedMsg")
	}
	if !m.IsClosed() {
		t.Error("should be closed")
	}
}

func TestFeedback_Navigate(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.selected != 1 {
		t.Errorf("selected = %d, want 1", m.selected)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.selected != 0 {
		t.Errorf("selected = %d, want 0", m.selected)
	}
}

func TestFeedback_ViewOpen(t *testing.T) {
	m := New("How was this?")
	v := m.View()
	if v == "" {
		t.Error("should produce output")
	}
}
