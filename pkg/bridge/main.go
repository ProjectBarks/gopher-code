// Bridge main orchestrator — top-level coordinator for the CCR bridge lifecycle.
// Source: src/bridge/bridgeMain.ts (core orchestration subset)
package bridge

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// StatusUpdateInterval is the period between status display refreshes.
const StatusUpdateInterval = 1 * time.Second

// SpawnSessionsDefault is the default max_sessions for multi-session spawn modes.
const SpawnSessionsDefault = 32

// TitleMaxLen is the maximum length for a derived session title.
const TitleMaxLen = 80

// ---------------------------------------------------------------------------
// BackoffConfig — exponential backoff parameters for the poll loop
// ---------------------------------------------------------------------------

// BackoffConfig configures the dual-track exponential backoff used by the
// bridge poll loop. Connection errors and general errors each have independent
// initial/cap/give-up timers so a transient DNS blip doesn't reset the budget
// for a sustained 429 stream (and vice versa).
type BackoffConfig struct {
	ConnInitialMS   int // first delay for connection errors (ms)
	ConnCapMS       int // max delay for connection errors (ms)
	ConnGiveUpMS    int // total elapsed before giving up on connection errors (ms)
	GeneralInitialMS int // first delay for general (non-connection) errors (ms)
	GeneralCapMS     int // max delay for general errors (ms)
	GeneralGiveUpMS  int // total elapsed before giving up on general errors (ms)

	// ShutdownGraceMS is the SIGTERM→SIGKILL grace period. Default 30_000.
	ShutdownGraceMS int
	// StopWorkBaseDelayMS is the base delay for stopWork retries. Default 1_000.
	StopWorkBaseDelayMS int
}

// DefaultBackoff matches the TS DEFAULT_BACKOFF constants.
var DefaultBackoff = BackoffConfig{
	ConnInitialMS:   2_000,
	ConnCapMS:       120_000,  // 2 minutes
	ConnGiveUpMS:    600_000,  // 10 minutes
	GeneralInitialMS: 500,
	GeneralCapMS:     30_000,
	GeneralGiveUpMS:  600_000, // 10 minutes
}

// shutdownGrace returns the configured grace period or the default (30s).
func (b BackoffConfig) shutdownGrace() time.Duration {
	if b.ShutdownGraceMS > 0 {
		return time.Duration(b.ShutdownGraceMS) * time.Millisecond
	}
	return 30 * time.Second
}

// PollSleepDetectionThreshold returns the threshold for detecting system
// sleep/wake in the poll loop. Must exceed the max backoff cap — otherwise
// normal backoff delays trigger false sleep detection. Uses 2x the
// connection backoff cap, matching WebSocketTransport and replBridge.
func (b BackoffConfig) PollSleepDetectionThreshold() time.Duration {
	return time.Duration(b.ConnCapMS*2) * time.Millisecond
}

// ---------------------------------------------------------------------------
// Error classifiers
// ---------------------------------------------------------------------------

// IsConnectionError reports whether err looks like a network-level failure
// (refused, reset, timeout, unreachable). Matches the TS CONNECTION_ERROR_CODES
// set: ECONNREFUSED, ECONNRESET, ETIMEDOUT, ENETUNREACH, EHOSTUNREACH.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// net.OpError wraps most Go network failures.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	// net.DNSError covers resolution failures.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	// context.DeadlineExceeded is the Go equivalent of ETIMEDOUT.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Check for common network error substrings as a fallback.
	msg := err.Error()
	for _, code := range []string{"connection refused", "connection reset", "no such host", "network is unreachable", "host is unreachable"} {
		if strings.Contains(strings.ToLower(msg), code) {
			return true
		}
	}
	return false
}

// IsServerError reports whether err represents an HTTP 5xx response.
// In Go the bridge API client returns fmt.Errorf("server error: %d", status)
// for status >= 500, so we check for that prefix.
func IsServerError(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), "server error:")
}

// ---------------------------------------------------------------------------
// BridgeOrchestrator — top-level coordinator
// ---------------------------------------------------------------------------

// BridgeOrchestrator ties together the API client, poll config, session
// runner, logger, and debug logging into the main bridge lifecycle:
// register environment → poll loop → dispatch work → graceful shutdown.
type BridgeOrchestrator struct {
	Config            BridgeConfig
	EnvironmentID     string
	EnvironmentSecret string
	API               BridgeAPIClient
	Spawner           SessionSpawner
	Logger            BridgeLogger
	Debug             *BridgeDebug
	Backoff           BackoffConfig
	PollConfig        *DynamicPollConfig

	// Now is an injectable clock for testing. Defaults to time.Now.
	Now func() time.Time
	// Sleep is an injectable sleep for testing. Defaults to time.Sleep.
	Sleep func(d time.Duration)

	mu              sync.Mutex
	running         bool
	cancel          context.CancelFunc
	activeSessions  map[string]*SessionHandle
	sessionWorkIDs  map[string]string
	completedWorkIDs map[string]struct{}
	done            chan struct{}
}

// NewBridgeOrchestrator creates an orchestrator with default backoff and
// poll configuration. The caller must set Config, API, Spawner, and Logger
// before calling Start.
func NewBridgeOrchestrator() *BridgeOrchestrator {
	return &BridgeOrchestrator{
		Backoff:          DefaultBackoff,
		PollConfig:       NewDynamicPollConfig(DefaultPollConfig, false),
		activeSessions:   make(map[string]*SessionHandle),
		sessionWorkIDs:   make(map[string]string),
		completedWorkIDs: make(map[string]struct{}),
		done:             make(chan struct{}),
		Now:              time.Now,
		Sleep:            time.Sleep,
	}
}

// ActiveSessionCount returns the number of currently active sessions.
func (o *BridgeOrchestrator) ActiveSessionCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.activeSessions)
}

// Done returns a channel closed when the orchestrator has fully stopped.
func (o *BridgeOrchestrator) Done() <-chan struct{} {
	return o.done
}

// ---------------------------------------------------------------------------
// Start — register environment, begin poll loop
// ---------------------------------------------------------------------------

// Start registers the environment with the bridge API and begins the poll
// loop. It blocks until the context is cancelled or a fatal error occurs.
// The caller should run Start in a goroutine and select on Done().
func (o *BridgeOrchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	if o.running {
		o.mu.Unlock()
		return fmt.Errorf("orchestrator already running")
	}
	o.running = true
	ctx, o.cancel = context.WithCancel(ctx)
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.running = false
		o.mu.Unlock()
		close(o.done)
	}()

	o.debug("[bridge:orchestrator] Starting bridge orchestrator")

	// Register environment.
	resp, err := o.API.RegisterBridgeEnvironment(o.Config)
	if err != nil {
		o.debug(fmt.Sprintf("[bridge:orchestrator] Registration failed: %s", err))
		return fmt.Errorf("register environment: %w", err)
	}
	o.EnvironmentID = resp.EnvironmentID
	o.EnvironmentSecret = resp.EnvironmentSecret
	o.debug(fmt.Sprintf("[bridge:orchestrator] Registered environment_id=%s", o.EnvironmentID))

	if o.Logger != nil {
		o.Logger.PrintBanner(o.Config, o.EnvironmentID)
	}

	// Run the poll loop.
	return o.pollLoop(ctx)
}

// ---------------------------------------------------------------------------
// Stop — graceful shutdown
// ---------------------------------------------------------------------------

// Stop initiates a graceful shutdown: cancels the poll loop context, kills
// active sessions (SIGTERM then SIGKILL after grace period), deregisters the
// environment, and closes the Done channel.
func (o *BridgeOrchestrator) Stop() {
	o.mu.Lock()
	if !o.running {
		o.mu.Unlock()
		return
	}
	cancelFn := o.cancel
	o.mu.Unlock()

	o.debug("[bridge:orchestrator] Stop requested")
	if cancelFn != nil {
		cancelFn()
	}

	// Wait for poll loop to finish.
	<-o.done
}

// ---------------------------------------------------------------------------
// Poll loop
// ---------------------------------------------------------------------------

func (o *BridgeOrchestrator) pollLoop(ctx context.Context) error {
	var (
		connBackoff      int
		generalBackoff   int
		connErrorStart   *time.Time
		generalErrorStart *time.Time
		lastPollErrTime  *time.Time
	)

	o.debug(fmt.Sprintf("[bridge:orchestrator] Starting poll loop spawnMode=%s maxSessions=%d environmentId=%s",
		o.Config.SpawnMode, o.Config.MaxSessions, o.EnvironmentID))

	for {
		if ctx.Err() != nil {
			break
		}

		work, err := o.API.PollForWork(o.EnvironmentID, o.EnvironmentSecret, nil)

		if err != nil {
			if ctx.Err() != nil {
				break
			}

			// Fatal errors — no point retrying.
			var fatalErr *BridgeFatalError
			if errors.As(err, &fatalErr) {
				o.debug(fmt.Sprintf("[bridge:orchestrator] Fatal error: %s", fatalErr.Error()))
				if o.Logger != nil {
					if IsExpiredErrorType(fatalErr.ErrorType) {
						o.Logger.LogStatus(fatalErr.Error())
					} else {
						o.Logger.LogError(fatalErr.Error())
					}
				}
				o.shutdownSessions(ctx)
				return fatalErr
			}

			now := o.now()

			// Detect system sleep/wake.
			if lastPollErrTime != nil {
				gap := now.Sub(*lastPollErrTime)
				if gap > o.Backoff.PollSleepDetectionThreshold() {
					o.debug(fmt.Sprintf("[bridge:orchestrator] Detected system sleep (%ds gap), resetting error budget",
						int(gap.Seconds())))
					connErrorStart = nil
					connBackoff = 0
					generalErrorStart = nil
					generalBackoff = 0
				}
			}
			lastPollErrTime = &now

			if IsConnectionError(err) || IsServerError(err) {
				if connErrorStart == nil {
					connErrorStart = &now
				}
				elapsed := now.Sub(*connErrorStart)
				if elapsed >= time.Duration(o.Backoff.ConnGiveUpMS)*time.Millisecond {
					msg := fmt.Sprintf("Server unreachable for %d minutes, giving up.",
						int(elapsed.Minutes()))
					o.debug("[bridge:orchestrator] " + msg)
					if o.Logger != nil {
						o.Logger.LogError(msg)
					}
					o.shutdownSessions(ctx)
					return fmt.Errorf("%s", msg)
				}

				// Reset general track.
				generalErrorStart = nil
				generalBackoff = 0

				if connBackoff == 0 {
					connBackoff = o.Backoff.ConnInitialMS
				} else {
					connBackoff = min(connBackoff*2, o.Backoff.ConnCapMS)
				}
				delay := addJitter(connBackoff)
				if o.Logger != nil {
					o.Logger.LogVerbose(fmt.Sprintf("Connection error, retrying in %s (%ds elapsed): %s",
						formatDelay(delay), int(elapsed.Seconds()), err))
					o.Logger.UpdateReconnectingStatus(formatDelay(delay), formatDurationMS(int(elapsed.Milliseconds())))
				}
				o.sleep(time.Duration(delay) * time.Millisecond)
			} else {
				if generalErrorStart == nil {
					generalErrorStart = &now
				}
				elapsed := now.Sub(*generalErrorStart)
				if elapsed >= time.Duration(o.Backoff.GeneralGiveUpMS)*time.Millisecond {
					msg := fmt.Sprintf("Persistent errors for %d minutes, giving up.",
						int(elapsed.Minutes()))
					o.debug("[bridge:orchestrator] " + msg)
					if o.Logger != nil {
						o.Logger.LogError(msg)
					}
					o.shutdownSessions(ctx)
					return fmt.Errorf("%s", msg)
				}

				// Reset connection track.
				connErrorStart = nil
				connBackoff = 0

				if generalBackoff == 0 {
					generalBackoff = o.Backoff.GeneralInitialMS
				} else {
					generalBackoff = min(generalBackoff*2, o.Backoff.GeneralCapMS)
				}
				delay := addJitter(generalBackoff)
				if o.Logger != nil {
					o.Logger.LogVerbose(fmt.Sprintf("Poll failed, retrying in %s (%ds elapsed): %s",
						formatDelay(delay), int(elapsed.Seconds()), err))
					o.Logger.UpdateReconnectingStatus(formatDelay(delay), formatDurationMS(int(elapsed.Milliseconds())))
				}
				o.sleep(time.Duration(delay) * time.Millisecond)
			}
			continue
		}

		// Successful poll — reset error tracking.
		wasDisconnected := connErrorStart != nil || generalErrorStart != nil
		if wasDisconnected {
			disconnectedMS := int(o.now().Sub(*firstNonNil(connErrorStart, generalErrorStart)).Milliseconds())
			if o.Logger != nil {
				o.Logger.LogReconnected(time.Duration(disconnectedMS) * time.Millisecond)
			}
			o.debug(fmt.Sprintf("[bridge:orchestrator] Reconnected after %s", formatDurationMS(disconnectedMS)))
		}
		connBackoff = 0
		generalBackoff = 0
		connErrorStart = nil
		generalErrorStart = nil
		lastPollErrTime = nil

		// No work available.
		if work == nil {
			delay := o.PollConfig.NextPollDelay()
			o.PollConfig.RecordEmptyPoll()
			o.sleep(delay)
			continue
		}

		// Work received — reset empty poll counter.
		o.PollConfig.RecordWork()

		// Skip already-completed work (server redelivery).
		o.mu.Lock()
		_, completed := o.completedWorkIDs[work.ID]
		o.mu.Unlock()
		if completed {
			o.debug(fmt.Sprintf("[bridge:orchestrator] Skipping already-completed workId=%s", work.ID))
			o.sleep(1 * time.Second)
			continue
		}

		o.dispatchWork(ctx, work)
	}

	// Shutdown.
	o.shutdownSessions(ctx)
	return nil
}

// dispatchWork handles a single work item by type.
func (o *BridgeOrchestrator) dispatchWork(ctx context.Context, work *WorkResponse) {
	switch work.Data.Type {
	case WorkDataTypeHealthcheck:
		o.debug("[bridge:orchestrator] Healthcheck received")
		if o.Logger != nil {
			o.Logger.LogVerbose("Healthcheck received")
		}
		// Acknowledge healthcheck.
		secret, err := DecodeWorkSecret(work.Secret)
		if err == nil {
			_ = o.API.AcknowledgeWork(o.EnvironmentID, work.ID, secret.SessionIngressToken)
		}

	case WorkDataTypeSession:
		o.handleSessionWork(ctx, work)

	default:
		o.debug(fmt.Sprintf("[bridge:orchestrator] Unknown work type: %s, skipping", work.Data.Type))
	}
}

// handleSessionWork processes a session work item: decode secret, check
// capacity, acknowledge, and spawn the session.
func (o *BridgeOrchestrator) handleSessionWork(ctx context.Context, work *WorkResponse) {
	sessionID := work.Data.ID
	if _, err := ValidateBridgeID(sessionID, "session_id"); err != nil {
		o.debug(fmt.Sprintf("[bridge:orchestrator] Invalid session_id: %s", sessionID))
		if o.Logger != nil {
			o.Logger.LogError(fmt.Sprintf("Invalid session_id received: %s", sessionID))
		}
		return
	}

	// Decode work secret.
	secret, err := DecodeWorkSecret(work.Secret)
	if err != nil {
		if o.Logger != nil {
			o.Logger.LogError(fmt.Sprintf("Failed to decode work secret for workId=%s: %s", work.ID, err))
		}
		o.mu.Lock()
		o.completedWorkIDs[work.ID] = struct{}{}
		o.mu.Unlock()
		return
	}

	// Check for existing session (token refresh for re-dispatched work).
	o.mu.Lock()
	if existing, ok := o.activeSessions[sessionID]; ok {
		if existing.UpdateAccessToken != nil {
			existing.UpdateAccessToken(secret.SessionIngressToken)
		}
		o.sessionWorkIDs[sessionID] = work.ID
		o.mu.Unlock()
		o.debug(fmt.Sprintf("[bridge:orchestrator] Updated token for existing sessionId=%s workId=%s", sessionID, work.ID))
		_ = o.API.AcknowledgeWork(o.EnvironmentID, work.ID, secret.SessionIngressToken)
		return
	}

	// Check capacity.
	if len(o.activeSessions) >= o.Config.MaxSessions {
		o.mu.Unlock()
		o.debug(fmt.Sprintf("[bridge:orchestrator] At capacity (%d/%d), cannot spawn for workId=%s",
			len(o.activeSessions), o.Config.MaxSessions, work.ID))
		return
	}
	o.mu.Unlock()

	// Acknowledge work.
	if err := o.API.AcknowledgeWork(o.EnvironmentID, work.ID, secret.SessionIngressToken); err != nil {
		o.debug(fmt.Sprintf("[bridge:orchestrator] Acknowledge failed workId=%s: %s", work.ID, err))
	}

	// Build SDK URL.
	sdkURL := BuildSdkUrl(o.Config.SessionIngressURL, sessionID)

	// Spawn session.
	o.debug(fmt.Sprintf("[bridge:orchestrator] Spawning sessionId=%s sdkUrl=%s", sessionID, sdkURL))

	if o.Spawner == nil {
		o.debug("[bridge:orchestrator] No spawner configured")
		return
	}

	handle := o.Spawner.Spawn(SessionSpawnOpts{
		SessionID:   sessionID,
		SDKURL:      sdkURL,
		AccessToken: secret.SessionIngressToken,
	}, o.Config.Dir)

	if handle == nil {
		if o.Logger != nil {
			o.Logger.LogError(fmt.Sprintf("Failed to spawn session %s", sessionID))
		}
		o.mu.Lock()
		o.completedWorkIDs[work.ID] = struct{}{}
		o.mu.Unlock()
		return
	}

	o.mu.Lock()
	o.activeSessions[sessionID] = handle
	o.sessionWorkIDs[sessionID] = work.ID
	o.mu.Unlock()

	if o.Logger != nil {
		o.Logger.LogSessionStart(sessionID, fmt.Sprintf("Session %s", sessionID))
	}

	// Watch for session completion.
	go o.watchSession(sessionID, handle)
}

// watchSession waits for a session to complete and cleans up.
func (o *BridgeOrchestrator) watchSession(sessionID string, handle *SessionHandle) {
	status := <-handle.Done

	o.mu.Lock()
	workID := o.sessionWorkIDs[sessionID]
	delete(o.activeSessions, sessionID)
	delete(o.sessionWorkIDs, sessionID)
	if workID != "" {
		o.completedWorkIDs[workID] = struct{}{}
	}
	o.mu.Unlock()

	o.debug(fmt.Sprintf("[bridge:orchestrator] Session done sessionId=%s status=%s workId=%s",
		sessionID, status, workID))

	if o.Logger != nil {
		switch status {
		case SessionDoneCompleted:
			o.Logger.LogSessionComplete(sessionID, 0)
		case SessionDoneFailed:
			o.Logger.LogSessionFailed(sessionID, "Process exited with error")
		case SessionDoneInterrupted:
			o.Logger.LogVerbose(fmt.Sprintf("Session %s interrupted", sessionID))
		}
	}

	// Notify server that work is done (skip for interrupted — server-initiated).
	if status != SessionDoneInterrupted && workID != "" {
		if err := o.API.StopWork(o.EnvironmentID, workID, false); err != nil {
			o.debug(fmt.Sprintf("[bridge:orchestrator] StopWork failed workId=%s: %s", workID, err))
		}
	}
}

// shutdownSessions kills all active sessions with grace period, then
// deregisters the environment.
func (o *BridgeOrchestrator) shutdownSessions(_ context.Context) {
	o.mu.Lock()
	sessions := make(map[string]*SessionHandle, len(o.activeSessions))
	for k, v := range o.activeSessions {
		sessions[k] = v
	}
	o.mu.Unlock()

	if len(sessions) == 0 {
		o.deregister()
		return
	}

	o.debug(fmt.Sprintf("[bridge:orchestrator] Shutting down %d active session(s)", len(sessions)))
	if o.Logger != nil {
		o.Logger.LogStatus(fmt.Sprintf("Shutting down %d active session(s)\u2026", len(sessions)))
	}

	// SIGTERM all sessions.
	for sid, handle := range sessions {
		o.debug(fmt.Sprintf("[bridge:shutdown] Sending SIGTERM to sessionId=%s", sid))
		if handle.Kill != nil {
			handle.Kill()
		}
	}

	// Wait for graceful exit or timeout.
	grace := o.Backoff.shutdownGrace()
	timer := time.NewTimer(grace)
	defer timer.Stop()

	allDone := make(chan struct{})
	go func() {
		for _, handle := range sessions {
			<-handle.Done
		}
		close(allDone)
	}()

	select {
	case <-allDone:
		// All sessions exited gracefully.
	case <-timer.C:
		// Force-kill stuck sessions.
		for sid, handle := range sessions {
			o.debug(fmt.Sprintf("[bridge:shutdown] Force-killing stuck sessionId=%s", sid))
			if handle.ForceKill != nil {
				handle.ForceKill()
			}
		}
	}

	// Stop work for all sessions.
	o.mu.Lock()
	workIDs := make(map[string]string, len(o.sessionWorkIDs))
	for k, v := range o.sessionWorkIDs {
		workIDs[k] = v
	}
	o.mu.Unlock()

	for _, workID := range workIDs {
		if err := o.API.StopWork(o.EnvironmentID, workID, true); err != nil {
			o.debug(fmt.Sprintf("[bridge:shutdown] StopWork failed workId=%s: %s", workID, err))
		}
	}

	o.deregister()
}

// deregister calls DeregisterEnvironment as a best-effort cleanup.
func (o *BridgeOrchestrator) deregister() {
	if o.EnvironmentID == "" {
		return
	}
	o.debug(fmt.Sprintf("[bridge:orchestrator] Deregistering environment_id=%s", o.EnvironmentID))
	if err := o.API.DeregisterEnvironment(o.EnvironmentID); err != nil {
		o.debug(fmt.Sprintf("[bridge:orchestrator] Deregister failed: %s", err))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (o *BridgeOrchestrator) debug(msg string) {
	if o.Debug != nil {
		o.Debug.LogStatus(msg, nil)
	}
}

func (o *BridgeOrchestrator) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o *BridgeOrchestrator) sleep(d time.Duration) {
	if o.Sleep != nil {
		o.Sleep(d)
		return
	}
	time.Sleep(d)
}

// addJitter adds +-25% jitter to a millisecond delay value.
func addJitter(ms int) int {
	jitter := float64(ms) * 0.25 * (2*rand.Float64() - 1)
	result := int(math.Max(0, float64(ms)+jitter))
	return result
}

// formatDelay formats a millisecond delay as "Xs" or "Xms".
func formatDelay(ms int) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

// formatDurationMS formats a millisecond duration for display.
func formatDurationMS(ms int) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60_000 {
		return fmt.Sprintf("%ds", ms/1000)
	}
	return fmt.Sprintf("%dm%ds", ms/60_000, (ms%60_000)/1000)
}

// firstNonNil returns the first non-nil time pointer, or a zero-value pointer.
func firstNonNil(a, b *time.Time) *time.Time {
	if a != nil {
		return a
	}
	if b != nil {
		return b
	}
	t := time.Time{}
	return &t
}

// DeriveSessionTitle collapses whitespace and truncates to TitleMaxLen.
func DeriveSessionTitle(text string) string {
	// Collapse all whitespace to single spaces.
	fields := strings.Fields(text)
	title := strings.Join(fields, " ")
	if len(title) > TitleMaxLen {
		title = title[:TitleMaxLen]
	}
	return title
}
