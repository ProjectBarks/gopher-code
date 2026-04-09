// Package sandbox provides the sandbox settings UI component.
//
// Source: components/sandbox/SandboxSettings.tsx, SandboxConfigTab.tsx,
//         SandboxDependenciesTab.tsx, SandboxOverridesTab.tsx
//
// In TS this is a tabbed dialog with Select for mode picker + config/deps/overrides tabs.
// In Go it's a bubbletea model with mode selection and config display.
package sandbox

import (
	"fmt"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// SandboxMode describes the sandbox enforcement level.
type SandboxMode string

const (
	ModeAutoAllow SandboxMode = "auto-allow" // sandbox + auto-allow bash
	ModeRegular   SandboxMode = "regular"    // sandbox + ask permissions
	ModeDisabled  SandboxMode = "disabled"   // no sandbox
)

// ModeLabel returns the display label for a sandbox mode.
func ModeLabel(mode SandboxMode) string {
	switch mode {
	case ModeAutoAllow:
		return "Sandbox BashTool, with auto-allow"
	case ModeRegular:
		return "Sandbox BashTool, with regular permissions"
	case ModeDisabled:
		return "Disable sandboxing"
	default:
		return string(mode)
	}
}

// Config holds the current sandbox configuration for display.
type Config struct {
	Enabled          bool
	AutoAllow        bool
	ExcludedCommands []string
	ReadDeny         []string
	WriteAllow       []string
	NetworkAllow     []string
	NetworkDeny      []string
	UnixSockets      []string
	Warnings         []string
}

// CurrentMode returns the SandboxMode from the config state.
func (c *Config) CurrentMode() SandboxMode {
	if !c.Enabled {
		return ModeDisabled
	}
	if c.AutoAllow {
		return ModeAutoAllow
	}
	return ModeRegular
}

// ModeChangedMsg is sent when the user selects a new sandbox mode.
type ModeChangedMsg struct {
	Mode SandboxMode
}

// DoneMsg is sent when the user closes the settings.
type DoneMsg struct{}

// Model is the sandbox settings bubbletea model.
type Model struct {
	config   Config
	cursor   int // 0=auto-allow, 1=regular, 2=disabled
	tab      int // 0=mode, 1=config, 2=deps
	width    int
	modes    []SandboxMode
}

// New creates a sandbox settings model from the current config.
func New(cfg Config) Model {
	modes := []SandboxMode{ModeAutoAllow, ModeRegular, ModeDisabled}
	cursor := 0
	current := cfg.CurrentMode()
	for i, m := range modes {
		if m == current {
			cursor = i
			break
		}
	}
	return Model{
		config: cfg,
		cursor: cursor,
		modes:  modes,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.tab == 0 && m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, 'j':
			if m.tab == 0 && m.cursor < len(m.modes)-1 {
				m.cursor++
			}
		case tea.KeyTab:
			m.tab = (m.tab + 1) % 3
		case tea.KeyEnter:
			if m.tab == 0 {
				selected := m.modes[m.cursor]
				return m, func() tea.Msg { return ModeChangedMsg{Mode: selected} }
			}
		case tea.KeyEscape, 'q':
			return m, func() tea.Msg { return DoneMsg{} }
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	currentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))
	tabActiveStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.TabActive))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))

	var b strings.Builder

	// Tab headers
	tabs := []string{"Mode", "Config", "Dependencies"}
	for i, t := range tabs {
		if i > 0 {
			b.WriteString("  ")
		}
		if i == m.tab {
			b.WriteString(tabActiveStyle.Render("[" + t + "]"))
		} else {
			b.WriteString(dimStyle.Render(" " + t + " "))
		}
	}
	b.WriteString("\n\n")

	switch m.tab {
	case 0:
		b.WriteString(m.viewModeTab(titleStyle, selectedStyle, dimStyle, currentStyle))
	case 1:
		b.WriteString(m.viewConfigTab(titleStyle, dimStyle, warnStyle))
	case 2:
		b.WriteString(m.viewDepsTab(titleStyle, dimStyle, warnStyle))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Tab switch · ↑/↓ navigate · Enter select · Esc close"))

	return b.String()
}

func (m Model) viewModeTab(titleStyle, selectedStyle, dimStyle, currentStyle lipgloss.Style) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Sandbox Mode"))
	b.WriteString("\n\n")

	current := m.config.CurrentMode()
	for i, mode := range m.modes {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		label := ModeLabel(mode)
		b.WriteString(cursor)
		b.WriteString(style.Render(label))

		if mode == current {
			b.WriteString(" ")
			b.WriteString(currentStyle.Render("(current)"))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	switch m.modes[m.cursor] {
	case ModeAutoAllow:
		b.WriteString(dimStyle.Render("  Bash commands run in a sandbox. Commands are auto-allowed\n  without permission prompts."))
	case ModeRegular:
		b.WriteString(dimStyle.Render("  Bash commands run in a sandbox. Each command requires\n  permission approval."))
	case ModeDisabled:
		b.WriteString(dimStyle.Render("  No sandboxing. Commands run with full system access."))
	}

	return b.String()
}

func (m Model) viewConfigTab(titleStyle, dimStyle, warnStyle lipgloss.Style) string {
	var b strings.Builder

	if !m.config.Enabled {
		b.WriteString(dimStyle.Render("  Sandbox is not enabled"))
		return b.String()
	}

	// Excluded commands
	b.WriteString(titleStyle.Render("Excluded Commands:"))
	b.WriteString("\n")
	if len(m.config.ExcludedCommands) > 0 {
		b.WriteString(dimStyle.Render("  " + strings.Join(m.config.ExcludedCommands, ", ")))
	} else {
		b.WriteString(dimStyle.Render("  None"))
	}
	b.WriteString("\n")

	// Read restrictions
	if len(m.config.ReadDeny) > 0 {
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("Filesystem Read Restrictions:"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Denied: " + strings.Join(m.config.ReadDeny, ", ")))
		b.WriteString("\n")
	}

	// Write restrictions
	if len(m.config.WriteAllow) > 0 {
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("Filesystem Write Restrictions:"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Allowed: " + strings.Join(m.config.WriteAllow, ", ")))
		b.WriteString("\n")
	}

	// Network restrictions
	if len(m.config.NetworkAllow) > 0 || len(m.config.NetworkDeny) > 0 {
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("Network Restrictions:"))
		b.WriteString("\n")
		if len(m.config.NetworkAllow) > 0 {
			b.WriteString(dimStyle.Render("  Allowed: " + strings.Join(m.config.NetworkAllow, ", ")))
			b.WriteString("\n")
		}
		if len(m.config.NetworkDeny) > 0 {
			b.WriteString(dimStyle.Render("  Denied: " + strings.Join(m.config.NetworkDeny, ", ")))
			b.WriteString("\n")
		}
	}

	// Unix sockets
	if len(m.config.UnixSockets) > 0 {
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("Allowed Unix Sockets:"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  " + strings.Join(m.config.UnixSockets, ", ")))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) viewDepsTab(titleStyle, dimStyle, warnStyle lipgloss.Style) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Sandbox Dependencies"))
	b.WriteString("\n\n")

	platform := runtime.GOOS
	switch platform {
	case "darwin":
		b.WriteString("  Platform: macOS (seatbelt)\n")
		b.WriteString(dimStyle.Render("  Uses sandbox-exec with custom profiles\n"))
	case "linux":
		b.WriteString("  Platform: Linux (bubblewrap)\n")
		b.WriteString(dimStyle.Render("  Requires bwrap (bubblewrap) to be installed\n"))
	default:
		b.WriteString(fmt.Sprintf("  Platform: %s\n", platform))
		b.WriteString(dimStyle.Render("  Sandboxing not supported on this platform\n"))
	}

	if len(m.config.Warnings) > 0 {
		b.WriteString("\n")
		b.WriteString(warnStyle.Render("⚠ Warnings:"))
		b.WriteString("\n")
		for _, w := range m.config.Warnings {
			b.WriteString(warnStyle.Render("  • " + w))
			b.WriteString("\n")
		}
	} else {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  No dependency issues found"))
	}

	return b.String()
}
