// Package selectcomp provides a scrollable select list component.
// Source: components/CustomSelect/ (10 files)
//
// In TS, CustomSelect is a React component with hooks for state, navigation,
// input, and rendering. In Go, this is a single bubbletea Model that handles
// all selection logic: scrolling, filtering, descriptions, and submit/cancel.
package selectcomp

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Option is a selectable item with optional description.
type Option struct {
	Label       string
	Value       string
	Description string
	Disabled    bool
}

// SelectMsg is returned when the user selects an option.
type SelectMsg struct {
	Value string
	Label string
	Index int
}

// CancelMsg is returned when the user cancels.
type CancelMsg struct{}

// Model is a scrollable, filterable select list.
type Model struct {
	options     []Option
	filtered    []int // indices into options matching filter
	selected    int   // index into filtered
	scroll      int
	maxVisible  int
	filter      string
	filtering   bool
	title       string
	width       int
}

// New creates a select list with the given options.
func New(title string, options []Option) Model {
	m := Model{
		options:    options,
		maxVisible: 8,
		title:      title,
		width:      60,
	}
	m.resetFilter()
	return m
}

// SetMaxVisible sets the maximum number of visible options.
func (m *Model) SetMaxVisible(n int) { m.maxVisible = n }

// SetWidth sets the rendering width.
func (m *Model) SetWidth(w int) { m.width = w }

func (m *Model) resetFilter() {
	m.filtered = make([]int, len(m.options))
	for i := range m.options {
		m.filtered[i] = i
	}
}

func (m *Model) applyFilter() {
	if m.filter == "" {
		m.resetFilter()
		return
	}
	lower := strings.ToLower(m.filter)
	m.filtered = m.filtered[:0]
	for i, opt := range m.options {
		if strings.Contains(strings.ToLower(opt.Label), lower) ||
			strings.Contains(strings.ToLower(opt.Description), lower) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.selected >= len(m.filtered) {
		m.selected = max(0, len(m.filtered)-1)
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEscape:
			if m.filtering && m.filter != "" {
				m.filter = ""
				m.filtering = false
				m.resetFilter()
				return m, nil
			}
			return m, func() tea.Msg { return CancelMsg{} }

		case tea.KeyUp, 'k':
			if !m.filtering {
				m.moveUp()
			}
		case tea.KeyDown, 'j':
			if !m.filtering {
				m.moveDown()
			}

		case tea.KeyEnter:
			if len(m.filtered) > 0 {
				idx := m.filtered[m.selected]
				opt := m.options[idx]
				if !opt.Disabled {
					return m, func() tea.Msg {
						return SelectMsg{Value: opt.Value, Label: opt.Label, Index: idx}
					}
				}
			}

		case '/':
			if !m.filtering {
				m.filtering = true
				m.filter = ""
				return m, nil
			}
			m.filter += "/"
			m.applyFilter()

		case tea.KeyBackspace:
			if m.filtering && len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
				if m.filter == "" {
					m.filtering = false
				}
			}

		default:
			if m.filtering && msg.Text != "" {
				m.filter += msg.Text
				m.applyFilter()
			}
		}
	}
	return m, nil
}

func (m *Model) moveUp() {
	if m.selected > 0 {
		m.selected--
		if m.selected < m.scroll {
			m.scroll = m.selected
		}
	}
}

func (m *Model) moveDown() {
	if m.selected < len(m.filtered)-1 {
		m.selected++
		if m.selected >= m.scroll+m.maxVisible {
			m.scroll = m.selected - m.maxVisible + 1
		}
	}
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	descStyle := lipgloss.NewStyle().Faint(true)
	disabledStyle := lipgloss.NewStyle().Faint(true).Strikethrough(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder

	if m.title != "" {
		sb.WriteString(titleStyle.Render(m.title))
		sb.WriteString("\n\n")
	}

	if m.filtering {
		sb.WriteString("/ " + m.filter + "█\n\n")
	}

	if len(m.filtered) == 0 {
		sb.WriteString(dimStyle.Render("No matching options"))
		return sb.String()
	}

	end := m.scroll + m.maxVisible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.scroll; i < end; i++ {
		opt := m.options[m.filtered[i]]
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.selected {
			cursor = "> "
			style = selStyle
		}
		if opt.Disabled {
			style = disabledStyle
		}

		sb.WriteString(cursor + style.Render(opt.Label) + "\n")
		if opt.Description != "" {
			sb.WriteString("    " + descStyle.Render(opt.Description) + "\n")
		}
	}

	if m.scroll > 0 {
		sb.WriteString(dimStyle.Render("  ↑ more") + "\n")
	}
	if end < len(m.filtered) {
		sb.WriteString(dimStyle.Render("  ↓ more") + "\n")
	}

	sb.WriteString("\n")
	hint := "↑/↓ select, Enter confirm, Escape cancel"
	if !m.filtering {
		hint += ", / filter"
	}
	sb.WriteString(dimStyle.Render(hint))

	return sb.String()
}

// SelectedOption returns the currently highlighted option, or nil.
func (m Model) SelectedOption() *Option {
	if len(m.filtered) == 0 || m.selected >= len(m.filtered) {
		return nil
	}
	opt := m.options[m.filtered[m.selected]]
	return &opt
}

// OptionCount returns the number of visible (filtered) options.
func (m Model) OptionCount() int { return len(m.filtered) }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
