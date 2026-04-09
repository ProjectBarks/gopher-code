// Package theme_picker provides the theme selection UI component.
//
// Source: components/ThemePicker.tsx, components/Passes/Passes.tsx
//
// ThemePicker lets users choose between dark/light/high-contrast themes
// with a live preview. In TS this uses usePreviewTheme for live switching;
// in Go the caller applies the theme from ThemeSelectedMsg.
package theme_picker

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ThemeSelectedMsg is sent when the user picks a theme.
type ThemeSelectedMsg struct {
	Theme theme.ThemeName
}

// CancelledMsg is sent when the user cancels.
type CancelledMsg struct{}

// ThemeOption describes a selectable theme.
type ThemeOption struct {
	Name        theme.ThemeName
	Label       string
	Description string
}

// DefaultOptions returns the standard theme options.
func DefaultOptions() []ThemeOption {
	return []ThemeOption{
		{Name: theme.ThemeDark, Label: "Dark", Description: "Dark background with light text"},
		{Name: theme.ThemeLight, Label: "Light", Description: "Light background with dark text"},
		{Name: theme.ThemeHighContrast, Label: "High Contrast", Description: "Maximum contrast for accessibility"},
	}
}

// Model is the theme picker bubbletea model.
type Model struct {
	options       []ThemeOption
	cursor        int
	current       theme.ThemeName
	showIntroText bool
	helpText      string
}

// New creates a theme picker. current is the currently active theme.
func New(current theme.ThemeName) Model {
	options := DefaultOptions()
	cursor := 0
	for i, o := range options {
		if o.Name == current {
			cursor = i
			break
		}
	}
	return Model{
		options: options,
		cursor:  cursor,
		current: current,
	}
}

// WithIntroText adds intro text above the picker.
func (m Model) WithIntroText(show bool) Model {
	m.showIntroText = show
	return m
}

// WithHelpText adds custom help text.
func (m Model) WithHelpText(text string) Model {
	m.helpText = text
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, 'j':
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case tea.KeyEnter:
			selected := m.options[m.cursor].Name
			return m, func() tea.Msg { return ThemeSelectedMsg{Theme: selected} }
		case tea.KeyEscape, 'q':
			return m, func() tea.Msg { return CancelledMsg{} }
		}
	}
	return m, nil
}

func (m Model) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)
	currentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Success))

	var b strings.Builder

	if m.showIntroText {
		b.WriteString(titleStyle.Render("Choose your theme"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  Select a color theme for Claude Code"))
		b.WriteString("\n\n")
	} else {
		b.WriteString(titleStyle.Render("Theme"))
		b.WriteString("\n\n")
	}

	for i, opt := range m.options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		b.WriteString(cursor)
		b.WriteString(style.Render(opt.Label))

		if opt.Name == m.current {
			b.WriteString(" ")
			b.WriteString(currentStyle.Render("(current)"))
		}

		b.WriteString("\n")

		// Show description for selected item
		if i == m.cursor && opt.Description != "" {
			b.WriteString("    ")
			b.WriteString(dimStyle.Render(opt.Description))
			b.WriteString("\n")
		}
	}

	// Preview swatch
	b.WriteString("\n")
	b.WriteString(m.renderPreview())

	if m.helpText != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  " + m.helpText))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter select · Esc cancel"))

	return b.String()
}

// renderPreview shows a text description of the currently highlighted theme.
func (m Model) renderPreview() string {
	opt := m.options[m.cursor]
	colors := theme.Current().Colors()
	dimStyle := lipgloss.NewStyle().Faint(true)

	var desc string
	switch opt.Name {
	case theme.ThemeDark:
		desc = "Dark background · colored accents · easy on the eyes"
	case theme.ThemeLight:
		desc = "Light background · dark text · bright environments"
	case theme.ThemeHighContrast:
		desc = "Maximum contrast · bold colors · accessibility-first"
	default:
		desc = string(opt.Name)
	}

	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent))
	return fmt.Sprintf("  %s %s\n", accentStyle.Render("✻"), dimStyle.Render(desc))
}

// SelectedTheme returns the currently highlighted theme name.
func (m Model) SelectedTheme() theme.ThemeName {
	return m.options[m.cursor].Name
}
