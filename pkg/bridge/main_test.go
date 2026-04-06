package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

// mockAPIClient implements BridgeAPIClient for testing.
type mockAPIClient struct {
	mu                   sync.Mutex
	registerResp         *RegisterEnvironmentResponse
	registerErr          error
	pollResults          []*WorkResponse // returned in order; cycles on last
	pollErrs             []error
	pollIndex            int
	ackCalls             []string // workIDs
	stopCalls            []string // workIDs
	deregisterCalled     atomic.Bool
	heartbeatResp        *HeartbeatResponse
	heartbeatErr         error
	reconnectErr         error
	archiveCalls         []string
	sendPermissionErr    error
}

func (m *mockAPIClient) RegisterBridgeEnvironment(_ BridgeConfig) (*RegisterEnvironmentResponse, error) {
	if m.registerErr != nil {
		return nil, m.registerErr
	}
	if m.registerResp != nil {
		return m.registerResp, nil
	}
	return &RegisterEnvironmentResponse{EnvironmentID: "env-123", EnvironmentSecret: "secret-abc"}, nil
}

func (m *mockAPIClient) PollForWork(_ string, _ string, _ *int) (*WorkResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := m.pollIndex
	if idx < len(m.pollErrs) && m.pollErrs[idx] != nil {
		if idx < len(m.pollErrs)-1 {
			m.pollIndex++
		}
		return nil, m.pollErrs[idx]
	}
	if idx < len(m.pollResults) {
		m.pollIndex++
		return m.pollResults[idx], nil
	}
	// No more results — return nil (no work).
	return nil, nil
}

func (m *mockAPIClient) AcknowledgeWork(_ string, workID string, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ackCalls = append(m.ackCalls, workID)
	return nil
}

func (m *mockAPIClient) StopWork(_ string, workID string, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalls = append(m.stopCalls, workID)
	return nil
}

func (m *mockAPIClient) DeregisterEnvironment(_ string) error {
	m.deregisterCalled.Store(true)
	return nil
}

func (m *mockAPIClient) SendPermissionResponseEvent(_ string, _ PermissionResponseEvent, _ string) error {
	return m.sendPermissionErr
}

func (m *mockAPIClient) ArchiveSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.archiveCalls = append(m.archiveCalls, sessionID)
	return nil
}

func (m *mockAPIClient) ReconnectSession(_ string, _ string) error {
	return m.reconnectErr
}

func (m *mockAPIClient) HeartbeatWork(_ string, _ string, _ string) (*HeartbeatResponse, error) {
	if m.heartbeatErr != nil {
		return nil, m.heartbeatErr
	}
	if m.heartbeatResp != nil {
		return m.heartbeatResp, nil
	}
	return &HeartbeatResponse{LeaseExtended: true, State: "active"}, nil
}

// mockLogger implements BridgeLogger for testing.
type mockLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *mockLogger) record(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, msg)
}

func (l *mockLogger) Messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]string, len(l.messages))
	copy(cp, l.messages)
	return cp
}

func (l *mockLogger) PrintBanner(_ BridgeConfig, _ string)  { l.record("banner") }
func (l *mockLogger) LogSessionStart(id string, _ string)   { l.record("session_start:" + id) }
func (l *mockLogger) LogSessionComplete(id string, _ time.Duration) {
	l.record("session_complete:" + id)
}
func (l *mockLogger) LogSessionFailed(id string, err string) {
	l.record("session_failed:" + id + ":" + err)
}
func (l *mockLogger) LogStatus(msg string)                 { l.record("status:" + msg) }
func (l *mockLogger) LogVerbose(msg string)                { l.record("verbose:" + msg) }
func (l *mockLogger) LogError(msg string)                  { l.record("error:" + msg) }
func (l *mockLogger) LogReconnected(_ time.Duration)       { l.record("reconnected") }
func (l *mockLogger) UpdateIdleStatus()                    {}
func (l *mockLogger) UpdateReconnectingStatus(_, _ string) { l.record("reconnecting") }
func (l *mockLogger) UpdateSessionStatus(_ string, _ string, _ SessionActivity, _ []string) {}
func (l *mockLogger) ClearStatus()                         {}
func (l *mockLogger) SetRepoInfo(_, _ string)              {}
func (l *mockLogger) SetDebugLogPath(_ string)             {}
func (l *mockLogger) SetAttached(_ string)                 {}
func (l *mockLogger) UpdateFailedStatus(_ string)          {}
func (l *mockLogger) ToggleQR()                            {}
func (l *mockLogger) UpdateSessionCount(_ int, _ int, _ SpawnMode) {}
func (l *mockLogger) SetSpawnModeDisplay(_ *SpawnMode)     {}
func (l *mockLogger) AddSession(_, _ string)               {}
func (l *mockLogger) UpdateSessionActivity(_ string, _ SessionActivity) {}
func (l *mockLogger) SetSessionTitle(_, _ string)          {}
func (l *mockLogger) RemoveSession(_ string)               {}
func (l *mockLogger) RefreshDisplay()                      {}

// mockSpawner implements SessionSpawner for testing.
type mockSpawner struct {
	mu      sync.Mutex
	calls   []string
	handler func(opts SessionSpawnOpts, dir string) *SessionHandle
}

func (s *mockSpawner) Spawn(opts SessionSpawnOpts, dir string) *SessionHandle {
	s.mu.Lock()
	s.calls = append(s.calls, opts.SessionID)
	handler := s.handler
	s.mu.Unlock()
	if handler != nil {
		return handler(opts, dir)
	}
	return nil
}

func (s *mockSpawner) SpawnedIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.calls))
	copy(cp, s.calls)
	return cp
}

// makeTestWorkSecret creates a valid base64url-encoded WorkSecret for testing.
func makeTestWorkSecret() string {
	ws := WorkSecret{
		Version:             1,
		SessionIngressToken: "test-token-abc",
		APIBaseURL:          "https://api.example.com",
	}
	data, _ := json.Marshal(ws)
	return base64.RawURLEncoding.EncodeToString(data)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestStartPollStop_Lifecycle(t *testing.T) {
	t.Parallel()

	doneCh := make(chan SessionDoneStatus, 1)
	doneCh <- SessionDoneCompleted

	api := &mockAPIClient{
		pollResults: []*WorkResponse{
			{
				ID:    "work-1",
				Data:  WorkData{Type: WorkDataTypeSession, ID: "session-abc"},
				Secret: makeTestWorkSecret(),
			},
			nil, // no more work after first
		},
	}

	spawner := &mockSpawner{
		handler: func(opts SessionSpawnOpts, _ string) *SessionHandle {
			return &SessionHandle{
				SessionID: opts.SessionID,
				Done:      doneCh,
			}
		},
	}

	logger := &mockLogger{}

	orch := NewBridgeOrchestrator()
	orch.Config = BridgeConfig{
		Dir:         "/tmp/test",
		MaxSessions: 1,
		SpawnMode:   SpawnModeSameDir,
	}
	orch.API = api
	orch.Spawner = spawner
	orch.Logger = logger
	// Use a short sleep to speed up tests.
	orch.Sleep = func(d time.Duration) { time.Sleep(1 * time.Millisecond) }

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- orch.Start(ctx)
	}()

	// Wait for the orchestrator to process work and the session to complete,
	// then stop.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("Start returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}

	// Verify: registration happened (API returned env-123).
	if orch.EnvironmentID != "env-123" {
		t.Errorf("expected EnvironmentID=env-123, got %s", orch.EnvironmentID)
	}

	// Verify: session was spawned.
	ids := spawner.SpawnedIDs()
	if len(ids) == 0 {
		t.Fatal("expected at least one spawned session")
	}
	if ids[0] != "session-abc" {
		t.Errorf("expected spawned session session-abc, got %s", ids[0])
	}

	// Verify: deregister was called on shutdown.
	if !api.deregisterCalled.Load() {
		t.Error("expected DeregisterEnvironment to be called on shutdown")
	}

	// Verify: banner was printed.
	msgs := logger.Messages()
	hasBanner := false
	for _, m := range msgs {
		if m == "banner" {
			hasBanner = true
			break
		}
	}
	if !hasBanner {
		t.Error("expected banner to be printed")
	}
}

func TestReconnectionAfterConnectionError(t *testing.T) {
	t.Parallel()

	connErr := &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")}

	api := &mockAPIClient{
		pollErrs: []error{
			connErr,
			connErr,
			nil, // third poll succeeds
		},
		pollResults: []*WorkResponse{
			nil, nil, nil, // no work, just testing reconnection
		},
	}

	logger := &mockLogger{}

	orch := NewBridgeOrchestrator()
	orch.Config = BridgeConfig{
		Dir:         "/tmp/test",
		MaxSessions: 1,
		SpawnMode:   SpawnModeSameDir,
	}
	orch.API = api
	orch.Logger = logger
	orch.Backoff = BackoffConfig{
		ConnInitialMS:    10,
		ConnCapMS:        50,
		ConnGiveUpMS:     600_000,
		GeneralInitialMS: 10,
		GeneralCapMS:     50,
		GeneralGiveUpMS:  600_000,
	}
	orch.Sleep = func(d time.Duration) { time.Sleep(1 * time.Millisecond) }

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- orch.Start(ctx)
	}()

	// Wait for reconnection to happen.
	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("Start returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}

	// Verify: logger recorded reconnection.
	msgs := logger.Messages()
	hasReconnect := false
	hasReconnecting := false
	for _, m := range msgs {
		if m == "reconnected" {
			hasReconnect = true
		}
		if m == "reconnecting" {
			hasReconnecting = true
		}
	}
	if !hasReconnecting {
		t.Error("expected reconnecting status updates during connection errors")
	}
	if !hasReconnect {
		t.Error("expected reconnected log after recovery from connection errors")
	}
}

func TestFatalErrorStopsLoop(t *testing.T) {
	t.Parallel()

	api := &mockAPIClient{
		pollErrs: []error{
			&BridgeFatalError{Status: 401, Msg: "Authentication failed", ErrorType: "auth_error"},
		},
	}

	logger := &mockLogger{}

	orch := NewBridgeOrchestrator()
	orch.Config = BridgeConfig{
		Dir:         "/tmp/test",
		MaxSessions: 1,
		SpawnMode:   SpawnModeSameDir,
	}
	orch.API = api
	orch.Logger = logger
	orch.Sleep = func(d time.Duration) { time.Sleep(1 * time.Millisecond) }

	ctx := context.Background()
	err := orch.Start(ctx)

	if err == nil {
		t.Fatal("expected error from fatal poll error")
	}

	var fatalErr *BridgeFatalError
	if !strings.Contains(err.Error(), "Authentication failed") {
		t.Errorf("expected Authentication failed error, got: %v", err)
	}
	_ = fatalErr

	// Verify: logger recorded error.
	msgs := logger.Messages()
	hasError := false
	for _, m := range msgs {
		if strings.HasPrefix(m, "error:") {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("expected error logged for fatal error")
	}
}

func TestRegistrationFailure(t *testing.T) {
	t.Parallel()

	api := &mockAPIClient{
		registerErr: fmt.Errorf("network down"),
	}

	orch := NewBridgeOrchestrator()
	orch.Config = BridgeConfig{Dir: "/tmp/test", MaxSessions: 1}
	orch.API = api
	orch.Sleep = func(d time.Duration) {}

	err := orch.Start(context.Background())
	if err == nil {
		t.Fatal("expected error from registration failure")
	}
	if !strings.Contains(err.Error(), "register environment") {
		t.Errorf("expected 'register environment' in error, got: %v", err)
	}
}

func TestConnectionGiveUp(t *testing.T) {
	t.Parallel()

	connErr := &net.OpError{Op: "dial", Err: fmt.Errorf("refused")}

	// Always return connection error.
	api := &mockAPIClient{
		pollErrs: []error{connErr},
	}

	logger := &mockLogger{}

	orch := NewBridgeOrchestrator()
	orch.Config = BridgeConfig{Dir: "/tmp/test", MaxSessions: 1, SpawnMode: SpawnModeSameDir}
	orch.API = api
	orch.Logger = logger
	orch.Backoff = BackoffConfig{
		ConnInitialMS:    1,
		ConnCapMS:        10_000, // high cap so sleep detection threshold stays large
		ConnGiveUpMS:     50,     // give up after 50ms
		GeneralInitialMS: 1,
		GeneralCapMS:     10_000,
		GeneralGiveUpMS:  600_000,
	}
	orch.Sleep = func(d time.Duration) { time.Sleep(5 * time.Millisecond) }

	err := orch.Start(context.Background())
	if err == nil {
		t.Fatal("expected give-up error")
	}
	if !strings.Contains(err.Error(), "giving up") {
		t.Errorf("expected 'giving up' in error, got: %v", err)
	}
}

func TestIsConnectionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"net.OpError", &net.OpError{Op: "dial", Err: fmt.Errorf("refused")}, true},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"connection refused string", fmt.Errorf("connection refused"), true},
		{"random error", fmt.Errorf("something else"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConnectionError(tt.err); got != tt.want {
				t.Errorf("IsConnectionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsServerError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"server error 500", fmt.Errorf("server error: 500"), true},
		{"server error 502", fmt.Errorf("server error: 502"), true},
		{"other error", fmt.Errorf("something else"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsServerError(tt.err); got != tt.want {
				t.Errorf("IsServerError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDeriveSessionTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"hello  world", "hello world"},
		{"  spaced  out  ", "spaced out"},
		{strings.Repeat("a", 100), strings.Repeat("a", TitleMaxLen)},
		{"short", "short"},
	}

	for _, tt := range tests {
		t.Run(tt.input[:min(len(tt.input), 20)], func(t *testing.T) {
			got := DeriveSessionTitle(tt.input)
			if got != tt.want {
				t.Errorf("DeriveSessionTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBackoffConfig_Defaults(t *testing.T) {
	t.Parallel()

	b := DefaultBackoff
	if b.ConnInitialMS != 2000 {
		t.Errorf("ConnInitialMS = %d, want 2000", b.ConnInitialMS)
	}
	if b.ConnCapMS != 120_000 {
		t.Errorf("ConnCapMS = %d, want 120000", b.ConnCapMS)
	}
	if b.ConnGiveUpMS != 600_000 {
		t.Errorf("ConnGiveUpMS = %d, want 600000", b.ConnGiveUpMS)
	}
	if b.GeneralInitialMS != 500 {
		t.Errorf("GeneralInitialMS = %d, want 500", b.GeneralInitialMS)
	}
	if b.GeneralCapMS != 30_000 {
		t.Errorf("GeneralCapMS = %d, want 30000", b.GeneralCapMS)
	}
	if b.GeneralGiveUpMS != 600_000 {
		t.Errorf("GeneralGiveUpMS = %d, want 600000", b.GeneralGiveUpMS)
	}
}

func TestBackoffConfig_ShutdownGrace(t *testing.T) {
	t.Parallel()

	b := BackoffConfig{}
	if b.shutdownGrace() != 30*time.Second {
		t.Errorf("default shutdownGrace = %v, want 30s", b.shutdownGrace())
	}

	b.ShutdownGraceMS = 5000
	if b.shutdownGrace() != 5*time.Second {
		t.Errorf("custom shutdownGrace = %v, want 5s", b.shutdownGrace())
	}
}

func TestBackoffConfig_PollSleepDetectionThreshold(t *testing.T) {
	t.Parallel()

	b := DefaultBackoff
	expected := time.Duration(b.ConnCapMS*2) * time.Millisecond
	if b.PollSleepDetectionThreshold() != expected {
		t.Errorf("PollSleepDetectionThreshold = %v, want %v", b.PollSleepDetectionThreshold(), expected)
	}
}

func TestAddJitter(t *testing.T) {
	t.Parallel()

	// addJitter should return values within +-25% of the input.
	for i := 0; i < 100; i++ {
		result := addJitter(1000)
		if result < 750 || result > 1250 {
			t.Errorf("addJitter(1000) = %d, outside [750, 1250]", result)
		}
	}

	// Zero input should return zero.
	if result := addJitter(0); result != 0 {
		t.Errorf("addJitter(0) = %d, want 0", result)
	}
}

func TestFormatDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ms   int
		want string
	}{
		{500, "500ms"},
		{1000, "1.0s"},
		{2500, "2.5s"},
	}
	for _, tt := range tests {
		if got := formatDelay(tt.ms); got != tt.want {
			t.Errorf("formatDelay(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

func TestFormatDurationMS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ms   int
		want string
	}{
		{500, "500ms"},
		{5000, "5s"},
		{65000, "1m5s"},
	}
	for _, tt := range tests {
		if got := formatDurationMS(tt.ms); got != tt.want {
			t.Errorf("formatDurationMS(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

func TestMainConstants(t *testing.T) {
	t.Parallel()

	if StatusUpdateInterval != 1*time.Second {
		t.Errorf("StatusUpdateInterval = %v, want 1s", StatusUpdateInterval)
	}
	if SpawnSessionsDefault != 32 {
		t.Errorf("SpawnSessionsDefault = %d, want 32", SpawnSessionsDefault)
	}
	if TitleMaxLen != 80 {
		t.Errorf("TitleMaxLen = %d, want 80", TitleMaxLen)
	}
}

