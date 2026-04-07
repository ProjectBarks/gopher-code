package bridge

import (
	"encoding/base64"
	"encoding/json"
	"testing"
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

// TestWorkSecretEncryptionRoundTrip exercises the full work secret lifecycle:
// construct WorkSecret → JSON-encode → base64url → DecodeWorkSecret → validate
// fields → BuildSdkUrl / BuildCCRv2SdkUrl. This is the integration path used
// by the binary's remote-control session init (T179).
func TestWorkSecretEncryptionRoundTrip(t *testing.T) {
	// Step 1: Construct a realistic work secret.
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

	// Step 2: Encode to base64url (simulating what the server sends).
	rawJSON, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(rawJSON)

	// Step 3: Decode (as the binary does on receipt).
	decoded, err := DecodeWorkSecret(encoded)
	if err != nil {
		t.Fatalf("DecodeWorkSecret: %v", err)
	}

	// Step 4: Validate all fields survived the round-trip.
	if decoded.Version != original.Version {
		t.Errorf("Version = %d, want %d", decoded.Version, original.Version)
	}
	if decoded.SessionIngressToken != original.SessionIngressToken {
		t.Errorf("SessionIngressToken = %q, want %q", decoded.SessionIngressToken, original.SessionIngressToken)
	}
	if decoded.APIBaseURL != original.APIBaseURL {
		t.Errorf("APIBaseURL = %q, want %q", decoded.APIBaseURL, original.APIBaseURL)
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

	// Step 5: Build SDK URLs from the decoded secret (integration with URL builders).
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

	// Step 6: Verify localhost variant uses ws:// and v2.
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
