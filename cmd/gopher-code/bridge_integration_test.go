package main

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/auth"
	"github.com/projectbarks/gopher-code/pkg/bridge"
)

// TestBridgeConfig_IntegrationDefaults verifies that the bridge config helpers
// are reachable from the binary and return sane defaults when no env overrides
// are set (the common case for non-Anthropic users).
func TestBridgeConfig_IntegrationDefaults(t *testing.T) {
	// Build the same ConfigDeps that main.go wires for remote-control.
	deps := bridge.ConfigDeps{
		GetAccessToken: func() (string, bool) {
			key, err := auth.GetAPIKey()
			if err != nil {
				return "", false
			}
			return key, true
		},
		GetBaseAPIURL: func() string { return "https://api.anthropic.com" },
	}

	// Without ant env overrides, BridgeBaseURL should return the production default.
	u := bridge.BridgeBaseURL(deps)
	if u != "https://api.anthropic.com" {
		t.Fatalf("BridgeBaseURL = %q, want production default", u)
	}

	// Without any token source, BridgeAccessToken should return not-authed.
	// Just verify it doesn't panic.
	_, _ = bridge.BridgeAccessToken(deps)
}

// TestBridgeConfig_EnvOverride verifies that ant-only env overrides flow
// through the same ConfigDeps path used in the binary.
func TestBridgeConfig_EnvOverride(t *testing.T) {
	t.Setenv("USER_TYPE", "ant")
	t.Setenv("CLAUDE_BRIDGE_BASE_URL", "https://dev.example.com")
	t.Setenv("CLAUDE_BRIDGE_OAUTH_TOKEN", "test-token-xyz")

	deps := bridge.ConfigDeps{
		GetAccessToken: func() (string, bool) { return "fallback", true },
		GetBaseAPIURL:  func() string { return "https://api.anthropic.com" },
	}

	// Env override should win over deps.
	u := bridge.BridgeBaseURL(deps)
	if u != "https://dev.example.com" {
		t.Fatalf("BridgeBaseURL = %q, want env override", u)
	}

	tok, ok := bridge.BridgeAccessToken(deps)
	if !ok {
		t.Fatal("expected bridge access token to be available")
	}
	if tok != "test-token-xyz" {
		t.Fatalf("BridgeAccessToken = %q, want env override token", tok)
	}
}

// TestInitBridgeDeps_SetsDefaultDeps verifies that initBridgeDeps registers a
// non-nil BridgeDeps bundle with safe defaults (bridge disabled, stubs wired).
func TestInitBridgeDeps_SetsDefaultDeps(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	if bridge.IsBridgeEnabled() {
		t.Fatal("expected IsBridgeEnabled=false after initBridgeDeps (default build has BridgeMode=false)")
	}

	reason, err := bridge.GetBridgeDisabledReason()
	if err != nil {
		t.Fatalf("unexpected error from GetBridgeDisabledReason: %v", err)
	}
	if reason != bridge.ErrBridgeNotAvailable {
		t.Fatalf("expected %q, got %q", bridge.ErrBridgeNotAvailable, reason)
	}
}

// TestInitBridgeDeps_VersionWired verifies that the Version field in BridgeDeps
// is set to the binary's Version constant.
func TestInitBridgeDeps_VersionWired(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	if msg := bridge.CheckBridgeMinVersion(); msg != "" {
		t.Fatalf("expected empty min-version check, got: %q", msg)
	}
}

// TestRemoteControl_BlockedByDefault verifies that the remote-control subcommand
// is blocked when bridge is not enabled and not forced.
func TestRemoteControl_BlockedByDefault(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()
	t.Setenv("CLAUDE_CODE_BRIDGE_FORCED", "")

	code, stderrOut, _ := withExitCapture(func() {
		reason, err := bridge.GetBridgeDisabledReason()
		if err != nil {
			cliErrorf("Error checking Remote Control eligibility: %v", err)
			return
		}
		if reason == "" {
			reason = bridge.ErrBridgeNotAvailable
		}
		cliError(reason)
	})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderrOut, "Remote Control is not available") {
		t.Fatalf("expected 'Remote Control is not available' in stderr, got: %q", stderrOut)
	}
}

// TestRemoteControl_ForcedByEnvVar verifies that CLAUDE_CODE_BRIDGE_FORCED=1
// bypasses the bridge enablement check.
func TestRemoteControl_ForcedByEnvVar(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()
	t.Setenv("CLAUDE_CODE_BRIDGE_FORCED", "1")

	if !bridge.IsBridgeForced() {
		t.Fatal("expected IsBridgeForced=true when CLAUDE_CODE_BRIDGE_FORCED=1")
	}
	if !bridge.IsBridgeEnabled() && !bridge.IsBridgeForced() {
		t.Fatal("expected bridge guard to pass when forced")
	}
}

// TestBridgeFeatureGates_CcrAutoConnect verifies that CCR auto-connect
// defaults to false in the default build.
func TestBridgeFeatureGates_CcrAutoConnect(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	if bridge.GetCcrAutoConnectDefault() {
		t.Fatal("expected GetCcrAutoConnectDefault=false in default build")
	}
}

// TestBridgeFeatureGates_CcrMirror verifies that CCR mirror mode
// defaults to false in the default build.
func TestBridgeFeatureGates_CcrMirror(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	if bridge.IsCcrMirrorEnabled() {
		t.Fatal("expected IsCcrMirrorEnabled=false in default build")
	}
}

// TestHybridTransport_SelectedByEnv verifies HybridTransport selection.
func TestHybridTransport_SelectedByEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "1")

	sel, err := bridge.GetTransportForUrl(
		"wss://api.example.com/v2/session_ingress/ws/sess-abc",
		map[string]string{"Authorization": "Bearer test-tok"},
		"sess-abc",
	)
	if err != nil {
		t.Fatalf("GetTransportForUrl error: %v", err)
	}
	if sel.Kind != bridge.TransportKindHybrid {
		t.Fatalf("expected TransportKindHybrid, got %d", sel.Kind)
	}
	hybrid := bridge.NewHybridTransport(bridge.HybridTransportOpts{
		URL:          sel.URL,
		Headers:      sel.Headers,
		SessionID:    sel.SessionID,
		GetAuthToken: func() string { return "test-tok" },
	})
	transport := bridge.NewV1ReplTransport(hybrid)
	if !transport.IsConnected() {
		t.Fatal("expected IsConnected=true before Close()")
	}
	transport.Close()
	if transport.IsConnected() {
		t.Fatal("expected not connected after Close()")
	}
}

// TestHybridTransport_DefaultSelectsV2 verifies default is not Hybrid.
func TestHybridTransport_DefaultSelectsV2(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "")

	sel, err := bridge.GetTransportForUrl(
		"wss://api.example.com/v2/session_ingress/ws/sess-abc",
		nil,
		"sess-abc",
	)
	if err != nil {
		t.Fatalf("GetTransportForUrl error: %v", err)
	}
	if sel.Kind == bridge.TransportKindHybrid {
		t.Fatal("expected non-Hybrid transport when env var is not set")
	}
}

// TestCCRClient_IntegrationConstruct verifies CCRClient construction.
func TestCCRClient_IntegrationConstruct(t *testing.T) {
	sessionURL := "https://api.example.com/v1/code/sessions/test-session-id"
	client, err := bridge.NewCCRClient(sessionURL, bridge.CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer test-token"}
		},
		UserAgent:       "gopher-code/test",
		OnEpochMismatch: func() { t.Log("epoch mismatch") },
	})
	if err != nil {
		t.Fatalf("NewCCRClient failed: %v", err)
	}
	defer client.Close()
	if epoch := client.WorkerEpoch(); epoch != 0 {
		t.Fatalf("expected initial epoch 0, got %d", epoch)
	}
	client.WriteEvent(map[string]any{"type": "test"})
	client.WriteInternalEvent("transcript", map[string]any{"data": "x"}, false, "")
	if n := client.PendingEventCount(); n != 1 {
		t.Fatalf("expected 1 pending event, got %d", n)
	}
	client.Close()
	client.Close()
}

// TestSerialBatchUploader_MessagingIntegration verifies uploader + messaging wiring.
func TestSerialBatchUploader_MessagingIntegration(t *testing.T) {
	var uploaded atomic.Int64
	uploader := bridge.NewSerialBatchEventUploader(bridge.SerialBatchUploaderConfig[bridge.BridgeEvent]{
		MaxBatchSize:           10,
		MaxQueueSize:           100,
		MaxConsecutiveFailures: 3,
		BaseDelay:              time.Millisecond,
		MaxDelay:               10 * time.Millisecond,
		Jitter:                 time.Millisecond,
		Send: func(_ context.Context, batch []bridge.BridgeEvent) error {
			uploaded.Add(int64(len(batch)))
			return nil
		},
	})
	defer uploader.Close()

	messaging := bridge.NewBridgeMessaging(bridge.BridgeMessagingConfig{
		Send: func(ctx context.Context, batch []bridge.BridgeEvent) error {
			uploader.Enqueue(batch...)
			return nil
		},
	})
	defer messaging.Close()

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := messaging.Enqueue(ctx, bridge.BridgeEvent{Type: "test"}); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}
	if err := messaging.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
	uploader.Flush()
	if n := uploaded.Load(); n != 5 {
		t.Fatalf("expected 5 events uploaded, got %d", n)
	}
}

// TestWebSocketTransport_SelectedByDefault verifies WebSocketTransport selection
// when no CCR v2 or hybrid env vars are set.
func TestWebSocketTransport_SelectedByDefault(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_CCR_V2", "")
	t.Setenv("CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2", "")

	sel, err := bridge.GetTransportForUrl(
		"wss://api.example.com/v2/session_ingress/ws/sess-abc",
		map[string]string{"Authorization": "Bearer test-tok"},
		"sess-abc",
	)
	if err != nil {
		t.Fatalf("GetTransportForUrl error: %v", err)
	}
	if sel.Kind != bridge.TransportKindWebSocket {
		t.Fatalf("expected TransportKindWebSocket, got %d", sel.Kind)
	}

	// Construct via the same path as main.go — NewWebSocketTransport + V1 adapter.
	ws := bridge.NewWebSocketTransport(bridge.WebSocketTransportOpts{
		URL:       sel.URL,
		Headers:   sel.Headers,
		SessionID: sel.SessionID,
		IsBridge:  true,
		AutoReconnect: func() *bool { b := false; return &b }(),
	})
	transport := bridge.NewV1ReplTransport(ws)

	// Before Connect: state is idle, not connected.
	if transport.IsConnected() {
		t.Fatal("expected IsConnected=false before Connect()")
	}
	if lab := transport.StateLabel(); lab != "idle" {
		t.Fatalf("expected state label 'idle', got %q", lab)
	}

	// DroppedBatchCount should be 0.
	if cnt := transport.DroppedBatchCount(); cnt != 0 {
		t.Fatalf("expected DroppedBatchCount=0, got %d", cnt)
	}

	// Close should be safe even without connecting.
	transport.Close()
	if lab := transport.StateLabel(); lab != "closed" {
		t.Fatalf("expected state label 'closed' after Close(), got %q", lab)
	}
}
