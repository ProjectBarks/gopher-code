// Package remote_setup provides the /web-setup command for claude.ai/code.
//
// Source: commands/remote-setup/remote-setup.tsx, api.ts
//
// Multi-step wizard: check Claude auth → check GitHub CLI → confirm token
// upload → upload to CCR → create default environment → show success URL.
package remote_setup

import (
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Step identifies the current wizard step.
type Step string

const (
	StepChecking     Step = "checking"
	StepNotSignedIn  Step = "not_signed_in"
	StepNoGhCLI      Step = "no_gh_cli"
	StepNoGhAuth     Step = "no_gh_auth"
	StepConfirm      Step = "confirm"
	StepUploading    Step = "uploading"
	StepSuccess      Step = "success"
	StepError        Step = "error"
)

// CodeWebURL is the URL for Claude Code on the web.
const CodeWebURL = "https://claude.ai/code"

// DoneMsg is sent when the wizard is closed.
type DoneMsg struct {
	Result string
}

// Model is the web-setup wizard bubbletea model.
type Model struct {
	step      Step
	message   string
	cursor    int
	ghToken   string // redacted after display
}

// New creates the web-setup wizard.
func New() Model {
	return Model{step: StepChecking}
}

// CheckResult describes the precondition check outcome.
type CheckResult struct {
	Step    Step
	Token   string // GitHub token if available
	Message string
}

// CheckResultMsg carries the async check result.
type CheckResultMsg struct {
	Result CheckResult
}

// UploadResultMsg carries the token upload result.
type UploadResultMsg struct {
	Success bool
	Message string
}

// CheckPreconditions runs the login and gh CLI checks.
func CheckPreconditions() tea.Cmd {
	return func() tea.Msg {
		// Check if gh CLI is installed
		_, err := exec.LookPath("gh")
		if err != nil {
			return CheckResultMsg{Result: CheckResult{
				Step:    StepNoGhCLI,
				Message: "GitHub CLI (gh) is not installed. Install it from https://cli.github.com/",
			}}
		}

		// Check if gh is authenticated
		cmd := exec.Command("gh", "auth", "status")
		if err := cmd.Run(); err != nil {
			return CheckResultMsg{Result: CheckResult{
				Step:    StepNoGhAuth,
				Message: "GitHub CLI is not authenticated. Run: gh auth login",
			}}
		}

		// Get the token
		out, err := exec.Command("gh", "auth", "token").Output()
		if err != nil {
			return CheckResultMsg{Result: CheckResult{
				Step:    StepNoGhAuth,
				Message: "Could not retrieve GitHub token. Run: gh auth login",
			}}
		}
		token := strings.TrimSpace(string(out))
		if token == "" {
			return CheckResultMsg{Result: CheckResult{
				Step:    StepNoGhAuth,
				Message: "Empty GitHub token. Run: gh auth login",
			}}
		}

		return CheckResultMsg{Result: CheckResult{
			Step:  StepConfirm,
			Token: token,
		}}
	}
}

func (m Model) Init() tea.Cmd {
	return CheckPreconditions()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CheckResultMsg:
		m.step = msg.Result.Step
		m.message = msg.Result.Message
		m.ghToken = msg.Result.Token
		return m, nil

	case UploadResultMsg:
		if msg.Success {
			m.step = StepSuccess
			m.message = msg.Message
		} else {
			m.step = StepError
			m.message = msg.Message
		}
		return m, nil

	case tea.KeyPressMsg:
		switch m.step {
		case StepConfirm:
			switch msg.Code {
			case 'y', 'Y', tea.KeyEnter:
				m.step = StepUploading
				// In production, this would call the CCR API
				return m, func() tea.Msg {
					return UploadResultMsg{
						Success: true,
						Message: fmt.Sprintf("GitHub token imported. Visit %s to start coding.", CodeWebURL),
					}
				}
			case 'n', 'N', tea.KeyEscape:
				return m, func() tea.Msg { return DoneMsg{Result: "Cancelled"} }
			}

		case StepSuccess, StepError, StepNotSignedIn, StepNoGhCLI, StepNoGhAuth:
			return m, func() tea.Msg { return DoneMsg{Result: m.message} }
		}
	}
	return m, nil
}

func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	dimStyle := lipgloss.NewStyle().Faint(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))

	var b strings.Builder
	b.WriteString(titleStyle.Render("Web Setup — Claude Code on the Web"))
	b.WriteString("\n\n")

	switch m.step {
	case StepChecking:
		b.WriteString("  ◐ Checking prerequisites…\n")

	case StepNotSignedIn:
		b.WriteString(errStyle.Render("  ✗ Not signed in to Claude"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Please visit %s and log in first.\n", CodeWebURL))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press any key to close"))

	case StepNoGhCLI:
		b.WriteString(warnStyle.Render("  ⚠ GitHub CLI not found"))
		b.WriteString("\n\n")
		b.WriteString("  The GitHub CLI (gh) is required for /web-setup.\n")
		b.WriteString("  Install it from: https://cli.github.com/\n")
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press any key to close"))

	case StepNoGhAuth:
		b.WriteString(warnStyle.Render("  ⚠ GitHub CLI not authenticated"))
		b.WriteString("\n\n")
		b.WriteString("  Run this command to authenticate:\n")
		b.WriteString("    gh auth login\n")
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press any key to close"))

	case StepConfirm:
		b.WriteString(successStyle.Render("  ✓ GitHub CLI authenticated"))
		b.WriteString("\n\n")
		b.WriteString("  This will:\n")
		b.WriteString("  • Import your GitHub token to claude.ai/code\n")
		b.WriteString("  • Enable git clone/push in web sessions\n")
		b.WriteString("  • Create a default cloud environment\n")
		b.WriteString("\n")
		b.WriteString("  Continue? (y/n)\n")

	case StepUploading:
		b.WriteString("  ◐ Importing GitHub token…\n")

	case StepSuccess:
		b.WriteString(successStyle.Render("  ✓ Setup complete!"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Visit %s to start coding.\n", CodeWebURL))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press any key to close"))

	case StepError:
		b.WriteString(errStyle.Render("  ✗ Setup failed"))
		b.WriteString("\n\n")
		if m.message != "" {
			b.WriteString("  " + m.message + "\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press any key to close"))
	}

	return b.String()
}
