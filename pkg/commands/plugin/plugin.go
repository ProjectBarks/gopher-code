// Package plugin provides the /plugin command for managing plugins.
//
// Source: commands/plugin/plugin.tsx, PluginSettings.tsx
//
// The /plugin command opens a settings UI for browsing, installing,
// enabling, and configuring plugins. In Go, it sends a message to
// open the plugin settings dialog.
package plugin

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ShowPluginSettingsMsg requests opening the plugin settings UI.
type ShowPluginSettingsMsg struct {
	Args string // optional subcommand: "install", "list", "enable", "disable"
}

// PluginInfo describes an installed plugin for display.
type PluginInfo struct {
	Name        string
	Version     string
	Source      string // "marketplace", "local", "git"
	Enabled     bool
	Description string
	Skills      int // number of skills provided
	Agents      int // number of agents provided
}

// DoneMsg is sent when the plugin dialog is closed.
type DoneMsg struct {
	Result string
}

// Model is the plugin management bubbletea model.
type Model struct {
	plugins  []PluginInfo
	cursor   int
	level    int // 0=list, 1=detail
	selected *PluginInfo
}

// New creates a plugin management dialog.
func New(plugins []PluginInfo) Model {
	return Model{plugins: plugins}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.level == 1 {
			switch msg.Code {
			case 'e': // toggle enable/disable
				if m.selected != nil {
					for i, p := range m.plugins {
						if p.Name == m.selected.Name {
							m.plugins[i].Enabled = !m.plugins[i].Enabled
							m.selected = &m.plugins[i]
							break
						}
					}
				}
			case tea.KeyEscape:
				m.level = 0
				m.selected = nil
			}
			return m, nil
		}

		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, 'j':
			if m.cursor < len(m.plugins)-1 {
				m.cursor++
			}
		case tea.KeyEnter:
			if m.cursor < len(m.plugins) {
				p := m.plugins[m.cursor]
				m.selected = &p
				m.level = 1
			}
		case tea.KeyEscape, 'q':
			return m, func() tea.Msg { return DoneMsg{} }
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.level == 1 && m.selected != nil {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m Model) viewList() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	enabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))
	disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextMuted))

	var b strings.Builder
	b.WriteString(titleStyle.Render("Plugins"))
	b.WriteString(dimStyle.Render(fmt.Sprintf(" (%d installed)", len(m.plugins))))
	b.WriteString("\n\n")

	if len(m.plugins) == 0 {
		b.WriteString(dimStyle.Render("  No plugins installed.\n"))
		b.WriteString(dimStyle.Render("  Use /plugin install <name> to add plugins.\n"))
	} else {
		for i, p := range m.plugins {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}

			status := enabledStyle.Render("●")
			if !p.Enabled {
				status = disabledStyle.Render("○")
			}

			b.WriteString(fmt.Sprintf("%s%s %s", cursor, status, style.Render(p.Name)))
			if p.Version != "" {
				b.WriteString(dimStyle.Render(" v" + p.Version))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter details · Esc close"))
	return b.String()
}

func (m Model) viewDetail() string {
	if m.selected == nil {
		return ""
	}
	p := m.selected
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	keyStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render(p.Name))
	b.WriteString("\n\n")

	if p.Version != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Version:"), p.Version))
	}
	if p.Source != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Source:"), p.Source))
	}

	statusStr := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success)).Render("enabled")
	if !p.Enabled {
		statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TextMuted)).Render("disabled")
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("Status:"), statusStr))

	if p.Description != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n", dimStyle.Render(p.Description)))
	}

	if p.Skills > 0 {
		b.WriteString(fmt.Sprintf("  %s %d\n", keyStyle.Render("Skills:"), p.Skills))
	}
	if p.Agents > 0 {
		b.WriteString(fmt.Sprintf("  %s %d\n", keyStyle.Render("Agents:"), p.Agents))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  e toggle enable/disable · Esc back"))
	return b.String()
}
