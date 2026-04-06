// Package bridge — SessionRunner lifecycle state machine.
// Source: src/bridge/sessionRunner.ts (lifecycle portion)
package bridge

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// MaxActivities is the ring buffer size for session activities.
const MaxActivities = 10

// MaxStderrLines is the ring buffer size for stderr capture.
const MaxStderrLines = 10

// DefaultHeartbeatInterval is the default period between heartbeat POSTs.
const DefaultHeartbeatInterval = 30 * time.Second

// safeFilenameIDPattern strips anything not alphanumeric, underscore, or hyphen.
var safeFilenameIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// SafeFilenameID sanitises a session ID for use in file names.
// Replaces anything outside [a-zA-Z0-9_-] with underscore.
func SafeFilenameID(id string) string {
	return safeFilenameIDPattern.ReplaceAllString(id, "_")
}

// ---------------------------------------------------------------------------
// ToolVerbs — human-readable verbs for tool activity display
// ---------------------------------------------------------------------------

// ToolVerbs maps tool names to human-readable verbs for the status display.
var ToolVerbs = map[string]string{
	"Read":             "Reading",
	"Write":            "Writing",
	"Edit":             "Editing",
	"MultiEdit":        "Editing",
	"Bash":             "Running",
	"Glob":             "Searching",
	"Grep":             "Searching",
	"WebFetch":         "Fetching",
	"WebSearch":        "Searching",
	"Task":             "Running task",
	"FileReadTool":     "Reading",
	"FileWriteTool":    "Writing",
	"FileEditTool":     "Editing",
	"GlobTool":         "Searching",
	"GrepTool":         "Searching",
	"BashTool":         "Running",
	"NotebookEditTool": "Editing notebook",
	"LSP":              "LSP",
}

// ---------------------------------------------------------------------------
// PermissionRequest — forwarded from child CLI
// ---------------------------------------------------------------------------

// PermissionRequest is a control_request emitted by the child CLI when it
// needs permission to execute a specific tool invocation.
type PermissionRequest struct {
	Type      string                   `json:"type"`
	RequestID string                   `json:"request_id"`
	Request   PermissionRequestPayload `json:"request"`
}

// PermissionRequestPayload is the inner request of a PermissionRequest.
type PermissionRequestPayload struct {
	Subtype   string         `json:"subtype"`
	ToolName  string         `json:"tool_name"`
	Input     map[string]any `json:"input"`
	ToolUseID string         `json:"tool_use_id"`
}

// ---------------------------------------------------------------------------
// RunnerState — lifecycle state machine
// ---------------------------------------------------------------------------

// RunnerState represents the current lifecycle phase of a SessionRunner.
type RunnerState int

const (
	// RunnerIdle is the initial state before Start is called.
	RunnerIdle RunnerState = iota
	// RunnerStarting means Start has been called; decoding secret, creating session.
	RunnerStarting
	// RunnerRunning means the session is active and heartbeating.
	RunnerRunning
	// RunnerStopping means Stop has been called; graceful teardown in progress.
	RunnerStopping
	// RunnerDone means the runner has fully shut down.
	RunnerDone
)

// String returns the human-readable name.
func (s RunnerState) String() string {
	switch s {
	case RunnerIdle:
		return "idle"
	case RunnerStarting:
		return "starting"
	case RunnerRunning:
		return "running"
	case RunnerStopping:
		return "stopping"
	case RunnerDone:
		return "done"
	default:
		return fmt.Sprintf("RunnerState(%d)", int(s))
	}
}

// ---------------------------------------------------------------------------
// StopReason
// ---------------------------------------------------------------------------

// StopReason explains why the runner was stopped.
type StopReason string

const (
	StopReasonCompleted   StopReason = "completed"
	StopReasonFailed      StopReason = "failed"
	StopReasonInterrupted StopReason = "interrupted"
	StopReasonTimeout     StopReason = "timeout"
	StopReasonLeaseExpiry StopReason = "lease_expired"
)

// ---------------------------------------------------------------------------
// SessionRunnerDeps — injected dependencies (for testability)
// ---------------------------------------------------------------------------

// SessionRunnerDeps holds the external dependencies a SessionRunner needs.
type SessionRunnerDeps struct {
	// API is the bridge API client for heartbeat/archive/deregister calls.
	API BridgeAPIClient

	// EnvironmentID is the registered environment ID for API calls.
	EnvironmentID string

	// HeartbeatInterval overrides DefaultHeartbeatInterval (zero = use default).
	HeartbeatInterval time.Duration

	// OnDebug receives debug log messages.
	OnDebug func(msg string)

	// OnStateChange is called after every state transition.
	OnStateChange func(from, to RunnerState)

	// OnFatalError is called when an unrecoverable error occurs.
	OnFatalError func(err error)
}

func (d *SessionRunnerDeps) debug(msg string) {
	if d.OnDebug != nil {
		d.OnDebug(msg)
	}
}

func (d *SessionRunnerDeps) heartbeatInterval() time.Duration {
	if d.HeartbeatInterval > 0 {
		return d.HeartbeatInterval
	}
	return DefaultHeartbeatInterval
}

// ---------------------------------------------------------------------------
// SessionRunner
// ---------------------------------------------------------------------------

// SessionRunner manages the full lifecycle of a single bridge work item:
// decode secret → register worker → heartbeat loop → graceful shutdown.
type SessionRunner struct {
	deps SessionRunnerDeps

	mu          sync.Mutex
	state       RunnerState
	stopReason  StopReason
	lastErr     error
	workID      string
	sessionID   string
	accessToken string

	// cancelHeartbeat stops the heartbeat goroutine.
	cancelHeartbeat context.CancelFunc

	// done is closed when the runner reaches RunnerDone.
	done chan struct{}
}

// NewSessionRunner creates a runner in the idle state.
func NewSessionRunner(deps SessionRunnerDeps) *SessionRunner {
	return &SessionRunner{
		deps: deps,
		done: make(chan struct{}),
	}
}

// State returns the current lifecycle state (thread-safe).
func (r *SessionRunner) State() RunnerState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// StopReason returns the reason the runner stopped (meaningful only in Done state).
func (r *SessionRunner) StopReasonValue() StopReason {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopReason
}

// LastError returns the last error that caused a transition, if any.
func (r *SessionRunner) LastError() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr
}

// Done returns a channel that is closed when the runner reaches RunnerDone.
func (r *SessionRunner) Done() <-chan struct{} {
	return r.done
}

// WorkID returns the work item ID (set after Start).
func (r *SessionRunner) WorkID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.workID
}

// SessionID returns the session ID (set after Start).
func (r *SessionRunner) SessionID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sessionID
}

// ---------------------------------------------------------------------------
// State transitions (must be called with r.mu held)
// ---------------------------------------------------------------------------

// transition moves from expected → next. Returns false if the current state
// does not match expected (invalid transition).
func (r *SessionRunner) transition(expected, next RunnerState) bool {
	if r.state != expected {
		return false
	}
	from := r.state
	r.state = next
	if r.deps.OnStateChange != nil {
		r.deps.OnStateChange(from, next)
	}
	return true
}

// transitionToDone is a convenience that moves to RunnerDone from any
// non-done state, records the reason, and closes the done channel.
func (r *SessionRunner) transitionToDone(reason StopReason, err error) {
	if r.state == RunnerDone {
		return
	}
	from := r.state
	r.state = RunnerDone
	r.stopReason = reason
	r.lastErr = err
	if r.deps.OnStateChange != nil {
		r.deps.OnStateChange(from, RunnerDone)
	}
	close(r.done)
}

// ---------------------------------------------------------------------------
// Start — idle → starting → running
// ---------------------------------------------------------------------------

// Start transitions the runner from idle → starting → running. It decodes
// the work secret from the WorkResponse, stores session metadata, and
// launches the heartbeat goroutine. The caller is responsible for spawning
// the actual child session (via SessionSpawner) — this runner handles only
// the API lifecycle (heartbeat, archive, deregister).
//
// Returns an error if the state transition is invalid or the work secret
// cannot be decoded.
func (r *SessionRunner) Start(ctx context.Context, work WorkResponse) error {
	r.mu.Lock()

	if !r.transition(RunnerIdle, RunnerStarting) {
		cur := r.state
		r.mu.Unlock()
		return fmt.Errorf("cannot start: runner is in state %s, expected idle", cur)
	}
	r.mu.Unlock()

	r.deps.debug(fmt.Sprintf("[bridge:runner] Starting work_id=%s", work.ID))

	// Decode work secret.
	secret, err := DecodeWorkSecret(work.Secret)
	if err != nil {
		r.mu.Lock()
		r.transitionToDone(StopReasonFailed, err)
		r.mu.Unlock()
		r.deps.debug(fmt.Sprintf("[bridge:runner] Failed to decode work secret: %s", err))
		return fmt.Errorf("decode work secret: %w", err)
	}

	r.mu.Lock()
	r.workID = work.ID
	r.sessionID = work.Data.ID
	r.accessToken = secret.SessionIngressToken

	// Transition starting → running.
	if !r.transition(RunnerStarting, RunnerRunning) {
		// Somebody called Stop while we were starting.
		cur := r.state
		r.mu.Unlock()
		return fmt.Errorf("runner moved to %s during start", cur)
	}

	// Launch heartbeat goroutine.
	hbCtx, hbCancel := context.WithCancel(ctx)
	r.cancelHeartbeat = hbCancel
	r.mu.Unlock()

	r.deps.debug(fmt.Sprintf("[bridge:runner] Running session_id=%s work_id=%s", work.Data.ID, work.ID))

	go r.heartbeatLoop(hbCtx)
	return nil
}

// ---------------------------------------------------------------------------
// Stop — running/starting → stopping → done
// ---------------------------------------------------------------------------

// Stop initiates a graceful shutdown: stops the heartbeat goroutine, archives
// the session, and transitions to done. It is safe to call from any state;
// calling Stop on an already-done runner is a no-op.
func (r *SessionRunner) Stop(reason StopReason) {
	r.mu.Lock()

	switch r.state {
	case RunnerDone:
		r.mu.Unlock()
		return
	case RunnerStopping:
		// Already stopping — wait for completion.
		r.mu.Unlock()
		<-r.done
		return
	case RunnerIdle:
		r.transitionToDone(reason, nil)
		r.mu.Unlock()
		return
	default:
		// starting or running → stopping
		from := r.state
		r.state = RunnerStopping
		if r.deps.OnStateChange != nil {
			r.deps.OnStateChange(from, RunnerStopping)
		}
	}

	// Cancel heartbeat if running.
	if r.cancelHeartbeat != nil {
		r.cancelHeartbeat()
	}

	sessionID := r.sessionID
	r.mu.Unlock()

	r.deps.debug(fmt.Sprintf("[bridge:runner] Stopping reason=%s session_id=%s", reason, sessionID))

	// Best-effort archive.
	if sessionID != "" {
		if err := r.deps.API.ArchiveSession(sessionID); err != nil {
			r.deps.debug(fmt.Sprintf("[bridge:runner] Archive failed: %s", err))
		}
	}

	r.mu.Lock()
	r.transitionToDone(reason, nil)
	r.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Heartbeat goroutine
// ---------------------------------------------------------------------------

func (r *SessionRunner) heartbeatLoop(ctx context.Context) {
	interval := r.deps.heartbeatInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.deps.debug(fmt.Sprintf("[bridge:runner] Heartbeat started interval=%s", interval))

	for {
		select {
		case <-ctx.Done():
			r.deps.debug("[bridge:runner] Heartbeat context cancelled")
			return
		case <-ticker.C:
			r.mu.Lock()
			if r.state != RunnerRunning {
				r.mu.Unlock()
				return
			}
			envID := r.deps.EnvironmentID
			workID := r.workID
			token := r.accessToken
			r.mu.Unlock()

			resp, err := r.deps.API.HeartbeatWork(envID, workID, token)
			if err != nil {
				r.deps.debug(fmt.Sprintf("[bridge:runner] Heartbeat error: %s", err))
				// Recoverable — we'll retry on the next tick.
				continue
			}

			r.deps.debug(fmt.Sprintf("[bridge:runner] Heartbeat ok lease_extended=%v state=%s", resp.LeaseExtended, resp.State))

			if !resp.LeaseExtended {
				// Fatal: server declined to extend the lease.
				r.deps.debug("[bridge:runner] Lease not extended — stopping")
				if r.deps.OnFatalError != nil {
					r.deps.OnFatalError(fmt.Errorf("heartbeat lease not extended (state=%s)", resp.State))
				}
				go r.Stop(StopReasonLeaseExpiry)
				return
			}
		}
	}
}
