package permissions

// Source: components/permissions/BashPermissionRequest/BashPermissionRequest.tsx,
//         bashToolUseOptions.tsx
//
// Specialized permission prompt for Bash/PowerShell commands. Shows the
// command with syntax highlighting hints, destructive command warnings,
// sandbox status, and configurable allow options.

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// BashPermissionModel is the permission prompt for bash/powershell commands.
type BashPermissionModel struct {
	ToolUseID    string
	Command      string
	Description  string
	IsDangerous  bool
	IsSandboxed  bool
	IsShell      string // "bash" or "powershell"
	selected     int
	options      []bashOption
}

type bashOption struct {
	Label    string
	Decision Decision
	Key      string
}

// NewBashPermission creates a bash permission prompt.
func NewBashPermission(toolUseID, command, description string) BashPermissionModel {
	dangerous := isDangerousCommand(command)

	opts := []bashOption{
		{Label: "Allow once", Decision: DecisionAllow, Key: "y"},
		{Label: "Deny", Decision: DecisionDeny, Key: "n"},
		{Label: "Always allow for this session", Decision: DecisionAlwaysAllow, Key: "a"},
	}

	return BashPermissionModel{
		ToolUseID:   toolUseID,
		Command:     command,
		Description: description,
		IsDangerous: dangerous,
		IsShell:     "bash",
		options:     opts,
	}
}

// SetSandboxed marks whether the command will run in a sandbox.
func (m *BashPermissionModel) SetSandboxed(sandboxed bool) { m.IsSandboxed = sandboxed }

func (m BashPermissionModel) Init() tea.Cmd { return nil }

func (m BashPermissionModel) Update(msg tea.Msg) (BashPermissionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyDown, 'j':
			if m.selected < len(m.options)-1 {
				m.selected++
			}
		case 'y':
			return m, m.decide(DecisionAllow)
		case 'n':
			return m, m.decide(DecisionDeny)
		case 'a':
			return m, m.decide(DecisionAlwaysAllow)
		case tea.KeyEnter:
			return m, m.decide(m.options[m.selected].Decision)
		case tea.KeyEscape:
			return m, m.decide(DecisionDeny)
		}
	}
	return m, nil
}

func (m BashPermissionModel) decide(d Decision) tea.Cmd {
	return func() tea.Msg {
		return PermissionDecisionMsg{
			ToolUseID: m.ToolUseID,
			Decision:  d,
			Request: Request{
				Type:        RequestBash,
				ToolName:    "Bash",
				Command:     m.Command,
				Description: m.Description,
				IsDangerous: m.IsDangerous,
			},
		}
	}
}

func (m BashPermissionModel) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true)
	cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))
	dangerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Error))
	dimStyle := lipgloss.NewStyle().Faint(true)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	sandboxStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))

	var b strings.Builder

	// Header
	headerStyle := warnStyle
	icon := "⚠"
	if m.IsDangerous {
		headerStyle = dangerStyle
		icon = "⚠"
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s Claude wants to run a command", icon)))
	b.WriteString("\n\n")

	// Command display
	b.WriteString(titleStyle.Render("  Command:"))
	b.WriteString("\n")
	b.WriteString("  " + cmdStyle.Render(m.Command))
	b.WriteString("\n")

	// Description
	if m.Description != "" {
		b.WriteString("\n")
		b.WriteString("  " + dimStyle.Render(m.Description))
		b.WriteString("\n")
	}

	// Danger warning
	if m.IsDangerous {
		b.WriteString("\n")
		warning := getDestructiveWarning(m.Command)
		b.WriteString("  " + dangerStyle.Render("⚠ "+warning))
		b.WriteString("\n")
	}

	// Sandbox status
	if m.IsSandboxed {
		b.WriteString("\n")
		b.WriteString("  " + sandboxStyle.Render("🔒 Sandboxed"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Options
	for i, opt := range m.options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.selected {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(fmt.Sprintf("%s%s  %s\n",
			cursor,
			style.Render(opt.Label),
			dimStyle.Render("("+opt.Key+")")))
	}

	return b.String()
}

// getDestructiveWarning returns a human-readable warning for dangerous commands.
func getDestructiveWarning(cmd string) string {
	lower := strings.ToLower(cmd)
	switch {
	case strings.Contains(lower, "rm -rf"):
		return "This command will recursively delete files. Verify the path carefully."
	case strings.Contains(lower, "dd if="):
		return "This command writes directly to disk. Data loss is possible."
	case strings.Contains(lower, "chmod 777") || strings.Contains(lower, "chmod -r 777"):
		return "This sets world-readable/writable permissions on files."
	case strings.Contains(lower, "mkfs"):
		return "This command formats a filesystem. All data will be erased."
	case strings.Contains(lower, "> /dev/"):
		return "This writes directly to a device. Data loss is possible."
	case strings.Contains(lower, "curl") && strings.Contains(lower, "| sh"),
		strings.Contains(lower, "curl") && strings.Contains(lower, "| bash"),
		strings.Contains(lower, "wget") && strings.Contains(lower, "| sh"),
		strings.Contains(lower, "wget") && strings.Contains(lower, "| bash"):
		return "This pipes remote content directly to a shell. The script could do anything."
	default:
		return "This command may make significant changes to your system."
	}
}
