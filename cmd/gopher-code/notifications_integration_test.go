package main

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/notifications"
)

func TestNotifications_IntegrationThroughAppModel(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	if app.NotifManager() == nil {
		t.Fatal("NotifManager() should not be nil after NewAppModel")
	}
	if app.NotifToast() == nil {
		t.Fatal("NotifToast() should not be nil after NewAppModel")
	}

	mgr := app.NotifManager()
	mgr.RunStartupChecks(notifications.StartupOptions{
		SettingsErrors: []notifications.SettingsError{
			{Path: "model", Message: "invalid model name"},
		},
	})

	notifs := mgr.Notifications()
	if len(notifs) == 0 {
		t.Fatal("expected at least one notification from settings error")
	}

	found := false
	for _, n := range notifs {
		if n.Key == "settings-errors" {
			found = true
			if !strings.Contains(n.Message, "1 settings issue") {
				t.Errorf("settings error message = %q, want '1 settings issue'", n.Message)
			}
			if !strings.Contains(n.Message, "/doctor") {
				t.Errorf("settings error message = %q, should mention /doctor", n.Message)
			}
			if n.Priority != notifications.PriorityHigh {
				t.Errorf("settings error priority = %d, want PriorityHigh", n.Priority)
			}
		}
	}
	if !found {
		t.Error("expected settings-errors notification, not found")
	}
}

func TestNotifications_RateLimitThroughManager(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	mgr := app.NotifManager()
	mgr.RunStartupChecks(notifications.StartupOptions{
		RateLimit: notifications.RateLimitState{
			IsUsingOverage: true,
			OverageText:    "Exceeded Pro plan limit",
		},
	})

	notifs := mgr.Notifications()
	found := false
	for _, n := range notifs {
		if n.Key == "limit-reached" {
			found = true
			if n.Message != "Exceeded Pro plan limit" {
				t.Errorf("message = %q", n.Message)
			}
			if n.Priority != notifications.PriorityImmediate {
				t.Errorf("priority = %d, want PriorityImmediate", n.Priority)
			}
		}
	}
	if !found {
		t.Error("expected limit-reached notification")
	}
}

func TestNotifications_FastModeThroughManager(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	mgr := app.NotifManager()
	mgr.AddFastModeEvent(notifications.FastModeEvent{
		Type: notifications.FastModeCooldownExpired,
	})

	notifs := mgr.Notifications()
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifs))
	}
	if notifs[0].Key != "fast-mode-cooldown-expired" {
		t.Errorf("key = %q", notifs[0].Key)
	}
	if !strings.Contains(notifs[0].Message, "Fast limit reset") {
		t.Errorf("message = %q", notifs[0].Message)
	}
}

func TestNotifications_EmptyStartupProducesNoToasts(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	mgr := app.NotifManager()
	mgr.RunStartupChecks(notifications.StartupOptions{})

	if len(mgr.Notifications()) != 0 {
		t.Errorf("expected 0 notifications for clean startup, got %d", len(mgr.Notifications()))
	}
}

func TestNotifications_PackageInBinary(t *testing.T) {
	var _ notifications.Manager
	var _ notifications.Notification
	var _ notifications.StartupOptions
	var _ notifications.RateLimitState
	var _ notifications.FastModeEvent
	var _ notifications.MigrationConfig
	var _ notifications.SettingsError
}
