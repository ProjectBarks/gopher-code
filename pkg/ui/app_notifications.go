// T400: Notification hooks integration.
package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/notifications"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// notifState holds the T400 notification hooks state for AppModel.
type notifState struct {
	manager *notifications.Manager
	toast   *components.NotificationToast
}

func initNotifState() *notifState {
	return &notifState{
		manager: notifications.NewManager(),
		toast:   components.NewNotificationToast(theme.Current()),
	}
}

func (ns *notifState) runStartupChecks(opts notifications.StartupOptions) []tea.Cmd {
	ns.manager.RunStartupChecks(opts)
	return ns.toastCmds()
}

func (ns *notifState) toastCmds() []tea.Cmd {
	notifs := ns.manager.Notifications()
	if len(notifs) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(notifs))
	for _, n := range notifs {
		n := n
		dur := time.Duration(n.TimeoutMs) * time.Millisecond
		if dur == 0 {
			dur = 3 * time.Second
		}
		toastType := components.ToastInfo
		if n.Priority == notifications.PriorityImmediate {
			toastType = components.ToastError
		}
		cmds = append(cmds, func() tea.Msg {
			return components.ToastMsg{
				Message:  n.Message,
				Type:     toastType,
				Duration: dur,
			}
		})
	}
	ns.manager.Clear()
	return cmds
}

// NotifManager returns the notification manager for external access.
func (a *AppModel) NotifManager() *notifications.Manager {
	if a.notifs == nil {
		return nil
	}
	return a.notifs.manager
}

// NotifToast returns the toast component for inspection.
func (a *AppModel) NotifToast() *components.NotificationToast {
	if a.notifs == nil {
		return nil
	}
	return a.notifs.toast
}
