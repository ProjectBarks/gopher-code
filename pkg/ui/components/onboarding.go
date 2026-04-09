package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Source: components/Onboarding.tsx
//
// Multi-step onboarding wizard shown on first run.
// Steps: welcome → theme → security notes → done.
// In TS, also handles OAuth/API key — those are separate auth flows in Go.

// OnboardingStep identifies which step is active.
type OnboardingStep int

const (
	StepWelcome  OnboardingStep = iota
	StepTheme
	StepSecurity
	StepDone
)

// OnboardingDoneMsg signals onboarding is complete.
type OnboardingDoneMsg struct{}

// OnboardingModel manages the multi-step onboarding flow.
type OnboardingModel struct {
	step   OnboardingStep
	width  int
	height int
}

// NewOnboardingModel creates a new onboarding flow.
func NewOnboardingModel() OnboardingModel {
	return OnboardingModel{step: StepWelcome, width: 80}
}

// Init initializes the onboarding model.
func (m OnboardingModel) Init() tea.Cmd { return nil }

// Update handles key events for step progression.
func (m OnboardingModel) Update(msg tea.Msg) (OnboardingModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEnter:
			return m.nextStep()
		case tea.KeyEscape:
			// Skip to done
			m.step = StepDone
			return m, func() tea.Msg { return OnboardingDoneMsg{} }
		case '1', '2', '3':
			if m.step == StepTheme {
				// Theme selection: 1=dark, 2=light, 3=high-contrast
				m.step = StepSecurity
				return m, nil
			}
		}
	}
	return m, nil
}

func (m OnboardingModel) nextStep() (OnboardingModel, tea.Cmd) {
	switch m.step {
	case StepWelcome:
		m.step = StepTheme
	case StepTheme:
		m.step = StepSecurity
	case StepSecurity:
		m.step = StepDone
		return m, func() tea.Msg { return OnboardingDoneMsg{} }
	}
	return m, nil
}

// View renders the current onboarding step.
func (m OnboardingModel) View() string {
	switch m.step {
	case StepWelcome:
		return m.viewWelcome()
	case StepTheme:
		return m.viewTheme()
	case StepSecurity:
		return m.viewSecurity()
	default:
		return ""
	}
}

// Step returns the current step.
func (m OnboardingModel) Step() OnboardingStep { return m.step }

// IsDone returns true when onboarding is complete.
func (m OnboardingModel) IsDone() bool { return m.step == StepDone }

func (m OnboardingModel) viewWelcome() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Welcome to Claude Code"))
	sb.WriteString("\n\n")
	sb.WriteString("Claude Code is an AI-powered coding assistant that runs\n")
	sb.WriteString("in your terminal. It can read files, run commands, edit\n")
	sb.WriteString("code, and help you with software engineering tasks.\n")
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("Press Enter to continue, Escape to skip"))
	return sb.String()
}

func (m OnboardingModel) viewTheme() string {
	titleStyle := lipgloss.NewStyle().Bold(true)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Choose a theme"))
	sb.WriteString("\n\n")
	sb.WriteString("  1. Dark (default)\n")
	sb.WriteString("  2. Light\n")
	sb.WriteString("  3. High Contrast\n")
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("To change later, run /theme"))
	return sb.String()
}

func (m OnboardingModel) viewSecurity() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Security notes"))
	sb.WriteString("\n\n")
	sb.WriteString("  1. Claude can make mistakes — always review responses,\n")
	sb.WriteString("     especially when running code.\n\n")
	sb.WriteString("  2. Claude can execute commands on your machine.\n")
	sb.WriteString("     You'll be asked to approve potentially dangerous actions.\n\n")
	sb.WriteString("  3. CLAUDE.md files are loaded automatically.\n")
	sb.WriteString("     Be cautious with untrusted repositories.\n")
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("Press Enter to start using Claude Code"))
	return sb.String()
}
