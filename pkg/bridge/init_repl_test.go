package bridge

import (
	"errors"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper — build deps with sensible defaults (all gates pass)
// ---------------------------------------------------------------------------

func happyDeps() InitReplDeps {
	return InitReplDeps{
		IsBridgeEnabledBlocking: func() (bool, error) { return true, nil },
		GetBridgeAccessToken:    func() (string, bool) { return "tok_abc", true },
		GetBridgeTokenOverride:  func() (string, bool) { return "", false },
		WaitForPolicyLimitsToLoad: func() error { return nil },
		IsPolicyAllowed:         func(key string) bool { return true },
		GetGlobalConfig:         func() GlobalBridgeConfig { return GlobalBridgeConfig{} },
		SaveGlobalConfig:        func(cfg GlobalBridgeConfig) {},
		GetOAuthTokens: func() *OAuthTokens {
			exp := int64(9999999999999) // far future
			return &OAuthTokens{AccessToken: "tok_abc", ExpiresAt: &exp}
		},
		CheckAndRefreshOAuthToken: func() error { return nil },
		GetOrganizationUUID:       func() (string, error) { return "org-uuid-123", nil },
		NowMillis:                 func() int64 { return 1000 },
		LogDebug:                  func(string) {},
	}
}

// ---------------------------------------------------------------------------
// Test: successful init — all pre-flight gates pass
// ---------------------------------------------------------------------------

func TestInitReplBridge_Success(t *testing.T) {
	// Reset the shim gate so SetCseShimGate inside InitReplBridge doesn't
	// carry over from other tests.
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deps := happyDeps()

	var stateChanges []BridgeState
	opts := &InitBridgeOptions{
		OnStateChange: func(state BridgeState, detail string) {
			stateChanges = append(stateChanges, state)
		},
	}

	handle, err := InitReplBridge(deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle, got nil (bridge was skipped)")
	}
	// Verify the handle carries through the org UUID.
	if handle.OrgUUID != "org-uuid-123" {
		t.Errorf("expected OrgUUID='org-uuid-123', got %q", handle.OrgUUID)
	}
	// No failure state changes should have been emitted.
	for _, s := range stateChanges {
		if s == BridgeStateFailed {
			t.Errorf("unexpected failed state change")
		}
	}
}

// ---------------------------------------------------------------------------
// Test: init failure — bridge not enabled
// ---------------------------------------------------------------------------

func TestInitReplBridge_BridgeDisabled(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deps := happyDeps()
	deps.IsBridgeEnabledBlocking = func() (bool, error) { return false, nil }

	handle, err := InitReplBridge(deps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Fatal("expected nil handle when bridge is disabled")
	}
}

// ---------------------------------------------------------------------------
// Test: init failure — bridge enabled check returns error
// ---------------------------------------------------------------------------

func TestInitReplBridge_BridgeEnabledError(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deps := happyDeps()
	deps.IsBridgeEnabledBlocking = func() (bool, error) {
		return false, errors.New("growthbook timeout")
	}

	handle, err := InitReplBridge(deps, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if handle != nil {
		t.Fatal("expected nil handle on error")
	}
}

// ---------------------------------------------------------------------------
// Test: init failure — no OAuth tokens
// ---------------------------------------------------------------------------

func TestInitReplBridge_NoOAuth(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deps := happyDeps()
	deps.GetBridgeAccessToken = func() (string, bool) { return "", false }

	var mu sync.Mutex
	var gotState BridgeState
	var gotDetail string

	opts := &InitBridgeOptions{
		OnStateChange: func(state BridgeState, detail string) {
			mu.Lock()
			gotState = state
			gotDetail = detail
			mu.Unlock()
		},
	}

	handle, err := InitReplBridge(deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Fatal("expected nil handle when no OAuth")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotState != BridgeStateFailed {
		t.Errorf("expected state 'failed', got %q", gotState)
	}
	if gotDetail != "/login" {
		t.Errorf("expected detail '/login', got %q", gotDetail)
	}
}

// ---------------------------------------------------------------------------
// Test: init failure — policy denied
// ---------------------------------------------------------------------------

func TestInitReplBridge_PolicyDenied(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deps := happyDeps()
	deps.IsPolicyAllowed = func(key string) bool { return false }

	var gotState BridgeState
	var gotDetail string
	opts := &InitBridgeOptions{
		OnStateChange: func(state BridgeState, detail string) {
			gotState = state
			gotDetail = detail
		},
	}

	handle, err := InitReplBridge(deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Fatal("expected nil handle when policy denied")
	}
	if gotState != BridgeStateFailed {
		t.Errorf("expected state 'failed', got %q", gotState)
	}
	if gotDetail != "disabled by your organization's policy" {
		t.Errorf("expected policy detail, got %q", gotDetail)
	}
}

// ---------------------------------------------------------------------------
// Test: init failure — cross-process backoff (dead token >= 3)
// ---------------------------------------------------------------------------

func TestInitReplBridge_CrossProcessBackoff(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deadExpiry := int64(5000)
	deps := happyDeps()
	deps.GetGlobalConfig = func() GlobalBridgeConfig {
		return GlobalBridgeConfig{
			BridgeOauthDeadExpiresAt: &deadExpiry,
			BridgeOauthDeadFailCount: 3,
		}
	}
	deps.GetOAuthTokens = func() *OAuthTokens {
		exp := deadExpiry // same expiresAt as the dead token
		return &OAuthTokens{AccessToken: "tok", ExpiresAt: &exp}
	}

	var debugMsgs []string
	deps.LogDebug = func(msg string) { debugMsgs = append(debugMsgs, msg) }

	handle, err := InitReplBridge(deps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Fatal("expected nil handle on cross-process backoff")
	}
	if len(debugMsgs) == 0 {
		t.Fatal("expected debug log for cross-process backoff")
	}
}

// ---------------------------------------------------------------------------
// Test: init failure — expired token post-refresh
// ---------------------------------------------------------------------------

func TestInitReplBridge_ExpiredTokenPostRefresh(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deps := happyDeps()
	// Token expired at 500, now is 1000.
	deps.GetOAuthTokens = func() *OAuthTokens {
		exp := int64(500)
		return &OAuthTokens{AccessToken: "tok", ExpiresAt: &exp}
	}
	deps.NowMillis = func() int64 { return 1000 }

	var savedCfg GlobalBridgeConfig
	deps.SaveGlobalConfig = func(cfg GlobalBridgeConfig) { savedCfg = cfg }

	var gotState BridgeState
	opts := &InitBridgeOptions{
		OnStateChange: func(state BridgeState, detail string) {
			gotState = state
		},
	}

	handle, err := InitReplBridge(deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Fatal("expected nil handle when token expired")
	}
	if gotState != BridgeStateFailed {
		t.Errorf("expected state 'failed', got %q", gotState)
	}
	// Should have persisted dead token info.
	if savedCfg.BridgeOauthDeadExpiresAt == nil {
		t.Fatal("expected dead expiresAt to be saved")
	}
	if *savedCfg.BridgeOauthDeadExpiresAt != 500 {
		t.Errorf("expected dead expiresAt=500, got %d", *savedCfg.BridgeOauthDeadExpiresAt)
	}
	if savedCfg.BridgeOauthDeadFailCount != 1 {
		t.Errorf("expected fail count=1, got %d", savedCfg.BridgeOauthDeadFailCount)
	}
}

// ---------------------------------------------------------------------------
// Test: init failure — no org UUID
// ---------------------------------------------------------------------------

func TestInitReplBridge_NoOrgUUID(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deps := happyDeps()
	deps.GetOrganizationUUID = func() (string, error) { return "", nil }

	var gotState BridgeState
	var gotDetail string
	opts := &InitBridgeOptions{
		OnStateChange: func(state BridgeState, detail string) {
			gotState = state
			gotDetail = detail
		},
	}

	handle, err := InitReplBridge(deps, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Fatal("expected nil handle when no org UUID")
	}
	if gotState != BridgeStateFailed {
		t.Errorf("expected state 'failed', got %q", gotState)
	}
	if gotDetail != "/login" {
		t.Errorf("expected detail '/login', got %q", gotDetail)
	}
}

// ---------------------------------------------------------------------------
// Test: token override skips cross-process backoff
// ---------------------------------------------------------------------------

func TestInitReplBridge_TokenOverrideSkipsBackoff(t *testing.T) {
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	deadExpiry := int64(5000)
	deps := happyDeps()
	// Token override is set — cross-process backoff should be skipped.
	deps.GetBridgeTokenOverride = func() (string, bool) { return "override-tok", true }
	deps.GetGlobalConfig = func() GlobalBridgeConfig {
		return GlobalBridgeConfig{
			BridgeOauthDeadExpiresAt: &deadExpiry,
			BridgeOauthDeadFailCount: 999, // would normally trigger backoff
		}
	}
	deps.GetOAuthTokens = func() *OAuthTokens {
		exp := deadExpiry
		return &OAuthTokens{AccessToken: "tok", ExpiresAt: &exp}
	}

	handle, err := InitReplBridge(deps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle — token override should skip backoff")
	}
}
