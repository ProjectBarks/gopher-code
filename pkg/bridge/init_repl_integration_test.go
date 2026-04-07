package bridge

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

// ---------------------------------------------------------------------------
// T197: Integration test — V2 transport wired after InitReplBridge
// ---------------------------------------------------------------------------

// TestV2Transport_BinaryWiring exercises the same wiring pattern used in
// cmd/gopher-code/main.go: after InitReplBridge succeeds, a V2 transport
// is created with the session URL and auth token, callbacks are registered
// for status-machine transitions, and Connect() is called.
func TestV2Transport_BinaryWiring(t *testing.T) {
	t.Parallel()
	resetCseShimGate()
	t.Cleanup(func() { resetCseShimGate() })

	// Stand up a minimal mock server for the V2 transport endpoints.
	var initCalled atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header is present.
		if r.Header.Get("Authorization") == "" {
			t.Error("SSE request missing Authorization header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		// Send one event then hold open until cancelled.
		fmt.Fprintf(w, "id: 1\ndata: {\"type\":\"ping\"}\n\n")
		flusher.Flush()
		<-r.Context().Done()
	})
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		initCalled.Store(true)
		// Verify auth + content-type.
		if r.Header.Get("Authorization") == "" {
			t.Error("initialize request missing Authorization header")
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Error("initialize request has empty body")
		}
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Run InitReplBridge (mirrors main.go T194 block).
	bridgeDebug := NewBridgeDebug(LogLevelDebug, DefaultBufferSize, nil)
	SetGlobalBridgeDebug(bridgeDebug)
	defer SetGlobalBridgeDebug(nil)

	deps := InitReplDeps{
		IsBridgeEnabledBlocking:   func() (bool, error) { return true, nil },
		GetBridgeAccessToken:      func() (string, bool) { return "test-tok", true },
		GetBridgeTokenOverride:    func() (string, bool) { return "", false },
		WaitForPolicyLimitsToLoad: func() error { return nil },
		IsPolicyAllowed:           func(key string) bool { return true },
		GetGlobalConfig:           func() GlobalBridgeConfig { return GlobalBridgeConfig{} },
		SaveGlobalConfig:          func(cfg GlobalBridgeConfig) {},
		GetOAuthTokens: func() *OAuthTokens {
			exp := time.Now().Add(time.Hour).UnixMilli()
			return &OAuthTokens{AccessToken: "test-tok", ExpiresAt: &exp}
		},
		CheckAndRefreshOAuthToken: func() error { return nil },
		GetOrganizationUUID:       func() (string, error) { return "org-v2-wiring", nil },
		LogDebug:                  func(msg string) { bridgeDebug.LogStatus(msg, nil) },
	}

	handle, err := InitReplBridge(deps, &InitBridgeOptions{InitialName: "v2-wiring-test"})
	if err != nil {
		t.Fatalf("InitReplBridge error: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle")
	}

	// 2. Wire V2 transport the same way main.go does (T197 integration).
	bridgeURL := srv.URL
	bridgeToken := "test-tok"

	connectedCh := make(chan struct{}, 1)
	var dataReceived atomic.Bool
	var transportConnected atomic.Bool

	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:   bridgeURL,
		IngressToken: bridgeToken,
		SessionID:    handle.OrgUUID,
		GetAuthToken: func() string { return "test-tok" },
		Logger:       func(msg string) { bridgeDebug.LogStatus(msg, nil) },
	})

	transport.SetOnData(func(data string) {
		dataReceived.Store(true)
	})
	transport.SetOnClose(func(closeCode int) {
		// In the real binary, this would drive status machine transitions.
	})
	transport.SetOnConnect(func() {
		transportConnected.Store(true)
		select {
		case connectedCh <- struct{}{}:
		default:
		}
	})
	transport.Connect()
	defer transport.Close()

	// 3. Wait for the transport to connect.
	select {
	case <-connectedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for V2 transport connect")
	}

	// 4. Verify the integration.
	if !initCalled.Load() {
		t.Error("expected /worker/initialize to be called")
	}
	if !transport.IsConnected() {
		t.Error("expected transport to report connected")
	}
	if transport.StateLabel() != "connected" {
		t.Errorf("StateLabel = %q, want %q", transport.StateLabel(), "connected")
	}
	if !transportConnected.Load() {
		t.Error("expected onConnect callback to fire")
	}

	// Give SSE event a moment to arrive.
	deadline := time.After(2 * time.Second)
	for !dataReceived.Load() {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for SSE data event")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// 5. Close and verify teardown.
	transport.Close()
	if transport.IsConnected() {
		t.Error("expected transport to report disconnected after Close()")
	}
}
