package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/output_styles"
)

// Source: components/OutputStylePicker.tsx
//
// Displays a selectable list of output styles (default, Explanatory, Learning,
// plus any user/project custom styles). In TS this uses CustomSelect; in Go
// we render a simple list with Up/Down/Enter.

// OutputStyleSelectedMsg signals the user picked an output style.
type OutputStyleSelectedMsg struct {
	StyleName string
}

// OutputStyleCanceledMsg signals the user canceled the picker.
type OutputStyleCanceledMsg struct{}

// OutputStylePickerModel is the output style selection list.
type OutputStylePickerModel struct {
	styles   []styleOption
	selected int
	cwd      string
}

type styleOption struct {
	name        string
	description string
}

// NewOutputStylePicker creates a picker with styles loaded from the given cwd.
func NewOutputStylePicker(cwd string) OutputStylePickerModel {
	allStyles := output_styles.GetAllOutputStyles(cwd)

	var options []styleOption
	for name, cfg := range allStyles {
		desc := "Claude completes coding tasks efficiently and provides concise responses"
		displayName := "Default"
		if cfg != nil {
			displayName = cfg.Name
			desc = cfg.Description
		}
		if name == output_styles.DefaultOutputStyleName {
			displayName = "Default"
		}
		options = append(options, styleOption{name: displayName, description: desc})
	}

	// Ensure Default is first
	for i, o := range options {
		if o.name == "Default" && i > 0 {
			options[0], options[i] = options[i], options[0]
			break
		}
	}

	return OutputStylePickerModel{styles: options, cwd: cwd}
}

func (m OutputStylePickerModel) Init() tea.Cmd { return nil }

func (m OutputStylePickerModel) Update(msg tea.Msg) (OutputStylePickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.selected > 0 {
				m.selected--
			}
		case tea.KeyDown, 'j':
			if m.selected < len(m.styles)-1 {
				m.selected++
			}
		case tea.KeyEnter:
			name := m.styles[m.selected].name
			return m, func() tea.Msg { return OutputStyleSelectedMsg{StyleName: name} }
		case tea.KeyEscape:
			return m, func() tea.Msg { return OutputStyleCanceledMsg{} }
		}
	}
	return m, nil
}

func (m OutputStylePickerModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	descStyle := lipgloss.NewStyle().Faint(true)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Choose an output style"))
	sb.WriteString("\n\n")

	for i, opt := range m.styles {
		cursor := "  "
		nameStyle := lipgloss.NewStyle()
		if i == m.selected {
			cursor = "> "
			nameStyle = selectedStyle
		}
		sb.WriteString(cursor + nameStyle.Render(opt.name) + "\n")
		sb.WriteString("    " + descStyle.Render(opt.description) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓ to select, Enter to confirm, Escape to cancel"))
	return sb.String()
}

// SelectedStyle returns the currently highlighted style name.
func (m OutputStylePickerModel) SelectedStyle() string {
	if m.selected < len(m.styles) {
		return m.styles[m.selected].name
	}
	return ""
}
