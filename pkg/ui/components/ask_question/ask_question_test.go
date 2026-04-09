package ask_question

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New([]Question{{Text: "What is your name?"}})
	if m.QuestionCount() != 1 {
		t.Errorf("count = %d", m.QuestionCount())
	}
	if m.CurrentQuestionIndex() != 0 {
		t.Error("should start at index 0")
	}
}

func TestModel_FreeText_Submit(t *testing.T) {
	m := New([]Question{{Text: "Favorite color?"}})

	// Type answer
	for _, ch := range "blue" {
		m, _ = m.Update(tea.KeyPressMsg{Code: ch})
	}
	if m.textInput != "blue" {
		t.Errorf("textInput = %q", m.textInput)
	}

	// Submit
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should submit")
	}
	msg := cmd()
	sub, ok := msg.(AnswerSubmittedMsg)
	if !ok {
		t.Fatalf("expected AnswerSubmittedMsg, got %T", msg)
	}
	if sub.Answers["Favorite color?"] != "blue" {
		t.Errorf("answer = %q", sub.Answers["Favorite color?"])
	}
}

func TestModel_Options_Select(t *testing.T) {
	m := New([]Question{{
		Text: "Pick one",
		Options: []QuestionOption{
			{Label: "Yes"},
			{Label: "No"},
			{Label: "Maybe"},
		},
	}})

	// Move to "No"
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d", m.cursor)
	}

	// Select
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	sub, ok := msg.(AnswerSubmittedMsg)
	if !ok {
		t.Fatalf("expected AnswerSubmittedMsg, got %T", msg)
	}
	if sub.Answers["Pick one"] != "No" {
		t.Errorf("answer = %q", sub.Answers["Pick one"])
	}
}

func TestModel_MultiQuestion_Navigation(t *testing.T) {
	m := New([]Question{
		{Text: "Q1", Options: []QuestionOption{{Label: "A"}, {Label: "B"}}},
		{Text: "Q2"},
	})

	// Answer Q1
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // select "A"
	if m.CurrentQuestionIndex() != 1 {
		t.Errorf("should advance to Q2, index = %d", m.CurrentQuestionIndex())
	}

	// Answer Q2
	for _, ch := range "hi" {
		m, _ = m.Update(tea.KeyPressMsg{Code: ch})
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	sub, ok := msg.(AnswerSubmittedMsg)
	if !ok {
		t.Fatalf("expected AnswerSubmittedMsg, got %T", msg)
	}
	if sub.Answers["Q1"] != "A" || sub.Answers["Q2"] != "hi" {
		t.Errorf("answers = %v", sub.Answers)
	}
}

func TestModel_Cancel(t *testing.T) {
	m := New([]Question{{Text: "?"}})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should return cmd")
	}
	if _, ok := cmd().(AnswerCancelledMsg); !ok {
		t.Error("expected AnswerCancelledMsg")
	}
}

func TestModel_Backspace(t *testing.T) {
	m := New([]Question{{Text: "?"}})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a'})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'b'})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.textInput != "a" {
		t.Errorf("after backspace: %q", m.textInput)
	}
}

func TestModel_View_FreeText(t *testing.T) {
	m := New([]Question{{Text: "What is your name?"}})
	v := m.View()
	if !strings.Contains(v, "What is your name?") {
		t.Error("should show question")
	}
	if !strings.Contains(v, "Type your answer") {
		t.Error("should show placeholder")
	}
}

func TestModel_View_Options(t *testing.T) {
	m := New([]Question{{
		Text:    "Choose",
		Options: []QuestionOption{{Label: "A"}, {Label: "B"}},
	}})
	v := m.View()
	if !strings.Contains(v, "Choose") {
		t.Error("should show question")
	}
	if !strings.Contains(v, "A") || !strings.Contains(v, "B") {
		t.Error("should show options")
	}
}

func TestModel_View_MultiQuestion(t *testing.T) {
	m := New([]Question{{Text: "Q1"}, {Text: "Q2"}})
	v := m.View()
	if !strings.Contains(v, "1 of 2") {
		t.Error("should show question counter")
	}
}

func TestModel_EmptyInput_DefaultAnswer(t *testing.T) {
	m := New([]Question{{Text: "?"}})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(AnswerSubmittedMsg)
	if msg.Answers["?"] != "(no answer)" {
		t.Errorf("empty input should default to '(no answer)', got %q", msg.Answers["?"])
	}
}

func TestModel_VimKeys(t *testing.T) {
	m := New([]Question{{
		Text:    "?",
		Options: []QuestionOption{{Label: "A"}, {Label: "B"}, {Label: "C"}},
	}})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Error("k should move up")
	}
}
