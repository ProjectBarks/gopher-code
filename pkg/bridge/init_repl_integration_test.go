package bridge

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Integration tests — exercise InitReplBridge through the same wiring
// pattern used by cmd/gopher-code/main.go's remote-control path (T194).
// ---------------------------------------------------------------------------

// TestInitReplBridge_BinaryWiring_Success simulates the wiring pattern from
// main.go: constructing InitReplDeps and InitBridgeOptions the same way the
// binary does, then verifying the handle is returned correctly.
func TestInitReplBridge_BinaryWiring_Success(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	// Simulate the bridgeDeps.GetAccessToken path from main.go.
	getAccessToken := func() (string, bool) { return "test-access-tok", true }

	// Simulate the bridge debug logger created in the remote-control path.
	bridgeDebug := NewBridgeDebug(LogLevelDebug, DefaultBufferSize, nil)
	SetGlobalBridgeDebug(bridgeDebug)
	defer SetGlobalBridgeDebug(nil)

	// Simulate the status machine created in main.go.
	bridgeStatus := NewStatusMachine()
	var lastTransition BridgeStatus
	bridgeStatus.OnStatusChange(func(from, to BridgeStatus) {
		lastTransition = to
	})

	// Build deps the same way main.go does.
	deps := InitReplDeps{
		IsBridgeEnabledBlocking:   func() (bool, error) { return true, nil },
		GetBridgeAccessToken:      getAccessToken,
		GetBridgeTokenOverride:    func() (string, bool) { return "", false },
		WaitForPolicyLimitsToLoad: func() error { return nil },
		IsPolicyAllowed:           func(key string) bool { return true },
		GetGlobalConfig:           func() GlobalBridgeConfig { return GlobalBridgeConfig{} },
		SaveGlobalConfig:          func(cfg GlobalBridgeConfig) {},
		GetOAuthTokens: func() *OAuthTokens {
			exp := time.Now().Add(time.Hour).UnixMilli()
			return &OAuthTokens{AccessToken: "test-access-tok", ExpiresAt: &exp}
		},
		CheckAndRefreshOAuthToken: func() error { return nil },
		GetOrganizationUUID:       func() (string, error) { return "org-integration-test", nil },
		LogDebug:                  func(msg string) { bridgeDebug.LogStatus(msg, nil) },
	}

	// Build opts the same way main.go does.
	opts := &InitBridgeOptions{
		InitialName: "test-session",
		OnStateChange: func(state BridgeState, detail string) {
			_ = bridgeStatus.Transition(StatusConnecting)
		},
	}

	handle, err := InitReplBridge(deps, opts)
	if err != nil {
		t.Fatalf("InitReplBridge returned error: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle from successful init")
	}
	if handle.OrgUUID != "org-integration-test" {
		t.Errorf("OrgUUID = %q, want %q", handle.OrgUUID, "org-integration-test")
	}

	// The OnStateChange callback should NOT have been called on success
	// (it's only invoked on failure paths), so no transition should have fired.
	if lastTransition != "" {
		t.Errorf("unexpected status transition on success: %q", lastTransition)
	}
}

// TestInitReplBridge_BinaryWiring_Skipped simulates the binary wiring when
// bridge is not enabled — verifying that the binary path handles a nil handle
// gracefully (no error, nil handle).
func TestInitReplBridge_BinaryWiring_Skipped(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	bridgeDebug := NewBridgeDebug(LogLevelDebug, DefaultBufferSize, nil)
	SetGlobalBridgeDebug(bridgeDebug)
	defer SetGlobalBridgeDebug(nil)

	deps := InitReplDeps{
		IsBridgeEnabledBlocking:   func() (bool, error) { return false, nil },
		GetBridgeAccessToken:      func() (string, bool) { return "tok", true },
		GetBridgeTokenOverride:    func() (string, bool) { return "", false },
		WaitForPolicyLimitsToLoad: func() error { return nil },
		IsPolicyAllowed:           func(key string) bool { return true },
		GetGlobalConfig:           func() GlobalBridgeConfig { return GlobalBridgeConfig{} },
		SaveGlobalConfig:          func(cfg GlobalBridgeConfig) {},
		GetOAuthTokens:            func() *OAuthTokens { return nil },
		GetOrganizationUUID:       func() (string, error) { return "org-123", nil },
		LogDebug:                  func(msg string) { bridgeDebug.LogStatus(msg, nil) },
	}

	handle, err := InitReplBridge(deps, &InitBridgeOptions{
		InitialName: "skip-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Fatal("expected nil handle when bridge is disabled")
	}
}

// TestInitReplBridge_BinaryWiring_FailedState verifies that the OnStateChange
// callback fires with BridgeStateFailed when OAuth is missing, matching the
// pattern the binary uses to drive status transitions.
func TestInitReplBridge_BinaryWiring_FailedState(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	bridgeDebug := NewBridgeDebug(LogLevelDebug, DefaultBufferSize, nil)
	SetGlobalBridgeDebug(bridgeDebug)
	defer SetGlobalBridgeDebug(nil)

	bridgeStatus := NewStatusMachine()
	var transitioned bool
	bridgeStatus.OnStatusChange(func(from, to BridgeStatus) {
		transitioned = true
	})

	deps := InitReplDeps{
		IsBridgeEnabledBlocking:   func() (bool, error) { return true, nil },
		GetBridgeAccessToken:      func() (string, bool) { return "", false }, // no OAuth
		GetBridgeTokenOverride:    func() (string, bool) { return "", false },
		WaitForPolicyLimitsToLoad: func() error { return nil },
		IsPolicyAllowed:           func(key string) bool { return true },
		GetGlobalConfig:           func() GlobalBridgeConfig { return GlobalBridgeConfig{} },
		SaveGlobalConfig:          func(cfg GlobalBridgeConfig) {},
		GetOAuthTokens:            func() *OAuthTokens { return nil },
		GetOrganizationUUID:       func() (string, error) { return "org-123", nil },
		LogDebug:                  func(msg string) { bridgeDebug.LogStatus(msg, nil) },
	}

	var gotState BridgeState
	opts := &InitBridgeOptions{
		OnStateChange: func(state BridgeState, detail string) {
			gotState = state
			// Mirror main.go: drive status machine on failure.
			_ = bridgeStatus.Transition(StatusConnecting)
		},
	}

	handle, err := InitReplBridge(deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Fatal("expected nil handle when no OAuth")
	}
	if gotState != BridgeStateFailed {
		t.Errorf("expected BridgeStateFailed, got %q", gotState)
	}
	if !transitioned {
		t.Error("expected status machine transition on failure callback")
	}
}
