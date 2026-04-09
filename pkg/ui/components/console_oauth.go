package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Source: components/ConsoleOAuthFlow.tsx
//
// Multi-state OAuth login flow. In TS this is a React component with
// spinner, browser open, paste input. In Go this is a bubbletea Model.

// OAuthState tracks the current phase of the OAuth flow.
type OAuthState int

const (
	OAuthIdle           OAuthState = iota // Initial — select login method
	OAuthPlatformSetup                    // Show platform setup info
	OAuthReadyToStart                     // About to open browser
	OAuthWaitingForLogin                  // Browser opened, waiting
	OAuthCreatingAPIKey                   // Got token, creating key
	OAuthSuccess                          // Login complete
	OAuthError                            // Something failed
)

// OAuthFlowDoneMsg signals the OAuth flow is complete.
type OAuthFlowDoneMsg struct {
	Success bool
	Token   string
	Error   string
}

// OAuthFlowModel manages the console OAuth login flow.
type OAuthFlowModel struct {
	state        OAuthState
	mode         string // "login" or "setup-token"
	message      string // status or error message
	loginURL     string
	pasteInput   TextInputModel
	startMessage string
}

// NewOAuthFlowModel creates a new OAuth flow.
func NewOAuthFlowModel(mode, startMessage string) OAuthFlowModel {
	if mode == "" {
		mode = "login"
	}
	ti := NewTextInput("Paste code here if prompted > ")
	return OAuthFlowModel{
		state:        OAuthIdle,
		mode:         mode,
		pasteInput:   ti,
		startMessage: startMessage,
	}
}

func (m OAuthFlowModel) Init() tea.Cmd { return nil }

func (m OAuthFlowModel) Update(msg tea.Msg) (OAuthFlowModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEscape:
			return m, func() tea.Msg {
				return OAuthFlowDoneMsg{Success: false, Error: "canceled"}
			}
		case tea.KeyEnter:
			return m.handleEnter()
		}

		// Forward to paste input when waiting
		if m.state == OAuthWaitingForLogin {
			var cmd tea.Cmd
			m.pasteInput, cmd = m.pasteInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m OAuthFlowModel) handleEnter() (OAuthFlowModel, tea.Cmd) {
	switch m.state {
	case OAuthIdle:
		m.state = OAuthReadyToStart
		m.message = "Opening browser for login..."
		return m, nil
	case OAuthReadyToStart:
		m.state = OAuthWaitingForLogin
		m.loginURL = "https://console.anthropic.com/login"
		m.message = "Waiting for login in browser..."
		return m, nil
	case OAuthWaitingForLogin:
		// Check if user pasted a code
		code := strings.TrimSpace(m.pasteInput.Value())
		if code != "" {
			m.state = OAuthCreatingAPIKey
			m.message = "Creating API key..."
			return m, nil
		}
	case OAuthCreatingAPIKey:
		m.state = OAuthSuccess
		return m, func() tea.Msg {
			return OAuthFlowDoneMsg{Success: true}
		}
	}
	return m, nil
}

func (m OAuthFlowModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	var sb strings.Builder

	if m.startMessage != "" {
		sb.WriteString(dimStyle.Render(m.startMessage))
		sb.WriteString("\n\n")
	}

	switch m.state {
	case OAuthIdle:
		sb.WriteString(titleStyle.Render("Login to Claude"))
		sb.WriteString("\n\n")
		sb.WriteString("Press Enter to open the browser for authentication.\n")
		sb.WriteString(dimStyle.Render("Press Escape to cancel"))

	case OAuthReadyToStart:
		sb.WriteString("⏺ Opening browser for login...\n")
		sb.WriteString(dimStyle.Render("Press Enter to continue"))

	case OAuthWaitingForLogin:
		sb.WriteString("⏺ Waiting for login...\n\n")
		if m.loginURL != "" {
			sb.WriteString(fmt.Sprintf("  If the browser didn't open, visit:\n  %s\n\n", m.loginURL))
		}
		sb.WriteString(m.pasteInput.View())

	case OAuthCreatingAPIKey:
		sb.WriteString("⏺ Creating API key...")

	case OAuthSuccess:
		sb.WriteString(successStyle.Render("✓ Login successful"))

	case OAuthError:
		sb.WriteString(errStyle.Render("✗ " + m.message))
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render("Press Enter to retry, Escape to cancel"))
	}

	return sb.String()
}

// State returns the current OAuth state.
func (m OAuthFlowModel) State() OAuthState { return m.state }

// IsDone returns true when the flow has completed (success or error+cancel).
func (m OAuthFlowModel) IsDone() bool {
	return m.state == OAuthSuccess
}
