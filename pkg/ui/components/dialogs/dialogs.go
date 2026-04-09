// Package dialogs provides generic dialog and status components.
// Source: components/ — InvalidConfigDialog, ExportDialog, IdleReturnDialog,
//         MCPServerApprovalDialog, toast notifications, error alerts
package dialogs

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// --- Confirmation Dialog ---

// ConfirmMsg carries the user's yes/no decision.
type ConfirmMsg struct {
	Confirmed bool
	ID        string // optional identifier
}

// ConfirmModel is a simple yes/no confirmation dialog.
type ConfirmModel struct {
	title    string
	message  string
	id       string
	selected int // 0=yes, 1=no
}

// NewConfirm creates a confirmation dialog.
func NewConfirm(title, message, id string) ConfirmModel {
	return ConfirmModel{title: title, message: message, id: id}
}

func (m ConfirmModel) Init() tea.Cmd { return nil }

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case 'y':
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: true, ID: m.id} }
		case 'n', tea.KeyEscape:
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: false, ID: m.id} }
		case tea.KeyUp, tea.KeyLeft:
			m.selected = 0
		case tea.KeyDown, tea.KeyRight:
			m.selected = 1
		case tea.KeyEnter:
			return m, func() tea.Msg { return ConfirmMsg{Confirmed: m.selected == 0, ID: m.id} }
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	dimStyle := lipgloss.NewStyle().Faint(true)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(m.title))
	sb.WriteString("\n\n")
	sb.WriteString(m.message)
	sb.WriteString("\n\n")

	options := []string{"Yes", "No"}
	for i, opt := range options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.selected {
			cursor = "> "
			style = selStyle
		}
		sb.WriteString(cursor + style.Render(opt) + "  ")
	}
	sb.WriteString("\n\n")
	sb.WriteString(dimStyle.Render("y/n or Enter to confirm"))
	return sb.String()
}

// --- Toast Notification ---

// ToastLevel classifies toast severity.
type ToastLevel string

const (
	ToastInfo    ToastLevel = "info"
	ToastSuccess ToastLevel = "success"
	ToastWarning ToastLevel = "warning"
	ToastError   ToastLevel = "error"
)

// Toast is a temporary notification shown at the top/bottom of the TUI.
type Toast struct {
	Message string
	Level   ToastLevel
	Key     string // unique key for dedup
}

// RenderToast renders a toast notification.
func RenderToast(t Toast) string {
	var icon string
	var style lipgloss.Style

	switch t.Level {
	case ToastSuccess:
		icon = "✓"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	case ToastWarning:
		icon = "⚠"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	case ToastError:
		icon = "✗"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	default:
		icon = "ℹ"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	}

	return style.Render(icon + " " + t.Message)
}

// --- Error Alert ---

// RenderErrorAlert renders a prominent error message with title and details.
func RenderErrorAlert(title, detail string) string {
	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Bold(true)
	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9"))

	var sb strings.Builder
	sb.WriteString(errStyle.Render("✗ " + title))
	if detail != "" {
		sb.WriteString("\n")
		sb.WriteString(detailStyle.Render("  " + detail))
	}
	return sb.String()
}

// --- Config Error Dialog ---

// RenderConfigError renders the invalid config dialog content.
// Source: components/InvalidConfigDialog.tsx
func RenderConfigError(filePath, errorDesc string) string {
	return RenderErrorAlert(
		"Invalid configuration",
		filePath+": "+errorDesc,
	)
}
