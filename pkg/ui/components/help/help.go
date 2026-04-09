// Package help provides the interactive help screen (HelpV2).
// Source: components/HelpV2/HelpV2.tsx + General.tsx + Commands.tsx
//
// Two-tab help screen: General (keyboard shortcuts) and Commands (slash commands).
package help

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// HelpDismissedMsg signals the help screen was closed.
type HelpDismissedMsg struct{}

// CommandInfo describes a slash command for the help screen.
type CommandInfo struct {
	Name        string
	Description string
	IsHidden    bool
}

// Tab identifies which help tab is active.
type Tab int

const (
	TabGeneral  Tab = iota
	TabCommands
)

// Model is the interactive help screen.
type Model struct {
	tab      Tab
	commands []CommandInfo
	scroll   int
	width    int
	height   int
}

// New creates a help screen with the given commands.
func New(commands []CommandInfo, width, height int) Model {
	// Filter hidden commands
	var visible []CommandInfo
	for _, c := range commands {
		if !c.IsHidden {
			visible = append(visible, c)
		}
	}
	return Model{commands: visible, width: width, height: height}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEscape, 'q':
			return m, func() tea.Msg { return HelpDismissedMsg{} }
		case tea.KeyTab:
			if m.tab == TabGeneral {
				m.tab = TabCommands
			} else {
				m.tab = TabGeneral
			}
			m.scroll = 0
		case tea.KeyUp, 'k':
			if m.scroll > 0 {
				m.scroll--
			}
		case tea.KeyDown, 'j':
			m.scroll++
		}
	}
	return m, nil
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTab := lipgloss.NewStyle().Bold(true).Underline(true).Padding(0, 2)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render("Help"))
	sb.WriteString("\n\n")

	// Tabs
	generalTab := tabStyle.Render("General")
	commandsTab := tabStyle.Render("Commands")
	if m.tab == TabGeneral {
		generalTab = activeTab.Render("General")
	} else {
		commandsTab = activeTab.Render("Commands")
	}
	sb.WriteString(generalTab + " " + commandsTab)
	sb.WriteString("\n\n")

	// Content
	if m.tab == TabGeneral {
		sb.WriteString(m.viewGeneral())
	} else {
		sb.WriteString(m.viewCommands())
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("Tab to switch, ↑/↓ to scroll, Escape to close"))
	return sb.String()
}

func (m Model) viewGeneral() string {
	shortcuts := []struct {
		key  string
		desc string
	}{
		{"Enter", "Send message"},
		{"Escape", "Cancel current operation / interrupt"},
		{"Ctrl+C (×2)", "Exit Claude Code"},
		{"Ctrl+R", "Search command history"},
		{"Ctrl+O", "Toggle transcript view"},
		{"Ctrl+T", "View tasks"},
		{"Ctrl+B", "Background current task"},
		{"↑/↓", "Navigate command history"},
		{"Tab", "Accept file suggestion"},
		{"@file", "Reference a file in your prompt"},
		{"/command", "Run a slash command"},
	}

	keyStyle := lipgloss.NewStyle().Bold(true).Width(16)
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Keyboard Shortcuts"))
	sb.WriteString("\n\n")

	for _, s := range shortcuts {
		sb.WriteString("  " + keyStyle.Render(s.key) + s.desc + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Tips"))
	sb.WriteString("\n\n")
	sb.WriteString("  • Create a CLAUDE.md file to give Claude project context\n")
	sb.WriteString("  • Use /doctor to diagnose configuration issues\n")
	sb.WriteString("  • Use /compact to reduce context window usage\n")
	sb.WriteString("  • Use /help for this screen\n")

	return sb.String()
}

func (m Model) viewCommands() string {
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	descStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d commands available:\n\n", len(m.commands)))

	maxShow := 15
	start := m.scroll
	if start > len(m.commands) {
		start = len(m.commands)
	}
	end := start + maxShow
	if end > len(m.commands) {
		end = len(m.commands)
	}

	for _, cmd := range m.commands[start:end] {
		sb.WriteString("  " + nameStyle.Render("/"+cmd.Name))
		sb.WriteString("  " + descStyle.Render(cmd.Description) + "\n")
	}

	if end < len(m.commands) {
		sb.WriteString(fmt.Sprintf("\n  ... and %d more (scroll down)\n", len(m.commands)-end))
	}

	return sb.String()
}
