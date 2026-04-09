// Package interactive provides small interactive UI widgets.
//
// Source: components/LanguagePicker.tsx, ExportDialog.tsx, ExitFlow.tsx,
//         WorktreeExitDialog.tsx, BridgeDialog.tsx, etc.
//
// These are the "23 interactive components" — small dialogs and pickers
// that handle user input for specific operations.
package interactive

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ---------------------------------------------------------------------------
// LanguagePicker — Source: components/LanguagePicker.tsx
// ---------------------------------------------------------------------------

// LanguageSelectedMsg is sent when the user picks a language.
type LanguageSelectedMsg struct {
	Language string // empty = default (English)
}

// LanguageCancelledMsg is sent when the user cancels.
type LanguageCancelledMsg struct{}

// LanguagePickerModel lets the user type a preferred language.
type LanguagePickerModel struct {
	value       string
	placeholder string
}

// NewLanguagePicker creates a language picker with optional initial value.
func NewLanguagePicker(initial string) LanguagePickerModel {
	return LanguagePickerModel{
		value:       initial,
		placeholder: "e.g., Japanese, 日本語, Español…",
	}
}

func (m LanguagePickerModel) Init() tea.Cmd { return nil }

func (m LanguagePickerModel) Update(msg tea.Msg) (LanguagePickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEnter:
			lang := strings.TrimSpace(m.value)
			return m, func() tea.Msg { return LanguageSelectedMsg{Language: lang} }
		case tea.KeyEscape:
			return m, func() tea.Msg { return LanguageCancelledMsg{} }
		case tea.KeyBackspace:
			if len(m.value) > 0 {
				m.value = m.value[:len(m.value)-1]
			}
		default:
			if msg.Code >= 32 && msg.Code < 127 && msg.Mod == 0 {
				m.value += string(rune(msg.Code))
			}
		}
	}
	return m, nil
}

func (m LanguagePickerModel) View() string {
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString("Enter your preferred response and voice language:\n\n")
	b.WriteString("❯ ")
	if m.value == "" {
		b.WriteString(dimStyle.Render(m.placeholder))
	} else {
		b.WriteString(m.value)
	}
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Leave empty for default (English)"))
	return b.String()
}

// ---------------------------------------------------------------------------
// ExportDialog — Source: components/ExportDialog.tsx
// ---------------------------------------------------------------------------

// ExportFormat is the format for conversation export.
type ExportFormat string

const (
	ExportJSON     ExportFormat = "json"
	ExportMarkdown ExportFormat = "markdown"
	ExportText     ExportFormat = "text"
)

// ExportSelectedMsg is sent when the user picks an export format.
type ExportSelectedMsg struct {
	Format ExportFormat
	Path   string
}

// ExportCancelledMsg is sent when the user cancels.
type ExportCancelledMsg struct{}

// ExportDialogModel lets the user choose an export format.
type ExportDialogModel struct {
	formats []ExportFormat
	cursor  int
}

// NewExportDialog creates an export format picker.
func NewExportDialog() ExportDialogModel {
	return ExportDialogModel{
		formats: []ExportFormat{ExportJSON, ExportMarkdown, ExportText},
	}
}

func (m ExportDialogModel) Init() tea.Cmd { return nil }

func (m ExportDialogModel) Update(msg tea.Msg) (ExportDialogModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, 'j':
			if m.cursor < len(m.formats)-1 {
				m.cursor++
			}
		case tea.KeyEnter:
			f := m.formats[m.cursor]
			return m, func() tea.Msg { return ExportSelectedMsg{Format: f} }
		case tea.KeyEscape:
			return m, func() tea.Msg { return ExportCancelledMsg{} }
		}
	}
	return m, nil
}

func (m ExportDialogModel) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Export Conversation"))
	b.WriteString("\n\n")

	descs := map[ExportFormat]string{
		ExportJSON:     "Full conversation data with metadata",
		ExportMarkdown: "Formatted markdown with code blocks",
		ExportText:     "Plain text transcript",
	}

	for i, f := range m.formats {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(string(f))))
		if i == m.cursor {
			b.WriteString("    " + dimStyle.Render(descs[f]) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Enter select · Esc cancel"))
	return b.String()
}

// ---------------------------------------------------------------------------
// WorktreeExitDialog — Source: components/WorktreeExitDialog.tsx
// ---------------------------------------------------------------------------

// WorktreeAction is what to do when exiting a worktree.
type WorktreeAction string

const (
	WorktreeKeep   WorktreeAction = "keep"
	WorktreeDelete WorktreeAction = "delete"
	WorktreeCancel WorktreeAction = "cancel"
)

// WorktreeExitMsg is sent when the user decides what to do with the worktree.
type WorktreeExitMsg struct {
	Action WorktreeAction
}

// WorktreeExitModel asks the user what to do with a worktree on exit.
type WorktreeExitModel struct {
	WorktreeName string
	BranchName   string
	cursor       int
}

// NewWorktreeExitDialog creates a worktree exit dialog.
func NewWorktreeExitDialog(name, branch string) WorktreeExitModel {
	return WorktreeExitModel{WorktreeName: name, BranchName: branch}
}

func (m WorktreeExitModel) Init() tea.Cmd { return nil }

func (m WorktreeExitModel) Update(msg tea.Msg) (WorktreeExitModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k':
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown, 'j':
			if m.cursor < 2 {
				m.cursor++
			}
		case tea.KeyEnter:
			actions := []WorktreeAction{WorktreeKeep, WorktreeDelete, WorktreeCancel}
			a := actions[m.cursor]
			return m, func() tea.Msg { return WorktreeExitMsg{Action: a} }
		case tea.KeyEscape:
			return m, func() tea.Msg { return WorktreeExitMsg{Action: WorktreeCancel} }
		}
	}
	return m, nil
}

func (m WorktreeExitModel) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Warning))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Exit Worktree"))
	b.WriteString("\n\n")
	if m.WorktreeName != "" {
		b.WriteString(fmt.Sprintf("  Worktree: %s\n", m.WorktreeName))
	}
	if m.BranchName != "" {
		b.WriteString(fmt.Sprintf("  Branch:   %s\n", m.BranchName))
	}
	b.WriteString("\n")

	options := []struct {
		label string
		desc  string
	}{
		{"Keep worktree", "Leave the worktree on disk for later use"},
		{"Delete worktree", "Remove the worktree and clean up"},
		{"Cancel", "Stay in the worktree"},
	}
	for i, opt := range options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt.label)))
		if i == m.cursor {
			b.WriteString("    " + dimStyle.Render(opt.desc) + "\n")
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// ConfirmDialog — Generic yes/no confirmation
// ---------------------------------------------------------------------------

// ConfirmResult is the user's choice.
type ConfirmResult bool

// ConfirmMsg is sent with the user's choice.
type ConfirmMsg struct {
	Confirmed bool
	ID        string // optional identifier for which confirm this is
}

// ConfirmModel is a generic yes/no dialog.
type ConfirmModel struct {
	Title   string
	Message string
	ID      string
	cursor  int // 0=yes, 1=no
}

// NewConfirmDialog creates a confirmation dialog.
func NewConfirmDialog(title, message, id string) ConfirmModel {
	return ConfirmModel{Title: title, Message: message, ID: id}
}

func (m ConfirmModel) Init() tea.Cmd { return nil }

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp, 'k', tea.KeyLeft:
			m.cursor = 0
		case tea.KeyDown, 'j', tea.KeyRight:
			m.cursor = 1
		case 'y', 'Y':
			id := m.ID
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: true, ID: id} }
		case 'n', 'N', tea.KeyEscape:
			id := m.ID
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: false, ID: id} }
		case tea.KeyEnter:
			confirmed := m.cursor == 0
			id := m.ID
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: confirmed, ID: id} }
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	colors := theme.Current().Colors()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Primary))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.Title))
	b.WriteString("\n\n")
	if m.Message != "" {
		b.WriteString("  " + m.Message + "\n\n")
	}

	options := []string{"Yes", "No"}
	for i, opt := range options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(fmt.Sprintf("%s%s  ", cursor, style.Render(opt)))
	}
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  y/n · Enter · Esc cancel"))
	return b.String()
}
