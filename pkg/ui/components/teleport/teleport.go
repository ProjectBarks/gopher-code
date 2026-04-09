// Package teleport provides UI components for remote session setup (teleport).
//
// Source: components/TeleportProgress.tsx, TeleportError.tsx, TeleportStash.tsx,
//         TeleportRepoMismatchDialog.tsx, RemoteEnvironmentDialog.tsx
//
// Teleport is the process of resuming a remote session locally — fetching logs,
// checking out the branch, and restoring the conversation. These components show
// progress, handle errors (auth, git stash), and confirm repo mismatches.
package teleport

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ProgressStep identifies a step in the teleport process.
type ProgressStep string

const (
	StepValidating   ProgressStep = "validating"
	StepFetchingLogs ProgressStep = "fetching_logs"
	StepFetchBranch  ProgressStep = "fetching_branch"
	StepCheckingOut  ProgressStep = "checking_out"
	StepDone         ProgressStep = "done"
)

// stepInfo describes a progress step for display.
type stepInfo struct {
	Key   ProgressStep
	Label string
}

var steps = []stepInfo{
	{StepValidating, "Validating session"},
	{StepFetchingLogs, "Fetching session logs"},
	{StepFetchBranch, "Getting branch info"},
	{StepCheckingOut, "Checking out branch"},
}

var spinnerFrames = []string{"◐", "◓", "◑", "◒"}

// ProgressModel shows teleport progress with step indicators.
type ProgressModel struct {
	CurrentStep ProgressStep
	SessionID   string
	frame       int
}

// NewProgressModel creates a progress display for teleporting.
func NewProgressModel(sessionID string) ProgressModel {
	return ProgressModel{
		CurrentStep: StepValidating,
		SessionID:   sessionID,
	}
}

// Tick advances the spinner animation.
func (m *ProgressModel) Tick() { m.frame++ }

// SetStep updates the current progress step.
func (m *ProgressModel) SetStep(step ProgressStep) { m.CurrentStep = step }

// View renders the progress display.
func (m ProgressModel) View() string {
	colors := theme.Current().Colors()
	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))

	spinner := spinnerFrames[m.frame%len(spinnerFrames)]

	var b strings.Builder
	b.WriteString(accentStyle.Render(spinner+" Teleporting session…"))
	b.WriteString("\n")

	if m.SessionID != "" {
		b.WriteString(dimStyle.Render("  " + m.SessionID))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	currentIdx := -1
	for i, s := range steps {
		if s.Key == m.CurrentStep {
			currentIdx = i
			break
		}
	}

	for i, step := range steps {
		var icon, color string
		if i < currentIdx {
			icon = "✓"
			color = colors.Success
		} else if i == currentIdx {
			icon = spinnerFrames[m.frame%len(spinnerFrames)]
			color = colors.Accent
		} else {
			icon = "○"
			color = colors.TextMuted
		}

		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		if i < currentIdx {
			style = successStyle
		}

		b.WriteString(fmt.Sprintf("  %s %s\n", style.Render(icon), style.Render(step.Label)))
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Error types for teleport precondition failures
// ---------------------------------------------------------------------------

// ErrorType identifies a teleport precondition failure.
type ErrorType string

const (
	ErrorNeedsLogin    ErrorType = "needsLogin"
	ErrorNeedsGitStash ErrorType = "needsGitStash"
)

// ErrorModel handles teleport precondition errors.
type ErrorModel struct {
	ErrorType ErrorType
	Message   string
}

// ErrorResolvedMsg is sent when a teleport error is resolved.
type ErrorResolvedMsg struct{}

// ErrorCancelledMsg is sent when the user cancels error resolution.
type ErrorCancelledMsg struct{}

// View renders the error display.
func (m ErrorModel) View() string {
	colors := theme.Current().Colors()
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder

	switch m.ErrorType {
	case ErrorNeedsLogin:
		b.WriteString(errStyle.Render("⚠ Authentication Required"))
		b.WriteString("\n\n")
		b.WriteString("  You need to log in to claude.ai before teleporting.\n")
		b.WriteString("  Run ")
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("/login"))
		b.WriteString(" to authenticate.\n")

	case ErrorNeedsGitStash:
		b.WriteString(errStyle.Render("⚠ Uncommitted Changes"))
		b.WriteString("\n\n")
		b.WriteString("  Your working directory has uncommitted changes.\n")
		b.WriteString("  Teleport needs a clean git state to check out the remote branch.\n\n")
		b.WriteString("  Options:\n")
		b.WriteString("    • Stash changes (git stash)\n")
		b.WriteString("    • Commit changes\n")
		b.WriteString("    • Cancel teleport\n")

	default:
		b.WriteString(errStyle.Render("⚠ Error"))
		b.WriteString("\n\n")
		if m.Message != "" {
			b.WriteString("  " + m.Message + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Press Esc to cancel"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Repo mismatch dialog
// Source: TeleportRepoMismatchDialog.tsx
// ---------------------------------------------------------------------------

// RepoMismatchModel confirms when the remote session's repo differs from local.
type RepoMismatchModel struct {
	RemoteRepo string
	LocalRepo  string
	cursor     int // 0=continue, 1=cancel
}

// NewRepoMismatchModel creates a mismatch confirmation dialog.
func NewRepoMismatchModel(remoteRepo, localRepo string) RepoMismatchModel {
	return RepoMismatchModel{
		RemoteRepo: remoteRepo,
		LocalRepo:  localRepo,
	}
}

// RepoMismatchContinueMsg is sent when the user chooses to continue despite mismatch.
type RepoMismatchContinueMsg struct{}

// RepoMismatchCancelMsg is sent when the user cancels.
type RepoMismatchCancelMsg struct{}

// Update handles input for the repo mismatch dialog.
func (m RepoMismatchModel) Update(msg tea.Msg) (RepoMismatchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			m.cursor = 0
		case tea.KeyDown, 'j':
			m.cursor = 1
		case tea.KeyEnter:
			if m.cursor == 0 {
				return m, func() tea.Msg { return RepoMismatchContinueMsg{} }
			}
			return m, func() tea.Msg { return RepoMismatchCancelMsg{} }
		case tea.KeyEscape:
			return m, func() tea.Msg { return RepoMismatchCancelMsg{} }
		}
	}
	return m, nil
}

// View renders the repo mismatch dialog.
func (m RepoMismatchModel) View() string {
	colors := theme.Current().Colors()
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	dimStyle := lipgloss.NewStyle().Faint(true)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))

	var b strings.Builder
	b.WriteString(warnStyle.Render("⚠ Repository Mismatch"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Remote: %s\n", m.RemoteRepo))
	b.WriteString(fmt.Sprintf("  Local:  %s\n", m.LocalRepo))
	b.WriteString("\n")
	b.WriteString("  The remote session was started in a different repository.\n")
	b.WriteString("  Continue anyway?\n\n")

	options := []string{"Continue", "Cancel"}
	for i, opt := range options {
		if i == m.cursor {
			b.WriteString("  > " + selectedStyle.Render(opt) + "\n")
		} else {
			b.WriteString("    " + dimStyle.Render(opt) + "\n")
		}
	}

	return b.String()
}
