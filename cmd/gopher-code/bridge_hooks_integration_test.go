package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/ui"
	bridgehooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/bridge"
)

// TestBridgeHooks_WiredThroughAppModel verifies that the bridge/remote hook
// types from pkg/ui/hooks/bridge are wired into the AppModel and reachable
// from the binary. This exercises the real code path: main.go -> ui.NewAppModel
// -> bridge hook fields initialized and Init'd.
func TestBridgeHooks_WiredThroughAppModel(t *testing.T) {
	app := ui.NewAppModel(nil, nil)

	if app.ReplBridgeHook() == nil {
		t.Fatal("expected non-nil ReplBridgeHook from AppModel")
	}
	if app.RemoteSessionHook() == nil {
		t.Fatal("expected non-nil RemoteSessionHook from AppModel")
	}
	if app.MailboxHook() == nil {
		t.Fatal("expected non-nil MailboxBridgeHook from AppModel")
	}

	// ReplBridgeHook: disabled by default (no transport), Init returns nil.
	replHook := app.ReplBridgeHook()
	cmd := replHook.Init()
	if cmd != nil {
		t.Error("expected nil cmd from disabled ReplBridgeHook.Init()")
	}
	if replHook.Status() != bridgehooks.StatusDisconnected {
		t.Errorf("ReplBridgeHook status = %v, want StatusDisconnected", replHook.Status())
	}

	// RemoteSessionHook: nil config (non-remote mode), Init returns nil.
	remoteHook := app.RemoteSessionHook()
	cmd = remoteHook.Init()
	if cmd != nil {
		t.Error("expected nil cmd from nil-config RemoteSessionHook.Init()")
	}
	if remoteHook.IsRemoteMode() {
		t.Error("expected RemoteSessionHook.IsRemoteMode() = false for nil config")
	}

	// MailboxBridgeHook: nil mailbox, Init returns nil.
	mailboxHook := app.MailboxHook()
	cmd = mailboxHook.Init()
	if cmd != nil {
		t.Error("expected nil cmd from nil-mailbox MailboxBridgeHook.Init()")
	}
}

// TestBridgeHooks_AppModelInit verifies that AppModel.Init() includes the
// bridge hook Init methods without panicking.
func TestBridgeHooks_AppModelInit(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	cmd := app.Init()
	// Init should succeed without panicking. The returned cmd may be non-nil
	// (from input.Init) but should not error.
	if cmd != nil {
		_ = cmd()
	}
}

// TestBridgeHooks_StatusMessageRouting verifies that BridgeStatusMsg is
// routed to the bridge hooks through AppModel.Update().
func TestBridgeHooks_StatusMessageRouting(t *testing.T) {
	app := ui.NewAppModel(nil, nil)

	// Send a "repl" status message through Update.
	msg := bridgehooks.BridgeStatusMsg{
		Source: "repl",
		Status: bridgehooks.StatusConnected,
	}
	model, _ := app.Update(msg)
	updated := model.(*ui.AppModel)

	if updated.ReplBridgeHook().Status() != bridgehooks.StatusConnected {
		t.Errorf("after Update with repl StatusConnected, ReplBridgeHook status = %v",
			updated.ReplBridgeHook().Status())
	}

	// Send a "remote" status message.
	remoteMsg := bridgehooks.BridgeStatusMsg{
		Source: "remote",
		Status: bridgehooks.StatusConnected,
	}
	model, _ = updated.Update(remoteMsg)
	updated = model.(*ui.AppModel)

	if updated.RemoteSessionHook().Status() != bridgehooks.StatusConnected {
		t.Errorf("after Update with remote StatusConnected, RemoteSessionHook status = %v",
			updated.RemoteSessionHook().Status())
	}
}

// TestBridgeHooks_RemoteSessionURLRouting verifies RemoteSessionURLMsg routing.
func TestBridgeHooks_RemoteSessionURLRouting(t *testing.T) {
	app := ui.NewAppModel(nil, nil)

	msg := bridgehooks.RemoteSessionURLMsg{
		SessionID: "test-session",
		URL:       "https://claude.ai/code/test-session",
	}
	model, _ := app.Update(msg)
	updated := model.(*ui.AppModel)

	if updated.RemoteSessionHook().URL() != "https://claude.ai/code/test-session" {
		t.Errorf("RemoteSessionHook URL = %q, want test URL",
			updated.RemoteSessionHook().URL())
	}
}

// TestBridgeHooks_MailboxPollRouting verifies MailboxPollMsg routing.
func TestBridgeHooks_MailboxPollRouting(t *testing.T) {
	app := ui.NewAppModel(nil, nil)

	msg := bridgehooks.MailboxPollMsg{}
	model, _ := app.Update(msg)
	_ = model // should not panic
}
