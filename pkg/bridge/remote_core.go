// RemoteBridgeCore — shared abstraction composed by both BridgeOrchestrator
// (standalone bridge mode, T193) and ReplBridge (REPL bridge mode, T195).
//
// Encapsulates the common concerns:
//   - environment registration info
//   - poll config management
//   - session count tracking with capacity state
//   - config merging (local BridgeConfig + remote EnvLessBridgeConfig)
//   - feature flag refresh
//   - status tracking (BridgeState lifecycle)
//   - debug logging with [bridge:core] prefix
//   - graceful multi-session shutdown coordination
//
// Source: src/bridge/remoteBridgeCore.ts (shared env-less core abstraction)
package bridge

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// NOTE: AnthropicVersion ("2023-06-01") is declared in api.go and reused here.

// DefaultConnectTimeoutMS is the default timeout for transport connection (15s).
const DefaultConnectTimeoutMS = 15_000

// ---------------------------------------------------------------------------
// OAuthHeaders — builds the standard auth header set for bridge API calls
// ---------------------------------------------------------------------------

// OAuthHeaders returns the standard HTTP headers for OAuth-authenticated
// bridge API calls. Matches TS oauthHeaders().
func OAuthHeaders(accessToken string) map[string]string {
	return map[string]string{
		"Authorization":    "Bearer " + accessToken,
		"Content-Type":     "application/json",
		"anthropic-version": AnthropicVersion,
	}
}

// ---------------------------------------------------------------------------
// RemoteBridgeCoreConfig — all injectable dependencies for the core
// ---------------------------------------------------------------------------

// RemoteBridgeCoreConfig holds the configuration and callbacks for a
// RemoteBridgeCore instance. Callers wire these once; tests supply stubs.
type RemoteBridgeCoreConfig struct {
	// MaxSessions is the session capacity limit. Zero means unlimited.
	MaxSessions int

	// LocalConfig is the bridge-level configuration (Dir, SpawnMode, etc.).
	LocalConfig BridgeConfig

	// RemoteConfig is the timing/behavior config fetched from GrowthBook.
	// If nil, DefaultEnvLessBridgeConfig is used.
	RemoteConfig *EnvLessBridgeConfig

	// PollConfig is the dynamic poll interval configuration. If nil, a
	// default DynamicPollConfig is created.
	PollConfig *DynamicPollConfig

	// OnDebug receives debug log messages. May be nil.
	OnDebug func(msg string)

	// OnStateChange is called when the bridge state changes. May be nil.
	OnStateChange StateChangeFunc

	// OnSessionCountChange is called when the active session count changes.
	// Receives (activeCount, maxSessions). May be nil.
	OnSessionCountChange func(active, max int)
}

// ---------------------------------------------------------------------------
// MergedConfig — result of merging local + remote configs
// ---------------------------------------------------------------------------

// MergedConfig holds the merged result of local BridgeConfig and remote
// EnvLessBridgeConfig. Both are accessible; computed fields provide
// convenient time.Duration values.
type MergedConfig struct {
	Local  BridgeConfig
	Remote EnvLessBridgeConfig
}

// ConnectTimeout returns the connect timeout as a time.Duration.
func (m MergedConfig) ConnectTimeout() time.Duration {
	return time.Duration(m.Remote.ConnectTimeoutMS) * time.Millisecond
}

// HTTPTimeout returns the HTTP timeout as a time.Duration.
func (m MergedConfig) HTTPTimeout() time.Duration {
	return time.Duration(m.Remote.HTTPTimeoutMS) * time.Millisecond
}

// HeartbeatInterval returns the heartbeat interval as a time.Duration.
func (m MergedConfig) HeartbeatInterval() time.Duration {
	return time.Duration(m.Remote.HeartbeatIntervalMS) * time.Millisecond
}

// TokenRefreshBuffer returns the token refresh buffer as a time.Duration.
func (m MergedConfig) TokenRefreshBuffer() time.Duration {
	return time.Duration(m.Remote.TokenRefreshBufferMS) * time.Millisecond
}

// TeardownArchiveTimeout returns the teardown archive timeout as a time.Duration.
func (m MergedConfig) TeardownArchiveTimeout() time.Duration {
	return time.Duration(m.Remote.TeardownArchiveTimeoutMS) * time.Millisecond
}

// ---------------------------------------------------------------------------
// SessionEntry — metadata for a tracked session
// ---------------------------------------------------------------------------

// SessionEntry holds metadata for an active session tracked by the core.
type SessionEntry struct {
	SessionID string
	StartedAt time.Time
}

// ---------------------------------------------------------------------------
// RemoteBridgeCore
// ---------------------------------------------------------------------------

// RemoteBridgeCore is the shared abstraction between standalone bridge mode
// (BridgeOrchestrator) and REPL bridge mode (ReplBridge). It encapsulates
// environment registration, poll config, session spawning coordination,
// status tracking, debug logging, config merging, feature flag refresh,
// session count tracking, and graceful multi-session shutdown coordination.
//
// Thread-safe: all methods may be called concurrently.
type RemoteBridgeCore struct {
	cfg RemoteBridgeCoreConfig

	mu       sync.RWMutex
	state    BridgeState
	sessions map[string]SessionEntry
	merged   MergedConfig
	tornDown bool

	// done is closed when Shutdown completes.
	done     chan struct{}
	doneOnce sync.Once

	// now is an injectable clock for testing.
	now func() time.Time
}

// NewRemoteBridgeCore creates a new core with the given configuration.
// The core starts in BridgeStateReady.
func NewRemoteBridgeCore(cfg RemoteBridgeCoreConfig) *RemoteBridgeCore {
	remote := DefaultEnvLessBridgeConfig
	if cfg.RemoteConfig != nil {
		remote = *cfg.RemoteConfig
	}

	return &RemoteBridgeCore{
		cfg:      cfg,
		state:    BridgeStateReady,
		sessions: make(map[string]SessionEntry),
		merged: MergedConfig{
			Local:  cfg.LocalConfig,
			Remote: remote,
		},
		done: make(chan struct{}),
		now:  time.Now,
	}
}

// ---------------------------------------------------------------------------
// Config access
// ---------------------------------------------------------------------------

// Config returns the merged (local + remote) configuration.
func (c *RemoteBridgeCore) Config() MergedConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.merged
}

// UpdateRemoteConfig hot-swaps the remote config (e.g. on GrowthBook refresh).
// The new config is validated; if invalid, the default is used instead.
// Returns the validated config that was applied.
func (c *RemoteBridgeCore) UpdateRemoteConfig(remote EnvLessBridgeConfig) EnvLessBridgeConfig {
	validated, ok := ValidateEnvLessBridgeConfig(remote)
	if !ok {
		validated = DefaultEnvLessBridgeConfig
	}

	c.mu.Lock()
	c.merged.Remote = validated
	c.mu.Unlock()

	c.debug(fmt.Sprintf("[bridge:core] Remote config updated (connect_timeout=%dms, heartbeat=%dms)",
		validated.ConnectTimeoutMS, validated.HeartbeatIntervalMS))

	return validated
}

// MergeConfigs creates a MergedConfig from local and remote sources. This is
// a pure function that does not modify the core's state — useful for callers
// that need to preview a merge before committing.
func MergeConfigs(local BridgeConfig, remote EnvLessBridgeConfig) MergedConfig {
	return MergedConfig{
		Local:  local,
		Remote: remote,
	}
}

// ---------------------------------------------------------------------------
// State management
// ---------------------------------------------------------------------------

// State returns the current bridge state.
func (c *RemoteBridgeCore) State() BridgeState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// TransitionState moves to the given state with an optional detail message.
// Fires the OnStateChange callback if registered.
func (c *RemoteBridgeCore) TransitionState(next BridgeState, detail string) {
	c.mu.Lock()
	prev := c.state
	c.state = next
	c.mu.Unlock()

	c.debug(fmt.Sprintf("[bridge:core] State %s → %s (detail=%q)", prev, next, detail))

	if c.cfg.OnStateChange != nil {
		c.cfg.OnStateChange(next, detail)
	}
}

// IsTornDown reports whether Shutdown has been called.
func (c *RemoteBridgeCore) IsTornDown() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tornDown
}

// ---------------------------------------------------------------------------
// Session count tracking
// ---------------------------------------------------------------------------

// ActiveSessionCount returns the number of active sessions.
func (c *RemoteBridgeCore) ActiveSessionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sessions)
}

// MaxSessions returns the configured max sessions.
func (c *RemoteBridgeCore) MaxSessions() int {
	return c.cfg.MaxSessions
}

// AtCapacity reports whether the core is at maximum session capacity.
// Returns false when MaxSessions is zero (unlimited).
func (c *RemoteBridgeCore) AtCapacity() bool {
	if c.cfg.MaxSessions <= 0 {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sessions) >= c.cfg.MaxSessions
}

// CapacityState returns the current capacity state for poll interval selection.
func (c *RemoteBridgeCore) CapacityState() CapacityState {
	if c.cfg.MaxSessions <= 0 {
		return CapacityNone
	}
	c.mu.RLock()
	count := len(c.sessions)
	c.mu.RUnlock()

	switch {
	case count >= c.cfg.MaxSessions:
		return CapacityFull
	case count > 0:
		return CapacityPartial
	default:
		return CapacityNone
	}
}

// RegisterSession adds a session to the tracking map. Returns false if at
// capacity (caller should reject the session). Thread-safe.
func (c *RemoteBridgeCore) RegisterSession(sessionID string) bool {
	c.mu.Lock()
	if c.cfg.MaxSessions > 0 && len(c.sessions) >= c.cfg.MaxSessions {
		c.mu.Unlock()
		c.debug(fmt.Sprintf("[bridge:core] At capacity (%d/%d), rejecting session %s",
			len(c.sessions), c.cfg.MaxSessions, sessionID))
		return false
	}
	c.sessions[sessionID] = SessionEntry{
		SessionID: sessionID,
		StartedAt: c.clock(),
	}
	count := len(c.sessions)
	c.mu.Unlock()

	c.debug(fmt.Sprintf("[bridge:core] Registered session %s (active=%d/%d)",
		sessionID, count, c.cfg.MaxSessions))

	if c.cfg.OnSessionCountChange != nil {
		c.cfg.OnSessionCountChange(count, c.cfg.MaxSessions)
	}
	return true
}

// UnregisterSession removes a session from the tracking map. Thread-safe.
func (c *RemoteBridgeCore) UnregisterSession(sessionID string) {
	c.mu.Lock()
	delete(c.sessions, sessionID)
	count := len(c.sessions)
	c.mu.Unlock()

	c.debug(fmt.Sprintf("[bridge:core] Unregistered session %s (active=%d/%d)",
		sessionID, count, c.cfg.MaxSessions))

	if c.cfg.OnSessionCountChange != nil {
		c.cfg.OnSessionCountChange(count, c.cfg.MaxSessions)
	}
}

// Sessions returns a snapshot of all active session entries.
func (c *RemoteBridgeCore) Sessions() []SessionEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]SessionEntry, 0, len(c.sessions))
	for _, entry := range c.sessions {
		out = append(out, entry)
	}
	return out
}

// ---------------------------------------------------------------------------
// Debug logging
// ---------------------------------------------------------------------------

// Debug logs a debug message through the configured callback.
func (c *RemoteBridgeCore) Debug(msg string) {
	c.debug(msg)
}

func (c *RemoteBridgeCore) debug(msg string) {
	if c.cfg.OnDebug != nil {
		c.cfg.OnDebug(msg)
	}
}

// ---------------------------------------------------------------------------
// Shutdown coordination
// ---------------------------------------------------------------------------

// Shutdown initiates graceful teardown. It marks the core as torn down,
// transitions to the failed state, and closes the Done channel.
// Blocks until all registered sessions have been unregistered or the
// context deadline expires. Safe to call multiple times; only the first
// call has effect.
func (c *RemoteBridgeCore) Shutdown(ctx context.Context) {
	c.mu.Lock()
	if c.tornDown {
		c.mu.Unlock()
		<-c.done
		return
	}
	c.tornDown = true
	c.mu.Unlock()

	c.debug("[bridge:core] Shutdown initiated")

	// Wait for all sessions to unregister (or context to expire).
	c.waitForSessionDrain(ctx)

	c.TransitionState(BridgeStateFailed, "shutdown")

	c.doneOnce.Do(func() {
		close(c.done)
	})

	c.debug("[bridge:core] Shutdown complete")
}

// waitForSessionDrain blocks until all sessions are unregistered or the
// context expires. Polls every 50ms for session count.
func (c *RemoteBridgeCore) waitForSessionDrain(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		c.mu.RLock()
		count := len(c.sessions)
		c.mu.RUnlock()

		if count == 0 {
			return
		}

		select {
		case <-ctx.Done():
			c.debug(fmt.Sprintf("[bridge:core] Shutdown drain timeout with %d session(s) remaining", count))
			return
		case <-ticker.C:
			// Continue polling.
		}
	}
}

// Done returns a channel that is closed when Shutdown completes.
func (c *RemoteBridgeCore) Done() <-chan struct{} {
	return c.done
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (c *RemoteBridgeCore) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}
