// Package thinkback provides the /thinkback (think-back) command.
//
// Source: commands/thinkback/thinkback.tsx
//
// Year-in-review animation feature. Checks if the thinkback plugin is
// installed, installs it from the marketplace if needed, then shows a
// menu: play animation, edit, fix errors, or regenerate.
package thinkback

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// Step identifies the current wizard step.
type Step string

const (
	StepChecking     Step = "checking"
	StepInstalling   Step = "installing"
	StepMenu         Step = "menu"
	StepPlaying      Step = "playing"
	StepError        Step = "error"
)

// MenuAction is what the user wants to do.
type MenuAction string

const (
	ActionPlay       MenuAction = "play"
	ActionEdit       MenuAction = "edit"
	ActionFix        MenuAction = "fix"
	ActionRegenerate MenuAction = "regenerate"
)

// Prompt templates for skill invocation.
const (
	EditPrompt       = `Use the Skill tool to invoke the "thinkback" skill with mode=edit to modify my existing Claude Code year in review animation. Ask me what I want to change.`
	FixPrompt        = `Use the Skill tool to invoke the "thinkback" skill with mode=fix to fix validation or rendering errors in my existing Claude Code year in review animation.`
	RegeneratePrompt = `Use the Skill tool to invoke the "thinkback" skill with mode=regenerate to create a completely new Claude Code year in review animation from scratch.`
)

// DoneMsg is sent when the command is finished.
type DoneMsg struct {
	Result string
}

// SkillInvokeMsg requests invoking the thinkback skill.
type SkillInvokeMsg struct {
	Prompt string
}

// PlayAnimationMsg requests playing the animation.
type PlayAnimationMsg struct{}

// PluginCheckMsg carries the plugin check result.
type PluginCheckMsg struct {
	Installed    bool
	HasAnimation bool
	Error        string
}

// Model is the thinkback command bubbletea model.
type Model struct {
	step         Step
	cursor       int
	hasAnimation bool
	message      string
}

// New creates the thinkback wizard.
func New() Model {
	return Model{step: StepChecking}
}

func (m Model) Init() tea.Cmd {
	// In production, this would check plugin installation
	return func() tea.Msg {
		return PluginCheckMsg{Installed: true, HasAnimation: false}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case PluginCheckMsg:
		if msg.Error != "" {
			m.step = StepError
			m.message = msg.Error
			return m, nil
		}
		if !msg.Installed {
			m.step = StepInstalling
			return m, nil
		}
		m.step = StepMenu
		m.hasAnimation = msg.HasAnimation
		return m, nil

	case tea.KeyPressMsg:
		switch m.step {
		case StepMenu:
			return m.updateMenu(msg)
		case StepError:
			return m, func() tea.Msg { return DoneMsg{} }
		}
	}
	return m, nil
}

func (m Model) updateMenu(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	options := m.menuOptions()

	switch msg.Code {
	case tea.KeyUp, 'k':
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown, 'j':
		if m.cursor < len(options)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		if m.cursor < len(options) {
			action := options[m.cursor].Action
			switch action {
			case ActionPlay:
				return m, func() tea.Msg { return PlayAnimationMsg{} }
			case ActionEdit:
				return m, func() tea.Msg { return SkillInvokeMsg{Prompt: EditPrompt} }
			case ActionFix:
				return m, func() tea.Msg { return SkillInvokeMsg{Prompt: FixPrompt} }
			case ActionRegenerate:
				return m, func() tea.Msg { return SkillInvokeMsg{Prompt: RegeneratePrompt} }
			}
		}
	case tea.KeyEscape, 'q':
		return m, func() tea.Msg { return DoneMsg{} }
	}
	return m, nil
}

type menuOption struct {
	Label       string
	Action      MenuAction
	Description string
}

func (m Model) menuOptions() []menuOption {
	if m.hasAnimation {
		return []menuOption{
			{Label: "Play animation", Action: ActionPlay, Description: "Watch your year in review"},
			{Label: "Edit content", Action: ActionEdit, Description: "Modify the animation"},
			{Label: "Fix errors", Action: ActionFix, Description: "Fix validation or rendering issues"},
			{Label: "Regenerate", Action: ActionRegenerate, Description: "Create a new animation from scratch"},
		}
	}
	return []menuOption{
		{Label: "Let's go!", Action: ActionRegenerate, Description: "Generate your personalized animation"},
	}
}

func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	subtitleStyle := lipgloss.NewStyle().Faint(true)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))

	var b strings.Builder

	b.WriteString(titleStyle.Render("✻ Think Back on 2025 with Claude Code"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("  Generate your 2025 Claude Code Think Back"))
	b.WriteString("\n\n")

	switch m.step {
	case StepChecking:
		b.WriteString("  ◐ Checking thinkback installation…\n")

	case StepInstalling:
		b.WriteString("  ◐ Installing thinkback plugin…\n")

	case StepMenu:
		if !m.hasAnimation {
			b.WriteString("  Relive your year of coding with Claude.\n")
			b.WriteString(dimStyle.Render("  We'll create a personalized ASCII animation celebrating your journey."))
			b.WriteString("\n\n")
		}

		options := m.menuOptions()
		for i, opt := range options {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt.Label)))
			if i == m.cursor {
				b.WriteString("    " + dimStyle.Render(opt.Description) + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Enter select · Esc cancel"))

	case StepPlaying:
		b.WriteString("  ▶ Playing animation…\n")

	case StepError:
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
