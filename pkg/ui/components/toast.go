package components

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ToastType identifies the notification type.
type ToastType int

const (
	ToastSuccess ToastType = iota
	ToastError
	ToastInfo
)

// Toast represents a single notification.
type Toast struct {
	Message   string
	Type      ToastType
	ExpiresAt time.Time
}

// ToastMsg triggers adding a toast notification.
type ToastMsg struct {
	Message  string
	Type     ToastType
	Duration time.Duration
}

// ToastDismissMsg triggers dismissal of expired toasts.
type ToastDismissMsg struct{}

// NotificationToast displays ephemeral success/error messages.
type NotificationToast struct {
	queue   []Toast
	theme   theme.Theme
	width   int
	height  int
	focused bool
}

// NewNotificationToast creates a new notification toast component.
func NewNotificationToast(t theme.Theme) *NotificationToast {
	return &NotificationToast{
		queue: make([]Toast, 0),
		theme: t,
		width: 80,
	}
}

// Init initializes the component.
func (nt *NotificationToast) Init() tea.Cmd { return nil }

// Update handles toast messages and dismissals.
func (nt *NotificationToast) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ToastMsg:
		duration := msg.Duration
		if duration == 0 {
			duration = 3 * time.Second
		}
		nt.queue = append(nt.queue, Toast{
			Message:   msg.Message,
			Type:      msg.Type,
			ExpiresAt: time.Now().Add(duration),
		})
		// Schedule dismissal
		return nt, tea.Tick(duration, func(time.Time) tea.Msg {
			return ToastDismissMsg{}
		})

	case ToastDismissMsg:
		nt.dismissExpired()
	}
	return nt, nil
}

// View renders the active toasts.
func (nt *NotificationToast) View() tea.View {
	if len(nt.queue) == 0 {
		return tea.NewView("")
	}

	cs := nt.theme.Colors()
	// Show the most recent toast
	toast := nt.queue[len(nt.queue)-1]

	var icon string
	var style lipgloss.Style

	switch toast.Type {
	case ToastSuccess:
		icon = "✓"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(cs.Success)).Bold(true)
	case ToastError:
		icon = "✗"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(cs.Error)).Bold(true)
	default:
		icon = "ℹ"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(cs.Info)).Bold(true)
	}

	return tea.NewView(style.Render(icon + " " + toast.Message))
}

func (nt *NotificationToast) dismissExpired() {
	now := time.Now()
	active := make([]Toast, 0, len(nt.queue))
	for _, t := range nt.queue {
		if t.ExpiresAt.After(now) {
			active = append(active, t)
		}
	}
	nt.queue = active
}

// HasToasts returns true if there are active notifications.
func (nt *NotificationToast) HasToasts() bool {
	return len(nt.queue) > 0
}

// Count returns the number of active notifications.
func (nt *NotificationToast) Count() int {
	return len(nt.queue)
}

// Clear removes all notifications.
func (nt *NotificationToast) Clear() {
	nt.queue = nt.queue[:0]
}

// SetSize sets the dimensions.
func (nt *NotificationToast) SetSize(width, height int) {
	nt.width = width
	nt.height = height
}

func (nt *NotificationToast) Focus()        { nt.focused = true }
func (nt *NotificationToast) Blur()         { nt.focused = false }
func (nt *NotificationToast) Focused() bool { return nt.focused }

var _ tea.Model = (*NotificationToast)(nil)
