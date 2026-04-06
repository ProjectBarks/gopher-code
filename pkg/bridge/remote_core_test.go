package bridge

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// debugCollector collects debug messages for assertion.
type debugCollector struct {
	mu   sync.Mutex
	msgs []string
}

func (d *debugCollector) handler(msg string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.msgs = append(d.msgs, msg)
}

func (d *debugCollector) messages() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := make([]string, len(d.msgs))
	copy(cp, d.msgs)
	return cp
}

// ---------------------------------------------------------------------------
// Core initialization tests
// ---------------------------------------------------------------------------

func TestRemoteBridgeCore_NewDefaults(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 5,
		LocalConfig: BridgeConfig{Dir: "/tmp/test"},
	})

	// State should start as ready.
	if core.State() != BridgeStateReady {
		t.Errorf("initial state = %s, want %s", core.State(), BridgeStateReady)
	}

	// No sessions active.
	if core.ActiveSessionCount() != 0 {
		t.Errorf("initial ActiveSessionCount = %d, want 0", core.ActiveSessionCount())
	}

	// MaxSessions plumbed through.
	if core.MaxSessions() != 5 {
		t.Errorf("MaxSessions = %d, want 5", core.MaxSessions())
	}

	// Not torn down.
	if core.IsTornDown() {
		t.Error("expected IsTornDown = false on fresh core")
	}

	// Config should use defaults for remote.
	cfg := core.Config()
	if cfg.Remote.ConnectTimeoutMS != DefaultEnvLessBridgeConfig.ConnectTimeoutMS {
		t.Errorf("default ConnectTimeoutMS = %d, want %d",
			cfg.Remote.ConnectTimeoutMS, DefaultEnvLessBridgeConfig.ConnectTimeoutMS)
	}
	if cfg.Local.Dir != "/tmp/test" {
		t.Errorf("local Dir = %q, want /tmp/test", cfg.Local.Dir)
	}
}

func TestRemoteBridgeCore_NewWithRemoteConfig(t *testing.T) {
	t.Parallel()

	remote := EnvLessBridgeConfig{
		InitRetryMaxAttempts:       2,
		InitRetryBaseDelayMS:       200,
		InitRetryJitterFraction:    0.1,
		InitRetryMaxDelayMS:        2000,
		HTTPTimeoutMS:              5000,
		UUIDDedupBufferSize:        500,
		HeartbeatIntervalMS:        10_000,
		HeartbeatJitterFraction:    0.2,
		TokenRefreshBufferMS:       60_000,
		TeardownArchiveTimeoutMS:   1000,
		ConnectTimeoutMS:           10_000,
		MinVersion:                 "1.0.0",
		ShouldShowAppUpgradeMessage: false,
	}

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions:  3,
		RemoteConfig: &remote,
	})

	cfg := core.Config()
	if cfg.Remote.ConnectTimeoutMS != 10_000 {
		t.Errorf("ConnectTimeoutMS = %d, want 10000", cfg.Remote.ConnectTimeoutMS)
	}
	if cfg.Remote.HeartbeatIntervalMS != 10_000 {
		t.Errorf("HeartbeatIntervalMS = %d, want 10000", cfg.Remote.HeartbeatIntervalMS)
	}
}

func TestRemoteBridgeCore_DebugLogging(t *testing.T) {
	t.Parallel()

	dc := &debugCollector{}
	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 1,
		OnDebug:     dc.handler,
	})

	core.Debug("test message")

	msgs := dc.messages()
	if len(msgs) != 1 || msgs[0] != "test message" {
		t.Errorf("expected [\"test message\"], got %v", msgs)
	}

	// Register a session to produce debug output.
	core.RegisterSession("sess-1")
	msgs = dc.messages()
	found := false
	for _, m := range msgs {
		if m != "test message" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected debug output from RegisterSession")
	}
}

func TestRemoteBridgeCore_NilDebugDoesNotPanic(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 1,
		// OnDebug is nil — should not panic.
	})

	// These should all be no-ops without panic.
	core.Debug("hello")
	core.RegisterSession("s1")
	core.UnregisterSession("s1")
}

// ---------------------------------------------------------------------------
// Session count tracking tests
// ---------------------------------------------------------------------------

func TestRemoteBridgeCore_SessionTracking(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 3,
	})

	// Register three sessions.
	if !core.RegisterSession("s1") {
		t.Fatal("expected RegisterSession(s1) to succeed")
	}
	if !core.RegisterSession("s2") {
		t.Fatal("expected RegisterSession(s2) to succeed")
	}
	if !core.RegisterSession("s3") {
		t.Fatal("expected RegisterSession(s3) to succeed")
	}

	if core.ActiveSessionCount() != 3 {
		t.Errorf("ActiveSessionCount = %d, want 3", core.ActiveSessionCount())
	}

	// At capacity — fourth should be rejected.
	if core.RegisterSession("s4") {
		t.Error("expected RegisterSession(s4) to be rejected at capacity")
	}
	if core.ActiveSessionCount() != 3 {
		t.Errorf("after reject, ActiveSessionCount = %d, want 3", core.ActiveSessionCount())
	}

	// Unregister one — should allow a new one.
	core.UnregisterSession("s2")
	if core.ActiveSessionCount() != 2 {
		t.Errorf("after unregister, ActiveSessionCount = %d, want 2", core.ActiveSessionCount())
	}

	if !core.RegisterSession("s4") {
		t.Error("expected RegisterSession(s4) to succeed after unregister")
	}
	if core.ActiveSessionCount() != 3 {
		t.Errorf("after re-register, ActiveSessionCount = %d, want 3", core.ActiveSessionCount())
	}
}

func TestRemoteBridgeCore_UnlimitedSessions(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 0, // unlimited
	})

	// Should never be at capacity.
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("s%d", i)
		if !core.RegisterSession(id) {
			t.Fatalf("expected unlimited RegisterSession(%s) to succeed", id)
		}
	}
	if core.AtCapacity() {
		t.Error("expected AtCapacity = false with MaxSessions=0")
	}
	if core.ActiveSessionCount() != 100 {
		t.Errorf("ActiveSessionCount = %d, want 100", core.ActiveSessionCount())
	}
}

func TestRemoteBridgeCore_CapacityState(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 3,
	})

	if core.CapacityState() != CapacityNone {
		t.Errorf("empty CapacityState = %d, want CapacityNone", core.CapacityState())
	}

	core.RegisterSession("s1")
	if core.CapacityState() != CapacityPartial {
		t.Errorf("partial CapacityState = %d, want CapacityPartial", core.CapacityState())
	}

	core.RegisterSession("s2")
	core.RegisterSession("s3")
	if core.CapacityState() != CapacityFull {
		t.Errorf("full CapacityState = %d, want CapacityFull", core.CapacityState())
	}

	if !core.AtCapacity() {
		t.Error("expected AtCapacity = true at full capacity")
	}
}

func TestRemoteBridgeCore_SessionCountChangeCallback(t *testing.T) {
	t.Parallel()

	var counts []int
	var mu sync.Mutex

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 5,
		OnSessionCountChange: func(active, max int) {
			mu.Lock()
			counts = append(counts, active)
			mu.Unlock()
		},
	})

	core.RegisterSession("s1")
	core.RegisterSession("s2")
	core.UnregisterSession("s1")

	mu.Lock()
	defer mu.Unlock()
	if len(counts) != 3 {
		t.Fatalf("expected 3 count callbacks, got %d", len(counts))
	}
	if counts[0] != 1 || counts[1] != 2 || counts[2] != 1 {
		t.Errorf("counts = %v, want [1, 2, 1]", counts)
	}
}

func TestRemoteBridgeCore_Sessions(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 5,
	})

	core.RegisterSession("alpha")
	core.RegisterSession("beta")

	sessions := core.Sessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	ids := make(map[string]bool)
	for _, s := range sessions {
		ids[s.SessionID] = true
		if s.StartedAt.IsZero() {
			t.Errorf("session %s has zero StartedAt", s.SessionID)
		}
	}
	if !ids["alpha"] || !ids["beta"] {
		t.Errorf("expected sessions alpha and beta, got %v", ids)
	}
}

// ---------------------------------------------------------------------------
// Config merge tests
// ---------------------------------------------------------------------------

func TestMergeConfigs(t *testing.T) {
	t.Parallel()

	local := BridgeConfig{
		Dir:         "/workspace",
		MachineName: "test-host",
		MaxSessions: 4,
		SpawnMode:   SpawnModeWorktree,
	}
	remote := EnvLessBridgeConfig{
		ConnectTimeoutMS:    20_000,
		HeartbeatIntervalMS: 15_000,
		HTTPTimeoutMS:       8_000,
		TokenRefreshBufferMS: 120_000,
		TeardownArchiveTimeoutMS: 1500,
	}

	merged := MergeConfigs(local, remote)

	if merged.Local.Dir != "/workspace" {
		t.Errorf("Local.Dir = %q, want /workspace", merged.Local.Dir)
	}
	if merged.Local.MachineName != "test-host" {
		t.Errorf("Local.MachineName = %q, want test-host", merged.Local.MachineName)
	}
	if merged.Remote.ConnectTimeoutMS != 20_000 {
		t.Errorf("Remote.ConnectTimeoutMS = %d, want 20000", merged.Remote.ConnectTimeoutMS)
	}

	// Convenience accessors.
	if merged.ConnectTimeout() != 20*time.Second {
		t.Errorf("ConnectTimeout = %v, want 20s", merged.ConnectTimeout())
	}
	if merged.HTTPTimeout() != 8*time.Second {
		t.Errorf("HTTPTimeout = %v, want 8s", merged.HTTPTimeout())
	}
	if merged.HeartbeatInterval() != 15*time.Second {
		t.Errorf("HeartbeatInterval = %v, want 15s", merged.HeartbeatInterval())
	}
	if merged.TokenRefreshBuffer() != 120*time.Second {
		t.Errorf("TokenRefreshBuffer = %v, want 120s", merged.TokenRefreshBuffer())
	}
	if merged.TeardownArchiveTimeout() != 1500*time.Millisecond {
		t.Errorf("TeardownArchiveTimeout = %v, want 1.5s", merged.TeardownArchiveTimeout())
	}
}

func TestRemoteBridgeCore_UpdateRemoteConfig(t *testing.T) {
	t.Parallel()

	dc := &debugCollector{}
	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 1,
		OnDebug:     dc.handler,
	})

	// Initial config uses defaults.
	if core.Config().Remote.ConnectTimeoutMS != DefaultEnvLessBridgeConfig.ConnectTimeoutMS {
		t.Fatal("expected default connect timeout initially")
	}

	// Update with valid config.
	newRemote := DefaultEnvLessBridgeConfig
	newRemote.ConnectTimeoutMS = 30_000
	newRemote.HeartbeatIntervalMS = 25_000
	applied := core.UpdateRemoteConfig(newRemote)

	if applied.ConnectTimeoutMS != 30_000 {
		t.Errorf("applied ConnectTimeoutMS = %d, want 30000", applied.ConnectTimeoutMS)
	}
	if core.Config().Remote.ConnectTimeoutMS != 30_000 {
		t.Errorf("stored ConnectTimeoutMS = %d, want 30000", core.Config().Remote.ConnectTimeoutMS)
	}

	// Update with invalid config — should fall back to defaults.
	invalid := EnvLessBridgeConfig{
		ConnectTimeoutMS: 1, // below minimum of 5000
	}
	applied = core.UpdateRemoteConfig(invalid)
	if applied.ConnectTimeoutMS != DefaultEnvLessBridgeConfig.ConnectTimeoutMS {
		t.Errorf("invalid config should fall back to default, got ConnectTimeoutMS=%d",
			applied.ConnectTimeoutMS)
	}
}

// ---------------------------------------------------------------------------
// State transition tests
// ---------------------------------------------------------------------------

func TestRemoteBridgeCore_StateTransition(t *testing.T) {
	t.Parallel()

	var states []BridgeState
	var mu sync.Mutex

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 1,
		OnStateChange: func(state BridgeState, detail string) {
			mu.Lock()
			states = append(states, state)
			mu.Unlock()
		},
	})

	if core.State() != BridgeStateReady {
		t.Fatalf("initial state = %s, want ready", core.State())
	}

	core.TransitionState(BridgeStateConnected, "transport connected")
	if core.State() != BridgeStateConnected {
		t.Errorf("state after transition = %s, want connected", core.State())
	}

	core.TransitionState(BridgeStateReconnecting, "JWT expired")
	core.TransitionState(BridgeStateConnected, "recovered")
	core.TransitionState(BridgeStateFailed, "fatal error")

	mu.Lock()
	defer mu.Unlock()
	expected := []BridgeState{
		BridgeStateConnected,
		BridgeStateReconnecting,
		BridgeStateConnected,
		BridgeStateFailed,
	}
	if len(states) != len(expected) {
		t.Fatalf("expected %d state changes, got %d: %v", len(expected), len(states), states)
	}
	for i, s := range states {
		if s != expected[i] {
			t.Errorf("state[%d] = %s, want %s", i, s, expected[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Shutdown tests
// ---------------------------------------------------------------------------

func TestRemoteBridgeCore_ShutdownEmpty(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{MaxSessions: 5})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	core.Shutdown(ctx)

	if !core.IsTornDown() {
		t.Error("expected IsTornDown = true after Shutdown")
	}
	if core.State() != BridgeStateFailed {
		t.Errorf("state after shutdown = %s, want failed", core.State())
	}

	// Done channel should be closed.
	select {
	case <-core.Done():
		// OK
	default:
		t.Error("Done channel should be closed after Shutdown")
	}
}

func TestRemoteBridgeCore_ShutdownWaitsForSessions(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{MaxSessions: 5})
	core.RegisterSession("s1")
	core.RegisterSession("s2")

	var shutdownDone atomic.Bool

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		core.Shutdown(ctx)
		shutdownDone.Store(true)
	}()

	// Give shutdown a moment to start waiting.
	time.Sleep(100 * time.Millisecond)
	if shutdownDone.Load() {
		t.Fatal("Shutdown should block while sessions are active")
	}

	// Unregister sessions.
	core.UnregisterSession("s1")
	core.UnregisterSession("s2")

	// Wait for shutdown to complete.
	time.Sleep(200 * time.Millisecond)
	if !shutdownDone.Load() {
		t.Error("Shutdown should complete after all sessions unregistered")
	}
}

func TestRemoteBridgeCore_ShutdownTimeout(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{MaxSessions: 5})
	core.RegisterSession("stuck-session")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	core.Shutdown(ctx)
	elapsed := time.Since(start)

	if elapsed < 150*time.Millisecond {
		t.Errorf("Shutdown returned too early: %v", elapsed)
	}
	if !core.IsTornDown() {
		t.Error("expected IsTornDown = true even with timeout")
	}
}

func TestRemoteBridgeCore_ShutdownIdempotent(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{MaxSessions: 1})

	ctx := context.Background()
	core.Shutdown(ctx)
	// Second call should not panic or block.
	core.Shutdown(ctx)

	if !core.IsTornDown() {
		t.Error("expected IsTornDown = true")
	}
}

// ---------------------------------------------------------------------------
// OAuthHeaders tests
// ---------------------------------------------------------------------------

func TestOAuthHeaders(t *testing.T) {
	t.Parallel()

	headers := OAuthHeaders("tok-abc123")

	if headers["Authorization"] != "Bearer tok-abc123" {
		t.Errorf("Authorization = %q, want Bearer tok-abc123", headers["Authorization"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", headers["Content-Type"])
	}
	if headers["anthropic-version"] != AnthropicVersion {
		t.Errorf("anthropic-version = %q, want %s", headers["anthropic-version"], AnthropicVersion)
	}
}

func TestAnthropicVersionConstant(t *testing.T) {
	t.Parallel()

	if AnthropicVersion != "2023-06-01" {
		t.Errorf("AnthropicVersion = %q, want 2023-06-01", AnthropicVersion)
	}
}

// ---------------------------------------------------------------------------
// Concurrent access test
// ---------------------------------------------------------------------------

func TestRemoteBridgeCore_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	core := NewRemoteBridgeCore(RemoteBridgeCoreConfig{
		MaxSessions: 100,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sid := fmt.Sprintf("s%d", id)
			core.RegisterSession(sid)
			_ = core.ActiveSessionCount()
			_ = core.AtCapacity()
			_ = core.CapacityState()
			_ = core.Config()
			_ = core.State()
			_ = core.Sessions()
			core.UnregisterSession(sid)
		}(i)
	}
	wg.Wait()

	if core.ActiveSessionCount() != 0 {
		t.Errorf("after concurrent register/unregister, ActiveSessionCount = %d, want 0",
			core.ActiveSessionCount())
	}
}
