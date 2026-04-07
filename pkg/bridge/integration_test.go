package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Integration tests — exercise bridge types through real code paths
// (config construction → JSON serialization → API client registration).
// ---------------------------------------------------------------------------

// TestNewRemoteControlConfigIntegration verifies that NewRemoteControlConfig
// produces a BridgeConfig that round-trips through JSON and is accepted by
// RegisterBridgeEnvironment (via mock). This exercises the full path from
// CLI entry-point config construction through the API client interface.
func TestNewRemoteControlConfigIntegration(t *testing.T) {
	cfg := NewRemoteControlConfig("/tmp/project", "test-machine")

	// Validate defaults set by the constructor.
	if cfg.Dir != "/tmp/project" {
		t.Fatalf("Dir = %q, want /tmp/project", cfg.Dir)
	}
	if cfg.MachineName != "test-machine" {
		t.Fatalf("MachineName = %q, want test-machine", cfg.MachineName)
	}
	if cfg.MaxSessions != 1 {
		t.Fatalf("MaxSessions = %d, want 1", cfg.MaxSessions)
	}
	if cfg.SpawnMode != SpawnModeSingleSession {
		t.Fatalf("SpawnMode = %q, want %q", cfg.SpawnMode, SpawnModeSingleSession)
	}
	if cfg.WorkerType != string(WorkerTypeClaudeCode) {
		t.Fatalf("WorkerType = %q, want %q", cfg.WorkerType, WorkerTypeClaudeCode)
	}
	if cfg.SessionTimeoutMS == nil || *cfg.SessionTimeoutMS != DefaultSessionTimeoutMS {
		t.Fatalf("SessionTimeoutMS = %v, want %d", cfg.SessionTimeoutMS, DefaultSessionTimeoutMS)
	}

	// JSON round-trip — simulates wire serialization used by the bridge API.
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded BridgeConfig
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if decoded.Dir != cfg.Dir || decoded.SpawnMode != cfg.SpawnMode ||
		decoded.MaxSessions != cfg.MaxSessions || decoded.WorkerType != cfg.WorkerType {
		t.Fatalf("round-trip mismatch: got %+v", decoded)
	}

	// Feed the config into the mock API client — exercises the BridgeAPIClient
	// interface with a config produced by the real constructor.
	mock := &mockAPIClient{
		registerResp: &RegisterEnvironmentResponse{
			EnvironmentID:     "env-integration-1",
			EnvironmentSecret: "secret-integration-1",
		},
	}
	resp, err := mock.RegisterBridgeEnvironment(cfg)
	if err != nil {
		t.Fatalf("RegisterBridgeEnvironment: %v", err)
	}
	if resp.EnvironmentID != "env-integration-1" {
		t.Fatalf("EnvironmentID = %q, want env-integration-1", resp.EnvironmentID)
	}
}

// TestPollConfigIntegration exercises the poll config defaults through the
// DynamicPollConfig path that the binary uses (T175). It verifies that
// DefaultPollConfig flows through NewDynamicPollConfig and produces the
// expected poll delays at each capacity state.
func TestPollConfigIntegration(t *testing.T) {
	// Single-session mode (matches the binary's remote-control init).
	dpc := NewDynamicPollConfig(DefaultPollConfig, false)

	// At CapacityNone the delay should equal the not-at-capacity interval.
	if got := dpc.NextPollDelay(); got != DefaultPollConfig.PollIntervalNotAtCapacity {
		t.Fatalf("NextPollDelay(CapacityNone) = %v, want %v", got, DefaultPollConfig.PollIntervalNotAtCapacity)
	}

	// Switch to full capacity — should return at-capacity interval.
	dpc.SetCapacity(CapacityFull)
	if got := dpc.NextPollDelay(); got != DefaultPollConfig.PollIntervalAtCapacity {
		t.Fatalf("NextPollDelay(CapacityFull) = %v, want %v", got, DefaultPollConfig.PollIntervalAtCapacity)
	}

	// Config round-trips through JSON (simulates wire format from server).
	raw, err := json.Marshal(DefaultPollConfig)
	if err != nil {
		t.Fatalf("Marshal DefaultPollConfig: %v", err)
	}
	var decoded PollIntervalConfig
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal DefaultPollConfig: %v", err)
	}
	if decoded.PollIntervalNotAtCapacity != DefaultPollConfig.PollIntervalNotAtCapacity {
		t.Fatalf("round-trip PollIntervalNotAtCapacity = %v, want %v",
			decoded.PollIntervalNotAtCapacity, DefaultPollConfig.PollIntervalNotAtCapacity)
	}

	// Hot-swap config via SetConfig (simulates GrowthBook refresh).
	dpc.SetConfig(decoded)
	dpc.SetCapacity(CapacityNone)
	if got := dpc.NextPollDelay(); got != DefaultPollConfig.PollIntervalNotAtCapacity {
		t.Fatalf("NextPollDelay after SetConfig = %v, want %v", got, DefaultPollConfig.PollIntervalNotAtCapacity)
	}
}

// TestBridgeConfigToPollWorkflow exercises the full config → register →
// poll-for-work → acknowledge workflow with bridge types flowing through
// each step.
func TestBridgeConfigToPollWorkflow(t *testing.T) {
	// Step 1: Build config (as the binary does).
	cfg := NewRemoteControlConfig("/home/user/project", "ci-runner")
	cfg.APIBaseURL = "https://api.test.example.com"

	// Step 2: Register environment.
	mock := &mockAPIClient{
		registerResp: &RegisterEnvironmentResponse{
			EnvironmentID:     "env-wf-1",
			EnvironmentSecret: "secret-wf-1",
		},
		pollResults: []*WorkResponse{
			{
				ID:            "work-1",
				Type:          "work",
				EnvironmentID: "env-wf-1",
				State:         "pending",
				Data: WorkData{
					Type: WorkDataTypeSession,
					ID:   "sess-1",
				},
				CreatedAt: "2025-06-01T00:00:00Z",
			},
		},
	}

	regResp, err := mock.RegisterBridgeEnvironment(cfg)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Step 3: Poll for work.
	work, err := mock.PollForWork(regResp.EnvironmentID, regResp.EnvironmentSecret, nil)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if work.Data.Type != WorkDataTypeSession {
		t.Fatalf("work data type = %q, want %q", work.Data.Type, WorkDataTypeSession)
	}

	// Step 4: Acknowledge work.
	if err := mock.AcknowledgeWork(regResp.EnvironmentID, work.ID, "token-1"); err != nil {
		t.Fatalf("ack: %v", err)
	}

	// Verify the ack was recorded.
	if len(mock.ackCalls) != 1 || mock.ackCalls[0] != "work-1" {
		t.Fatalf("ack calls = %v, want [work-1]", mock.ackCalls)
	}

	// Step 5: Report session activity — exercises SessionActivity struct.
	activity := SessionActivity{
		Type:      ActivityToolStart,
		Summary:   "Reading file",
		Timestamp: 1700000000000,
	}
	actJSON, err := json.Marshal(activity)
	if err != nil {
		t.Fatalf("marshal activity: %v", err)
	}
	var decoded SessionActivity
	if err := json.Unmarshal(actJSON, &decoded); err != nil {
		t.Fatalf("unmarshal activity: %v", err)
	}
	if decoded.Type != ActivityToolStart || decoded.Summary != "Reading file" {
		t.Fatalf("activity round-trip failed: %+v", decoded)
	}
}

// TestTrustedDevice_LoadSaveValidateCycle exercises the full lifecycle of a
// trusted device token: enroll (save) ��� load from cache → clear → re-load
// from keyring → validate header injection.
func TestTrustedDevice_LoadSaveValidateCycle(t *testing.T) {
	kr := newMemKeyring()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(enrollmentResponse{
			DeviceToken: "integration-token-xyz",
			DeviceID:    "device-int-1",
		})
	}))
	defer srv.Close()

	deps := TrustedDeviceDeps{
		Keyring:                kr,
		GetFeatureValueBool:    gateOn,
		CheckGateBlocking:      blockingGateOn,
		GetAccessToken:         func() (string, bool) { return "oauth-test", true },
		GetBaseAPIURL:          func() string { return srv.URL },
		IsEssentialTrafficOnly: func() bool { return false },
		Hostname:               "integration-host",
		HTTPClient:             srv.Client(),
	}
	mgr := NewTrustedDeviceManager(deps)

	if tok := mgr.GetToken(); tok != "" {
		t.Fatalf("expected empty token before enrollment, got %q", tok)
	}
	mgr.Enroll(context.Background())
	tok := mgr.GetToken()
	if tok != "integration-token-xyz" {
		t.Fatalf("post-enroll GetToken: got %q, want %q", tok, "integration-token-xyz")
	}
	hdrKey, hdrVal := mgr.Header()
	if hdrKey != "X-Trusted-Device-Token" || hdrVal != "integration-token-xyz" {
		t.Fatalf("Header: got (%q, %q)", hdrKey, hdrVal)
	}
	mgr.ClearCache()
	tok = mgr.GetToken()
	if tok != "integration-token-xyz" {
		t.Fatalf("after ClearCache: got %q", tok)
	}
	mgr.ClearToken()
	tok = mgr.GetToken()
	if tok != "" {
		t.Fatalf("after ClearToken: expected empty, got %q", tok)
	}
	_, err := kr.Get(keyringService, keyringUser)
	if err == nil {
		t.Fatal("expected keyring to be empty after ClearToken")
	}
}

// TestWorkSecretEncryptionRoundTrip exercises the full work secret lifecycle.
func TestWorkSecretEncryptionRoundTrip(t *testing.T) {
	original := WorkSecret{
		Version:             1,
		SessionIngressToken: "tok_integration_abc123",
		APIBaseURL:          "https://api.anthropic.com",
		Sources: []WorkSecretSource{
			{Type: "git", GitInfo: &GitInfo{Type: "github", Repo: "org/repo", Token: "ghp_test"}},
		},
		Auth: []WorkSecretAuth{
			{Type: "api_key", Token: "sk-ant-integration"},
		},
		EnvironmentVariables: map[string]string{"FOO": "bar"},
	}

	rawJSON, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(rawJSON)

	decoded, err := DecodeWorkSecret(encoded)
	if err != nil {
		t.Fatalf("DecodeWorkSecret: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("Version = %d, want %d", decoded.Version, original.Version)
	}
	if decoded.SessionIngressToken != original.SessionIngressToken {
		t.Errorf("SessionIngressToken mismatch")
	}
	if decoded.APIBaseURL != original.APIBaseURL {
		t.Errorf("APIBaseURL mismatch")
	}
	if len(decoded.Sources) != 1 || decoded.Sources[0].GitInfo.Token != "ghp_test" {
		t.Error("git source credential not preserved")
	}
	if len(decoded.Auth) != 1 || decoded.Auth[0].Token != "sk-ant-integration" {
		t.Error("auth token not preserved")
	}
	if decoded.EnvironmentVariables["FOO"] != "bar" {
		t.Error("environment variables not preserved")
	}

	wsURL := BuildSdkUrl(decoded.APIBaseURL, "sess_test123")
	wantWS := "wss://api.anthropic.com/v1/session_ingress/ws/sess_test123"
	if wsURL != wantWS {
		t.Errorf("BuildSdkUrl = %q, want %q", wsURL, wantWS)
	}
	ccrURL := BuildCCRv2SdkUrl(decoded.APIBaseURL, "sess_test123")
	wantCCR := "https://api.anthropic.com/v1/code/sessions/sess_test123"
	if ccrURL != wantCCR {
		t.Errorf("BuildCCRv2SdkUrl = %q, want %q", ccrURL, wantCCR)
	}

	localSecret := original
	localSecret.APIBaseURL = "http://localhost:8080"
	localJSON, _ := json.Marshal(localSecret)
	localEncoded := base64.RawURLEncoding.EncodeToString(localJSON)
	localDecoded, err := DecodeWorkSecret(localEncoded)
	if err != nil {
		t.Fatalf("DecodeWorkSecret (localhost): %v", err)
	}
	localURL := BuildSdkUrl(localDecoded.APIBaseURL, "sess_local")
	wantLocal := "ws://localhost:8080/v2/session_ingress/ws/sess_local"
	if localURL != wantLocal {
		t.Errorf("BuildSdkUrl (localhost) = %q, want %q", localURL, wantLocal)
	}
}

// TestBridgeMessagingIntegration exercises message construction, JSON
// serialization, enqueue, flush, and ack — the full outbound event lifecycle
// that the binary's remote-control path relies on (T184).
func TestBridgeMessagingIntegration(t *testing.T) {
	var received [][]BridgeEvent
	var mu sync.Mutex

	bm := NewBridgeMessaging(BridgeMessagingConfig{
		MaxBatchSize: 10,
		MaxQueueSize: 100,
		AckTimeout:   5 * time.Second,
		Send: func(_ context.Context, batch []BridgeEvent) error {
			mu.Lock()
			cp := make([]BridgeEvent, len(batch))
			copy(cp, batch)
			received = append(received, cp)
			mu.Unlock()
			return nil
		},
	})
	defer bm.Close()

	ctx := context.Background()

	// Step 1: Construct events with typed payloads and serialize to JSON.
	type userPayload struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Content   string `json:"content"`
	}
	type assistantPayload struct {
		Type    string `json:"type"`
		Content string `json:"content"`
		Model   string `json:"model"`
	}

	payloads := []interface{}{
		userPayload{Type: "user", SessionID: "sess-int-1", Content: "hello"},
		assistantPayload{Type: "assistant", Content: "world", Model: "claude-opus-4-6"},
		userPayload{Type: "user", SessionID: "sess-int-1", Content: "follow-up"},
	}

	for _, p := range payloads {
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("json.Marshal payload: %v", err)
		}
		evt := BridgeEvent{
			Type:      "user",
			SessionID: "sess-int-1",
			Payload:   raw,
		}
		if err := bm.Enqueue(ctx, evt); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	// Step 2: Flush all events.
	if err := bm.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Step 3: Verify all events were sent with monotonic sequence numbers.
	mu.Lock()
	var allEvents []BridgeEvent
	for _, batch := range received {
		allEvents = append(allEvents, batch...)
	}
	mu.Unlock()

	if len(allEvents) != 3 {
		t.Fatalf("expected 3 events, got %d", len(allEvents))
	}
	for i := 1; i < len(allEvents); i++ {
		if allEvents[i].SeqNum <= allEvents[i-1].SeqNum {
			t.Errorf("seq[%d]=%d not > seq[%d]=%d", i, allEvents[i].SeqNum, i-1, allEvents[i-1].SeqNum)
		}
	}

	// Step 4: Verify each event's Payload round-trips through JSON.
	for i, evt := range allEvents {
		// The BridgeEvent itself should serialize cleanly.
		evtJSON, err := json.Marshal(evt)
		if err != nil {
			t.Fatalf("json.Marshal event[%d]: %v", i, err)
		}
		var decoded BridgeEvent
		if err := json.Unmarshal(evtJSON, &decoded); err != nil {
			t.Fatalf("json.Unmarshal event[%d]: %v", i, err)
		}
		if decoded.SeqNum != evt.SeqNum {
			t.Errorf("event[%d] SeqNum mismatch: got %d, want %d", i, decoded.SeqNum, evt.SeqNum)
		}
		if decoded.SessionID != evt.SessionID {
			t.Errorf("event[%d] SessionID mismatch: got %q, want %q", i, decoded.SessionID, evt.SessionID)
		}

		// Payload JSON should be valid and decodable.
		var raw map[string]interface{}
		if err := json.Unmarshal(decoded.Payload, &raw); err != nil {
			t.Fatalf("event[%d] payload unmarshal: %v", i, err)
		}
		if _, ok := raw["type"]; !ok {
			t.Errorf("event[%d] payload missing 'type' field", i)
		}
	}

	// Step 5: Acknowledge all events.
	for _, evt := range allEvents {
		if !bm.Acknowledge(evt.SeqNum) {
			t.Errorf("Acknowledge(%d) returned false", evt.SeqNum)
		}
	}
	if bm.PendingAckCount() != 0 {
		t.Errorf("expected 0 pending acks after acknowledging all, got %d", bm.PendingAckCount())
	}
}
