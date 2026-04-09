// Package ultraplan provides the /ultraplan remote planning command.
//
// Source: commands/ultraplan.tsx
//
// Launches a remote Claude Code session for multi-agent planning using
// the most powerful model (Opus). The remote session plans while the
// local terminal stays free. When the plan is ready, the user can
// execute it remotely or teleport it back.
package ultraplan

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// CCRTermsURL is the terms of service URL.
const CCRTermsURL = "https://code.claude.com/docs/en/claude-code-on-the-web"

// TimeoutDuration is the max planning duration (30 minutes).
const TimeoutDuration = 30 * time.Minute

// Step identifies the current command state.
type Step string

const (
	StepUsage         Step = "usage"       // bare /ultraplan with no args
	StepChecking      Step = "checking"    // checking eligibility
	StepLaunching     Step = "launching"   // creating remote session
	StepPolling       Step = "polling"     // waiting for plan approval
	StepNeedsInput    Step = "needs_input" // remote needs user input
	StepApproved      Step = "approved"    // plan approved
	StepChoosing      Step = "choosing"    // user choosing execute location
	StepAlreadyActive Step = "already"     // session already running
	StepError         Step = "error"
)

// DoneMsg is sent when the command is finished.
type DoneMsg struct {
	Result string
}

// LaunchResultMsg carries the async launch result.
type LaunchResultMsg struct {
	Success    bool
	SessionURL string
	SessionID  string
	Error      string
}

// PlanReadyMsg indicates the remote plan is ready.
type PlanReadyMsg struct {
	Plan     string
	SessionID string
}

// ExecuteChoice is how the user wants to run the plan.
type ExecuteChoice string

const (
	ChoiceRemote   ExecuteChoice = "remote"   // execute in CCR
	ChoiceTeleport ExecuteChoice = "teleport" // bring plan back locally
)

// ExecuteMsg carries the user's execution choice.
type ExecuteMsg struct {
	Choice    ExecuteChoice
	Plan      string
	SessionID string
}

// Model is the ultraplan command bubbletea model.
type Model struct {
	step       Step
	blurb      string
	sessionURL string
	plan       string
	message    string
	cursor     int
	startTime  time.Time
}

// New creates an ultraplan command from the user's prompt.
func New(blurb string) Model {
	if strings.TrimSpace(blurb) == "" {
		return Model{step: StepUsage}
	}
	return Model{
		step:      StepChecking,
		blurb:     blurb,
		startTime: time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	if m.step == StepUsage || m.step == StepAlreadyActive {
		return nil
	}
	// In production: check eligibility, then launch remote session
	return func() tea.Msg {
		return LaunchResultMsg{
			Success:    true,
			SessionURL: "https://claude.ai/code/session/example",
			SessionID:  "sess-ultra-001",
		}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LaunchResultMsg:
		if !msg.Success {
			m.step = StepError
			m.message = msg.Error
			return m, nil
		}
		m.step = StepPolling
		m.sessionURL = msg.SessionURL
		return m, nil

	case PlanReadyMsg:
		m.step = StepChoosing
		m.plan = msg.Plan
		return m, nil

	case tea.KeyPressMsg:
		switch m.step {
		case StepUsage, StepError, StepAlreadyActive:
			return m, func() tea.Msg { return DoneMsg{} }

		case StepPolling, StepNeedsInput:
			if msg.Code == tea.KeyEscape || msg.Code == 'q' {
				return m, func() tea.Msg {
					return DoneMsg{Result: "Ultraplan stopped"}
				}
			}

		case StepChoosing:
			switch msg.Code {
			case tea.KeyUp, 'k':
				m.cursor = 0
			case tea.KeyDown, 'j':
				m.cursor = 1
			case tea.KeyEnter:
				choices := []ExecuteChoice{ChoiceTeleport, ChoiceRemote}
				return m, func() tea.Msg {
					return ExecuteMsg{
						Choice: choices[m.cursor],
						Plan:   m.plan,
					}
				}
			case tea.KeyEscape:
				return m, func() tea.Msg { return DoneMsg{} }
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))

	var b strings.Builder

	switch m.step {
	case StepUsage:
		b.WriteString(titleStyle.Render("◆ Ultraplan"))
		b.WriteString("\n\n")
		b.WriteString("  Usage: /ultraplan <prompt>\n\n")
		b.WriteString("  Advanced multi-agent plan mode with our most powerful model\n")
		b.WriteString("  (Opus). Runs in Claude Code on the web. When the plan is\n")
		b.WriteString("  ready, you can execute it in the web session or send it\n")
		b.WriteString("  back here. Terminal stays free while the remote plans.\n")
		b.WriteString("  Requires /login.\n\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Terms: %s", CCRTermsURL)))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  Press any key to close"))

	case StepChecking:
		b.WriteString(titleStyle.Render("◆ Ultraplan"))
		b.WriteString("\n\n")
		b.WriteString("  ◐ Checking eligibility…\n")

	case StepLaunching:
		b.WriteString(titleStyle.Render("◆ Ultraplan"))
		b.WriteString("\n\n")
		b.WriteString("  ◐ Launching remote session…\n")
		b.WriteString(dimStyle.Render("  This may take a moment"))

	case StepPolling:
		elapsed := time.Since(m.startTime)
		b.WriteString(titleStyle.Render("◆ Ultraplan"))
		b.WriteString("\n\n")
		b.WriteString(successStyle.Render("  ✓ Remote session launched"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Session: %s\n", m.sessionURL))
		b.WriteString(fmt.Sprintf("  Elapsed: %s\n", formatDuration(elapsed)))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Planning with Opus in the cloud…"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Your terminal is free — keep working."))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  Esc/q to stop"))

	case StepNeedsInput:
		b.WriteString(titleStyle.Render("◆ Ultraplan"))
		b.WriteString("\n\n")
		b.WriteString(warnStyle.Render("  ⚠ The remote session needs your input"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Visit: %s\n", m.sessionURL))

	case StepChoosing:
		b.WriteString(titleStyle.Render("◆ Ultraplan — Plan Ready!"))
		b.WriteString("\n\n")
		b.WriteString(successStyle.Render("  ✓ Plan approved"))
		b.WriteString("\n\n")
		b.WriteString("  How do you want to execute the plan?\n\n")

		options := []struct {
			label string
			desc  string
		}{
			{"Execute here (teleport)", "Bring the plan back and run it locally"},
			{"Execute in the cloud", "Run it in the web session"},
		}
		for i, opt := range options {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt.label)))
			if i == m.cursor {
				b.WriteString("    " + dimStyle.Render(opt.desc) + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Enter select · Esc cancel"))

	case StepAlreadyActive:
		b.WriteString(titleStyle.Render("◆ Ultraplan"))
		b.WriteString("\n\n")
		b.WriteString(warnStyle.Render("  ⚠ An ultraplan session is already active"))
		b.WriteString("\n\n")
		if m.sessionURL != "" {
			b.WriteString(fmt.Sprintf("  Session: %s\n", m.sessionURL))
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press any key to close"))

	case StepError:
		b.WriteString(titleStyle.Render("◆ Ultraplan"))
		b.WriteString("\n\n")
		b.WriteString(errStyle.Render("  ✗ Error"))
		b.WriteString("\n\n")
		if m.message != "" {
			b.WriteString("  " + m.message + "\n")
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Press any key to close"))
	}

	return b.String()
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

// BuildPrompt assembles the remote session's initial user message.
func BuildPrompt(blurb string, seedPlan string) string {
	var parts []string
	if seedPlan != "" {
		parts = append(parts, "Here is a draft plan to refine:", "", seedPlan, "")
	}
	parts = append(parts, blurb)
	return strings.Join(parts, "\n")
}
