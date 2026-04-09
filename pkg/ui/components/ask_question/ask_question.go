// Package ask_question provides the AskUserQuestion permission UI.
//
// Source: components/permissions/AskUserQuestionPermissionRequest/*.tsx
//
// When Claude needs user input (via AskUserQuestion tool), this component
// shows the question(s) with optional multiple-choice options or free-text
// input. Supports multi-question navigation and submit.
package ask_question

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Question is a question from the model to the user.
type Question struct {
	Text    string
	Options []QuestionOption // nil = free-text input
}

// QuestionOption is a selectable answer option.
type QuestionOption struct {
	Label       string
	Description string
}

// AnswerSubmittedMsg is sent when the user submits all answers.
type AnswerSubmittedMsg struct {
	Answers map[string]string // question text → answer
}

// AnswerCancelledMsg is sent when the user cancels.
type AnswerCancelledMsg struct{}

// Model is the AskUserQuestion bubbletea model.
type Model struct {
	questions    []Question
	answers      map[string]string
	currentIndex int
	cursor       int    // for option selection
	textInput    string // for free-text input
	width        int
}

// New creates a question dialog from the given questions.
func New(questions []Question) Model {
	return Model{
		questions: questions,
		answers:   make(map[string]string),
		width:     80,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	q := m.currentQuestion()
	if q == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if len(q.Options) > 0 {
			return m.updateOptions(msg)
		}
		return m.updateFreeText(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

func (m Model) updateOptions(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	q := m.currentQuestion()
	switch msg.Code {
	case tea.KeyUp, 'k':
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown, 'j':
		if m.cursor < len(q.Options)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		// Select option and advance
		m.answers[q.Text] = q.Options[m.cursor].Label
		return m.advance()
	case tea.KeyEscape:
		return m, func() tea.Msg { return AnswerCancelledMsg{} }
	case tea.KeyTab:
		return m.advance()
	}
	return m, nil
}

func (m Model) updateFreeText(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEnter:
		q := m.currentQuestion()
		answer := strings.TrimSpace(m.textInput)
		if answer == "" {
			answer = "(no answer)"
		}
		m.answers[q.Text] = answer
		m.textInput = ""
		return m.advance()
	case tea.KeyEscape:
		return m, func() tea.Msg { return AnswerCancelledMsg{} }
	case tea.KeyBackspace:
		if len(m.textInput) > 0 {
			m.textInput = m.textInput[:len(m.textInput)-1]
		}
	default:
		if msg.Code >= 32 && msg.Code < 127 && msg.Mod == 0 {
			m.textInput += string(rune(msg.Code))
		}
	}
	return m, nil
}

// advance moves to the next question or submits if done.
func (m Model) advance() (Model, tea.Cmd) {
	if m.currentIndex < len(m.questions)-1 {
		m.currentIndex++
		m.cursor = 0
		m.textInput = ""
		return m, nil
	}
	// All questions answered — submit
	answers := make(map[string]string, len(m.answers))
	for k, v := range m.answers {
		answers[k] = v
	}
	return m, func() tea.Msg { return AnswerSubmittedMsg{Answers: answers} }
}

func (m Model) currentQuestion() *Question {
	if m.currentIndex < 0 || m.currentIndex >= len(m.questions) {
		return nil
	}
	return &m.questions[m.currentIndex]
}

func (m Model) View() string {
	q := m.currentQuestion()
	if q == nil {
		return ""
	}

	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	questionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder

	// Header with question counter
	if len(m.questions) > 1 {
		b.WriteString(titleStyle.Render(fmt.Sprintf("Question %d of %d", m.currentIndex+1, len(m.questions))))
		b.WriteString("\n\n")
	} else {
		b.WriteString(titleStyle.Render("Claude has a question"))
		b.WriteString("\n\n")
	}

	// Question text
	b.WriteString(questionStyle.Render(q.Text))
	b.WriteString("\n\n")

	if len(q.Options) > 0 {
		// Multiple choice
		for i, opt := range q.Options {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(cursor + style.Render(opt.Label))
			if opt.Description != "" && i == m.cursor {
				b.WriteString("\n    " + dimStyle.Render(opt.Description))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter select · Esc cancel"))
	} else {
		// Free text input
		b.WriteString("❯ ")
		if m.textInput == "" {
			b.WriteString(dimStyle.Render("Type your answer…"))
		} else {
			b.WriteString(m.textInput)
		}
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  Enter submit · Esc cancel"))
	}

	return b.String()
}

// CurrentQuestionIndex returns the current question index.
func (m Model) CurrentQuestionIndex() int { return m.currentIndex }

// Answers returns the collected answers so far.
func (m Model) Answers() map[string]string { return m.answers }

// QuestionCount returns the total number of questions.
func (m Model) QuestionCount() int { return len(m.questions) }
