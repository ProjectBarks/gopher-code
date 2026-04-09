// Package trust_dialog provides the project trust confirmation dialog.
// Source: components/TrustDialog/TrustDialog.tsx
//
// Shown when entering an untrusted project directory. Lists what the
// project configures (hooks, MCP servers, bash permissions) and asks
// the user to trust or reject before proceeding.
package trust_dialog

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TrustDecision is the user's choice in the trust dialog.
type TrustDecision int

const (
	TrustAccepted TrustDecision = iota
	TrustRejected
)

// TrustDecisionMsg signals the user's trust decision.
type TrustDecisionMsg struct {
	Decision TrustDecision
	Project  string
}

// ProjectConfig describes what a project has configured.
type ProjectConfig struct {
	CWD            string
	HasHooks       bool
	HasMCPServers  bool
	MCPServerCount int
	HasBashPerms   bool
	HasAPIKeyHelper bool
}

// Model is the trust dialog bubbletea model.
type Model struct {
	config   ProjectConfig
	selected int // 0=trust, 1=reject
	decided  bool
}

// New creates a trust dialog for the given project config.
func New(config ProjectConfig) Model {
	return Model{config: config}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.decided {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			m.selected = 0
		case tea.KeyDown, 'j':
			m.selected = 1
		case tea.KeyEnter:
			m.decided = true
			decision := TrustAccepted
			if m.selected == 1 {
				decision = TrustRejected
			}
			return m, func() tea.Msg {
				return TrustDecisionMsg{Decision: decision, Project: m.config.CWD}
			}
		case tea.KeyEscape:
			m.decided = true
			return m, func() tea.Msg {
				return TrustDecisionMsg{Decision: TrustRejected, Project: m.config.CWD}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder
	sb.WriteString(warnStyle.Render("⚠ Project Trust Required"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Directory: %s\n\n", titleStyle.Render(m.config.CWD)))

	sb.WriteString("This project has the following configurations:\n\n")

	if m.config.HasHooks {
		sb.WriteString("  • " + warnStyle.Render("Hooks") + " — shell commands that run on tool events\n")
	}
	if m.config.HasMCPServers {
		sb.WriteString(fmt.Sprintf("  • "+warnStyle.Render("MCP servers")+" — %d project-configured server(s)\n", m.config.MCPServerCount))
	}
	if m.config.HasBashPerms {
		sb.WriteString("  • " + warnStyle.Render("Bash permissions") + " — pre-approved shell commands\n")
	}
	if m.config.HasAPIKeyHelper {
		sb.WriteString("  • " + warnStyle.Render("API key helper") + " — custom authentication command\n")
	}

	sb.WriteString("\nDo you trust this project?\n\n")

	options := []string{"Yes, trust this project", "No, exit"}
	for i, opt := range options {
		cursor := "  "
		if i == m.selected {
			cursor = "> "
		}
		sb.WriteString(cursor + opt + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("Learn more: https://code.claude.com/docs/en/security"))
	return sb.String()
}

// HasRisks returns true if the project has any trust-relevant configurations.
func (c ProjectConfig) HasRisks() bool {
	return c.HasHooks || c.HasMCPServers || c.HasBashPerms || c.HasAPIKeyHelper
}
