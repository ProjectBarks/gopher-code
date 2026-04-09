// Package feedback_survey provides the post-turn feedback survey.
// Source: components/FeedbackSurvey/
//
// Shows a brief rating prompt after turns, optionally asks to share transcript.
package feedback_survey

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// State tracks the survey lifecycle.
type State string

const (
	StateClosed           State = "closed"
	StateOpen             State = "open"
	StateThanks           State = "thanks"
	StateTranscriptPrompt State = "transcript_prompt"
	StateSubmitted        State = "submitted"
)

// Response is the user's rating (1-5) or thumbs up/down.
type Response struct {
	Rating  int    // 1-5 scale
	Comment string // optional free-text
}

// FeedbackSubmittedMsg signals the user submitted feedback.
type FeedbackSubmittedMsg struct {
	Response Response
}

// FeedbackDismissedMsg signals the user dismissed the survey.
type FeedbackDismissedMsg struct{}

// Model manages the feedback survey flow.
type Model struct {
	state    State
	selected int // 0-4 for rating 1-5
	comment  string
	message  string // optional prompt message
}

// New creates a new feedback survey.
func New(message string) Model {
	return Model{state: StateOpen, message: message}
}

// State returns the current survey state.
func (m Model) GetState() State { return m.state }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.state == StateClosed || m.state == StateSubmitted {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case msg.Code == tea.KeyEscape:
			m.state = StateClosed
			return m, func() tea.Msg { return FeedbackDismissedMsg{} }

		case msg.Code >= '1' && msg.Code <= '5' && m.state == StateOpen:
			m.selected = int(msg.Code - '1')
			m.state = StateThanks
			return m, nil

		case msg.Code == tea.KeyEnter && m.state == StateThanks:
			m.state = StateSubmitted
			return m, func() tea.Msg {
				return FeedbackSubmittedMsg{Response: Response{Rating: m.selected + 1, Comment: m.comment}}
			}

		case msg.Code == tea.KeyLeft && m.state == StateOpen:
			if m.selected > 0 {
				m.selected--
			}
		case msg.Code == tea.KeyRight && m.state == StateOpen:
			if m.selected < 4 {
				m.selected++
			}
		case msg.Code == tea.KeyEnter && m.state == StateOpen:
			m.state = StateThanks
		}
	}
	return m, nil
}

func (m Model) View() string {
	dimStyle := lipgloss.NewStyle().Faint(true)

	switch m.state {
	case StateClosed:
		return ""
	case StateOpen:
		return m.viewRating()
	case StateThanks:
		return m.viewThanks()
	case StateSubmitted:
		return dimStyle.Render("Thanks for your feedback!")
	default:
		return ""
	}
}

func (m Model) viewRating() string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

	prompt := "How was this response?"
	if m.message != "" {
		prompt = m.message
	}

	stars := ""
	for i := 0; i < 5; i++ {
		if i == m.selected {
			stars += selStyle.Render(fmt.Sprintf(" %d ", i+1))
		} else {
			stars += dimStyle.Render(fmt.Sprintf(" %d ", i+1))
		}
	}

	return dimStyle.Render(prompt) + "\n" + stars + "\n" +
		dimStyle.Render("←/→ or 1-5 to rate, Escape to dismiss")
}

func (m Model) viewThanks() string {
	return lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf("Rating: %d/5 — Press Enter to submit, Escape to cancel", m.selected+1))
}

// IsClosed returns true when the survey is not visible.
func (m Model) IsClosed() bool {
	return m.state == StateClosed || m.state == StateSubmitted
}
