package bridge

import (
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
