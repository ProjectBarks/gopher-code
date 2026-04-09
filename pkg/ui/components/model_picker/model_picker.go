// Package model_picker provides the model selection UI component.
//
// Source: components/ModelPicker.tsx, utils/model/modelOptions.ts
//
// Displays a selectable list of available Claude models with descriptions,
// effort level indicator, and current selection marker. Used by /model command.
package model_picker

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ModelOption describes a selectable model.
type ModelOption struct {
	Value       string // model ID or "" for "no preference"
	Label       string // display name
	Description string // detail text
}

// EffortLevel describes the reasoning effort.
type EffortLevel string

const (
	EffortLow    EffortLevel = "low"
	EffortMedium EffortLevel = "medium"
	EffortHigh   EffortLevel = "high"
	EffortMax    EffortLevel = "max"
)

// ModelSelectedMsg is sent when the user selects a model.
type ModelSelectedMsg struct {
	Model  string      // model ID, or "" for default
	Effort EffortLevel // effort level, or "" for unchanged
}

// CancelledMsg is sent when the user cancels.
type CancelledMsg struct{}

// NoPreference is the sentinel for "use default model".
const NoPreference = ""

// DefaultModelOptions returns the standard model options for external users.
func DefaultModelOptions() []ModelOption {
	return []ModelOption{
		{Value: NoPreference, Label: "Default (recommended)", Description: "Use the recommended model"},
		{Value: provider.GetModelString("sonnet46"), Label: "Claude Sonnet 4.6", Description: "Fast, intelligent — best for most tasks"},
		{Value: provider.GetModelString("opus46"), Label: "Claude Opus 4.6", Description: "Most capable — complex reasoning and analysis"},
		{Value: provider.GetModelString("haiku45"), Label: "Claude Haiku 4.5", Description: "Fastest, most compact — quick answers"},
	}
}

// Model is the model picker bubbletea model.
type Model struct {
	options  []ModelOption
	cursor   int
	current  string // currently active model
	effort   EffortLevel
	width    int
}

// New creates a model picker with the given options and current model.
func New(options []ModelOption, currentModel string) Model {
	cursor := 0
	for i, opt := range options {
		if opt.Value == currentModel {
			cursor = i
			break
		}
	}
	return Model{
		options: options,
		cursor:  cursor,
		current: currentModel,
		width:   80,
	}
}

// NewDefault creates a model picker with default options.
func NewDefault(currentModel string) Model {
	return New(DefaultModelOptions(), currentModel)
}

// SetEffort sets the current effort level for display.
func (m *Model) SetEffort(level EffortLevel) { m.effort = level }

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
			opt := m.options[m.cursor]
			return m, func() tea.Msg {
				return ModelSelectedMsg{Model: opt.Value, Effort: m.effort}
			}
		case tea.KeyEscape, 'q':
			return m, func() tea.Msg { return CancelledMsg{} }
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
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
	b.WriteString(titleStyle.Render("Select model"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Choose the AI model for this session"))
	b.WriteString("\n\n")

	for i, opt := range m.options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		b.WriteString(cursor)
		b.WriteString(style.Render(opt.Label))

		if opt.Value == m.current {
			b.WriteString(" ")
			b.WriteString(currentStyle.Render("(current)"))
		}

		b.WriteString("\n")

		// Description on next line for selected item
		if i == m.cursor && opt.Description != "" {
			b.WriteString("    ")
			b.WriteString(dimStyle.Render(opt.Description))
			b.WriteString("\n")
		}
	}

	// Effort indicator
	if m.effort != "" {
		b.WriteString("\n")
		effortIcon := effortSymbol(m.effort)
		b.WriteString(fmt.Sprintf("  Effort: %s %s\n", effortIcon, string(m.effort)))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑/↓ navigate · Enter select · Esc cancel"))

	return b.String()
}

// SelectedOption returns the currently highlighted option.
func (m Model) SelectedOption() *ModelOption {
	if m.cursor < 0 || m.cursor >= len(m.options) {
		return nil
	}
	return &m.options[m.cursor]
}

func effortSymbol(level EffortLevel) string {
	switch level {
	case EffortLow:
		return "⚡"
	case EffortMedium:
		return "⚡⚡"
	case EffortHigh:
		return "⚡⚡⚡"
	case EffortMax:
		return "⚡⚡⚡⚡"
	default:
		return ""
	}
}
