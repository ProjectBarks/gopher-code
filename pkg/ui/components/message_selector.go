package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Source: components/MessageSelector.tsx
//
// Two-phase rewind picker: first select a message, then choose what to restore.
// In TS this is a React component with CustomSelect. In Go, a bubbletea Model.

const maxVisibleMessages = 7

// RestoreAction is what the user wants to restore when rewinding.
type RestoreAction string

const (
	RestoreBoth         RestoreAction = "both"
	RestoreConversation RestoreAction = "conversation"
	RestoreCode         RestoreAction = "code"
	RestoreSummarize    RestoreAction = "summarize"
	RestoreCancel       RestoreAction = "nevermind"
)

// MessageEntry is a selectable entry in the message list.
type MessageEntry struct {
	ID       string
	Preview  string // first ~60 chars of the message
	TurnNum  int
	IsUser   bool
}

// MessageSelectorDoneMsg signals the user completed the rewind selection.
type MessageSelectorDoneMsg struct {
	MessageID string
	Action    RestoreAction
}

// MessageSelectorCancelMsg signals the user closed the selector.
type MessageSelectorCancelMsg struct{}

// MessageSelectorModel is the two-phase rewind picker.
type MessageSelectorModel struct {
	messages []MessageEntry
	selected int
	scroll   int // top of visible window

	// Phase 2: choose restore action
	phase      int // 0 = pick message, 1 = pick action
	chosenMsg  string
	actionIdx  int
}

// NewMessageSelector creates a rewind picker from user messages.
func NewMessageSelector(entries []MessageEntry) MessageSelectorModel {
	return MessageSelectorModel{messages: entries}
}

func (m MessageSelectorModel) Init() tea.Cmd { return nil }

func (m MessageSelectorModel) Update(msg tea.Msg) (MessageSelectorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEscape:
			if m.phase == 1 {
				m.phase = 0 // back to message list
				return m, nil
			}
			return m, func() tea.Msg { return MessageSelectorCancelMsg{} }

		case tea.KeyUp, 'k':
			if m.phase == 0 {
				if m.selected > 0 {
					m.selected--
					if m.selected < m.scroll {
						m.scroll = m.selected
					}
				}
			} else {
				if m.actionIdx > 0 {
					m.actionIdx--
				}
			}

		case tea.KeyDown, 'j':
			if m.phase == 0 {
				if m.selected < len(m.messages)-1 {
					m.selected++
					if m.selected >= m.scroll+maxVisibleMessages {
						m.scroll = m.selected - maxVisibleMessages + 1
					}
				}
			} else {
				if m.actionIdx < 3 { // 4 actions (0-3)
					m.actionIdx++
				}
			}

		case tea.KeyEnter:
			if m.phase == 0 && len(m.messages) > 0 {
				m.chosenMsg = m.messages[m.selected].ID
				m.phase = 1
				m.actionIdx = 0
				return m, nil
			}
			if m.phase == 1 {
				actions := []RestoreAction{RestoreBoth, RestoreConversation, RestoreCode, RestoreCancel}
				action := actions[m.actionIdx]
				if action == RestoreCancel {
					return m, func() tea.Msg { return MessageSelectorCancelMsg{} }
				}
				return m, func() tea.Msg {
					return MessageSelectorDoneMsg{MessageID: m.chosenMsg, Action: action}
				}
			}
		}
	}
	return m, nil
}

func (m MessageSelectorModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder

	if m.phase == 0 {
		sb.WriteString(titleStyle.Render("Select a message to rewind to"))
		sb.WriteString("\n\n")

		if len(m.messages) == 0 {
			sb.WriteString(dimStyle.Render("No messages to rewind to"))
			return sb.String()
		}

		end := m.scroll + maxVisibleMessages
		if end > len(m.messages) {
			end = len(m.messages)
		}
		for i := m.scroll; i < end; i++ {
			entry := m.messages[i]
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.selected {
				cursor = "> "
				style = selStyle
			}
			label := fmt.Sprintf("Turn %d: %s", entry.TurnNum, entry.Preview)
			sb.WriteString(cursor + style.Render(label) + "\n")
		}

		if m.scroll > 0 {
			sb.WriteString(dimStyle.Render("  ↑ more") + "\n")
		}
		if end < len(m.messages) {
			sb.WriteString(dimStyle.Render("  ↓ more") + "\n")
		}

		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render("↑/↓ to navigate, Enter to select, Escape to cancel"))

	} else {
		sb.WriteString(titleStyle.Render("What would you like to restore?"))
		sb.WriteString("\n\n")

		actions := []struct {
			label string
			desc  string
		}{
			{"Restore both code and conversation", "Rewind files and messages"},
			{"Restore conversation only", "Rewind messages, keep current files"},
			{"Restore code only", "Rewind files, keep current conversation"},
			{"Never mind", "Cancel and go back"},
		}

		for i, a := range actions {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.actionIdx {
				cursor = "> "
				style = selStyle
			}
			sb.WriteString(cursor + style.Render(a.label) + "\n")
			sb.WriteString("    " + dimStyle.Render(a.desc) + "\n")
		}
	}

	return sb.String()
}
