package remote

import (
	"testing"
)

// Source: src/remote/RemoteSessionManager.ts

func TestRemotePermissionResponse_Allow(t *testing.T) {
	// T79: AllowResponse constructor
	resp := AllowResponse(map[string]any{"path": "/tmp/foo"})
	if resp.Behavior != "allow" {
		t.Errorf("Behavior = %q, want %q", resp.Behavior, "allow")
	}
	if resp.UpdatedInput["path"] != "/tmp/foo" {
		t.Errorf("UpdatedInput[path] = %v", resp.UpdatedInput["path"])
	}
	if resp.Message != "" {
		t.Errorf("Message should be empty for allow, got %q", resp.Message)
	}
}

func TestRemotePermissionResponse_Deny(t *testing.T) {
	// T79: DenyResponse constructor
	resp := DenyResponse("user denied")
	if resp.Behavior != "deny" {
		t.Errorf("Behavior = %q, want %q", resp.Behavior, "deny")
	}
	if resp.Message != "user denied" {
		t.Errorf("Message = %q", resp.Message)
	}
	if resp.UpdatedInput != nil {
		t.Errorf("UpdatedInput should be nil for deny")
	}
}

func TestRemoteSessionConfig_Factory(t *testing.T) {
	// T80: CreateRemoteSessionConfig factory
	tokenCalled := false
	cfg := CreateRemoteSessionConfig(
		"sess-123",
		func() string { tokenCalled = true; return "tok" },
		"org-abc",
		true,
		false,
	)

	if cfg.SessionID != "sess-123" {
		t.Errorf("SessionID = %q", cfg.SessionID)
	}
	if cfg.OrgUUID != "org-abc" {
		t.Errorf("OrgUUID = %q", cfg.OrgUUID)
	}
	if !cfg.HasInitialPrompt {
		t.Error("HasInitialPrompt should be true")
	}
	if cfg.ViewerOnly {
		t.Error("ViewerOnly should be false")
	}

	// Verify the token function is wired through.
	tok := cfg.GetAccessToken()
	if !tokenCalled {
		t.Error("GetAccessToken was not called")
	}
	if tok != "tok" {
		t.Errorf("token = %q", tok)
	}
}

func TestRemoteSessionCallbacks_Shape(t *testing.T) {
	// T80: Verify callback struct can hold all expected handlers.
	var called int
	cb := RemoteSessionCallbacks{
		OnMessage:             func(msg any) { called++ },
		OnPermissionRequest:   func(req SDKControlPermissionRequest, id string) { called++ },
		OnPermissionCancelled: func(id string, toolUseID string) { called++ },
		OnConnected:           func() { called++ },
		OnDisconnected:        func() { called++ },
		OnReconnecting:        func() { called++ },
		OnError:               func(err error) { called++ },
	}

	// Fire each callback once.
	cb.OnMessage(nil)
	cb.OnPermissionRequest(SDKControlPermissionRequest{}, "")
	cb.OnPermissionCancelled("", "")
	cb.OnConnected()
	cb.OnDisconnected()
	cb.OnReconnecting()
	cb.OnError(nil)

	if called != 7 {
		t.Errorf("expected 7 callbacks fired, got %d", called)
	}
}

func TestRemoteSessionConfig_ViewerOnly(t *testing.T) {
	// T80: viewer-only config
	cfg := CreateRemoteSessionConfig("s", func() string { return "" }, "o", false, true)
	if !cfg.ViewerOnly {
		t.Error("ViewerOnly should be true")
	}
	if cfg.HasInitialPrompt {
		t.Error("HasInitialPrompt should be false")
	}
}
