package bridge

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// mockBridgeAPI — minimal mock for SessionRunner tests
// ---------------------------------------------------------------------------

type mockBridgeAPI struct {
	mu                sync.Mutex
	heartbeatCalls    int
	heartbeatResp     *HeartbeatResponse
	heartbeatErr      error
	archiveCalls      int
	archiveSessionIDs []string
}

func newMockAPI() *mockBridgeAPI {
	return &mockBridgeAPI{
		heartbeatResp: &HeartbeatResponse{LeaseExtended: true, State: "active"},
	}
}

func (m *mockBridgeAPI) RegisterBridgeEnvironment(_ BridgeConfig) (*RegisterEnvironmentResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBridgeAPI) PollForWork(_, _ string, _ *int) (*WorkResponse, error) {
	return nil, nil
}

func (m *mockBridgeAPI) AcknowledgeWork(_, _, _ string) error { return nil }

func (m *mockBridgeAPI) StopWork(_, _ string, _ bool) error { return nil }

func (m *mockBridgeAPI) DeregisterEnvironment(_ string) error { return nil }

func (m *mockBridgeAPI) SendPermissionResponseEvent(_ string, _ PermissionResponseEvent, _ string) error {
	return nil
}

func (m *mockBridgeAPI) ArchiveSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.archiveCalls++
	m.archiveSessionIDs = append(m.archiveSessionIDs, sessionID)
	return nil
}

func (m *mockBridgeAPI) ReconnectSession(_, _ string) error { return nil }

func (m *mockBridgeAPI) HeartbeatWork(_, _, _ string) (*HeartbeatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeatCalls++
	if m.heartbeatErr != nil {
		return nil, m.heartbeatErr
	}
	return m.heartbeatResp, nil
}

func (m *mockBridgeAPI) getHeartbeatCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.heartbeatCalls
}

func (m *mockBridgeAPI) getArchiveCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.archiveCalls
}

// ---------------------------------------------------------------------------
// validWorkSecret helper — builds a base64url-encoded work secret
// ---------------------------------------------------------------------------

func validWorkResponse() WorkResponse {
	// Build a valid base64url work secret with version=1.
	secretJSON := `{"version":1,"session_ingress_token":"tok_test","api_base_url":"https://api.example.com","sources":[],"auth":[]}`
	encoded := encodeBase64URL(secretJSON)
	return WorkResponse{
		ID:            "work_123",
		Type:          "work",
		EnvironmentID: "env_abc",
		State:         "pending",
		Data: WorkData{
			Type: WorkDataTypeSession,
			ID:   "session_xyz",
		},
		Secret:    encoded,
		CreatedAt: "2025-01-01T00:00:00Z",
	}
}

func encodeBase64URL(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// ---------------------------------------------------------------------------
// Tests — SafeFilenameID
// ---------------------------------------------------------------------------

func TestSafeFilenameID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"abc-123_def", "abc-123_def"},
		{"session/../etc/passwd", "session____etc_passwd"},
		{"foo bar/baz", "foo_bar_baz"},
		{"", ""},
		{"a!@#$%b", "a_____b"},
	}
	for _, tc := range tests {
		got := SafeFilenameID(tc.in)
		if got != tc.want {
			t.Errorf("SafeFilenameID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests — ToolVerbs completeness
// ---------------------------------------------------------------------------

func TestToolVerbsHas18Entries(t *testing.T) {
	// TS source has exactly 18 entries in TOOL_VERBS.
	if got := len(ToolVerbs); got != 18 {
		t.Errorf("ToolVerbs has %d entries, want 18", got)
	}
}

// ---------------------------------------------------------------------------
// Tests — RunnerState.String()
// ---------------------------------------------------------------------------

func TestRunnerStateString(t *testing.T) {
	tests := []struct {
		state RunnerState
		want  string
	}{
		{RunnerIdle, "idle"},
		{RunnerStarting, "starting"},
		{RunnerRunning, "running"},
		{RunnerStopping, "stopping"},
		{RunnerDone, "done"},
		{RunnerState(99), "RunnerState(99)"},
	}
	for _, tc := range tests {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("RunnerState(%d).String() = %q, want %q", int(tc.state), got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests — State machine transitions
// ---------------------------------------------------------------------------

func TestSessionRunner_StartTransitions(t *testing.T) {
	api := newMockAPI()

	var transitions []string
	var mu sync.Mutex

	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 1 * time.Hour, // long so it doesn't fire during test
		OnStateChange: func(from, to RunnerState) {
			mu.Lock()
			transitions = append(transitions, fmt.Sprintf("%s->%s", from, to))
			mu.Unlock()
		},
	})

	if runner.State() != RunnerIdle {
		t.Fatalf("initial state = %s, want idle", runner.State())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := runner.Start(ctx, validWorkResponse())
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if runner.State() != RunnerRunning {
		t.Fatalf("state after Start = %s, want running", runner.State())
	}

	// Verify transitions: idle->starting, starting->running.
	mu.Lock()
	got := transitions
	mu.Unlock()

	if len(got) < 2 {
		t.Fatalf("expected at least 2 transitions, got %d: %v", len(got), got)
	}
	if got[0] != "idle->starting" {
		t.Errorf("transition[0] = %q, want idle->starting", got[0])
	}
	if got[1] != "starting->running" {
		t.Errorf("transition[1] = %q, want starting->running", got[1])
	}

	// Verify metadata.
	if runner.WorkID() != "work_123" {
		t.Errorf("WorkID = %q, want work_123", runner.WorkID())
	}
	if runner.SessionID() != "session_xyz" {
		t.Errorf("SessionID = %q, want session_xyz", runner.SessionID())
	}

	// Cleanup.
	cancel()
	runner.Stop(StopReasonCompleted)
}

func TestSessionRunner_StartFromNonIdleState(t *testing.T) {
	api := newMockAPI()
	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 1 * time.Hour,
	})

	ctx := context.Background()
	// Start once.
	if err := runner.Start(ctx, validWorkResponse()); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	// Start again should fail.
	err := runner.Start(ctx, validWorkResponse())
	if err == nil {
		t.Fatal("second Start should have returned an error")
	}

	runner.Stop(StopReasonCompleted)
}

func TestSessionRunner_StopFromIdle(t *testing.T) {
	api := newMockAPI()

	var transitions []string
	runner := NewSessionRunner(SessionRunnerDeps{
		API:           api,
		EnvironmentID: "env_1",
		OnStateChange: func(from, to RunnerState) {
			transitions = append(transitions, fmt.Sprintf("%s->%s", from, to))
		},
	})

	runner.Stop(StopReasonInterrupted)

	if runner.State() != RunnerDone {
		t.Fatalf("state = %s, want done", runner.State())
	}
	if runner.StopReasonValue() != StopReasonInterrupted {
		t.Errorf("stop reason = %q, want interrupted", runner.StopReasonValue())
	}

	// done channel should be closed.
	select {
	case <-runner.Done():
	default:
		t.Fatal("Done channel not closed")
	}

	if len(transitions) != 1 || transitions[0] != "idle->done" {
		t.Errorf("transitions = %v, want [idle->done]", transitions)
	}
}

func TestSessionRunner_StopFromRunning(t *testing.T) {
	api := newMockAPI()
	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 1 * time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runner.Start(ctx, validWorkResponse()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	runner.Stop(StopReasonCompleted)

	if runner.State() != RunnerDone {
		t.Fatalf("state = %s, want done", runner.State())
	}
	if runner.StopReasonValue() != StopReasonCompleted {
		t.Errorf("stop reason = %q, want completed", runner.StopReasonValue())
	}

	// Archive should have been called.
	if api.getArchiveCalls() != 1 {
		t.Errorf("archive calls = %d, want 1", api.getArchiveCalls())
	}
}

func TestSessionRunner_StopIdempotent(t *testing.T) {
	api := newMockAPI()
	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 1 * time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runner.Start(ctx, validWorkResponse()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	runner.Stop(StopReasonCompleted)
	runner.Stop(StopReasonFailed) // second call should be no-op

	if runner.StopReasonValue() != StopReasonCompleted {
		t.Errorf("stop reason = %q, want completed (first stop wins)", runner.StopReasonValue())
	}
}

// ---------------------------------------------------------------------------
// Tests — Heartbeat scheduling
// ---------------------------------------------------------------------------

func TestSessionRunner_HeartbeatFires(t *testing.T) {
	api := newMockAPI()
	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 25 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runner.Start(ctx, validWorkResponse()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for a few heartbeats.
	time.Sleep(120 * time.Millisecond)

	calls := api.getHeartbeatCalls()
	if calls < 2 {
		t.Errorf("heartbeat calls = %d after 120ms with 25ms interval, want >= 2", calls)
	}

	runner.Stop(StopReasonCompleted)
}

func TestSessionRunner_HeartbeatRecoverableError(t *testing.T) {
	api := newMockAPI()
	api.heartbeatErr = fmt.Errorf("network blip")

	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 20 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runner.Start(ctx, validWorkResponse()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for a couple of retries.
	time.Sleep(80 * time.Millisecond)

	// Runner should still be running (recoverable errors don't stop it).
	if runner.State() != RunnerRunning {
		t.Errorf("state = %s, want running (recoverable error should not stop)", runner.State())
	}

	calls := api.getHeartbeatCalls()
	if calls < 2 {
		t.Errorf("heartbeat retried %d times, want >= 2", calls)
	}

	runner.Stop(StopReasonCompleted)
}

func TestSessionRunner_HeartbeatLeaseExpiry(t *testing.T) {
	api := newMockAPI()
	api.heartbeatResp = &HeartbeatResponse{LeaseExtended: false, State: "expired"}

	var fatalErr error
	var fatalMu sync.Mutex

	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 20 * time.Millisecond,
		OnFatalError: func(err error) {
			fatalMu.Lock()
			fatalErr = err
			fatalMu.Unlock()
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runner.Start(ctx, validWorkResponse()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for the heartbeat to detect lease expiry and stop.
	select {
	case <-runner.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not reach done state after lease expiry")
	}

	if runner.State() != RunnerDone {
		t.Errorf("state = %s, want done", runner.State())
	}
	if runner.StopReasonValue() != StopReasonLeaseExpiry {
		t.Errorf("stop reason = %q, want lease_expired", runner.StopReasonValue())
	}

	fatalMu.Lock()
	if fatalErr == nil {
		t.Error("OnFatalError was not called")
	}
	fatalMu.Unlock()
}

// ---------------------------------------------------------------------------
// Tests — Start with invalid work secret
// ---------------------------------------------------------------------------

func TestSessionRunner_StartInvalidSecret(t *testing.T) {
	api := newMockAPI()
	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 1 * time.Hour,
	})

	work := validWorkResponse()
	work.Secret = "not-valid-base64!!!"

	err := runner.Start(context.Background(), work)
	if err == nil {
		t.Fatal("Start should have returned an error for invalid secret")
	}

	if runner.State() != RunnerDone {
		t.Errorf("state = %s, want done (failed on bad secret)", runner.State())
	}
	if runner.StopReasonValue() != StopReasonFailed {
		t.Errorf("stop reason = %q, want failed", runner.StopReasonValue())
	}
}

// ---------------------------------------------------------------------------
// Tests — Graceful stop archives session
// ---------------------------------------------------------------------------

func TestSessionRunner_StopArchivesSession(t *testing.T) {
	api := newMockAPI()
	runner := NewSessionRunner(SessionRunnerDeps{
		API:               api,
		EnvironmentID:     "env_1",
		HeartbeatInterval: 1 * time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := runner.Start(ctx, validWorkResponse()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	runner.Stop(StopReasonCompleted)

	api.mu.Lock()
	defer api.mu.Unlock()

	if len(api.archiveSessionIDs) != 1 {
		t.Fatalf("expected 1 archive call, got %d", len(api.archiveSessionIDs))
	}
	if api.archiveSessionIDs[0] != "session_xyz" {
		t.Errorf("archived session = %q, want session_xyz", api.archiveSessionIDs[0])
	}
}

// ---------------------------------------------------------------------------
// Tests — Done channel closed on stop
// ---------------------------------------------------------------------------

func TestSessionRunner_DoneChannelClosed(t *testing.T) {
	api := newMockAPI()
	runner := NewSessionRunner(SessionRunnerDeps{
		API:           api,
		EnvironmentID: "env_1",
	})

	// Before stop, channel should be open.
	select {
	case <-runner.Done():
		t.Fatal("Done channel closed before Stop")
	default:
	}

	runner.Stop(StopReasonInterrupted)

	select {
	case <-runner.Done():
		// Good.
	case <-time.After(time.Second):
		t.Fatal("Done channel not closed after Stop")
	}
}
