// Package bridge — status.go implements the bridge status state machine,
// display labels/colors, and footer text helpers.
// Source: src/bridge/bridgeStatusUtil.ts
package bridge

import (
	"fmt"
	"sync"
)

// ---------------------------------------------------------------------------
// BridgeStatus — lifecycle state machine states
// ---------------------------------------------------------------------------

// BridgeStatus represents a bridge lifecycle state.
type BridgeStatus string

const (
	StatusDisconnected BridgeStatus = "disconnected"
	StatusConnecting   BridgeStatus = "connecting"
	StatusRegistering  BridgeStatus = "registering"
	StatusPolling      BridgeStatus = "polling"
	StatusRunning      BridgeStatus = "running"
	StatusError        BridgeStatus = "error"
	StatusStopping     BridgeStatus = "stopping"
)

// Display returns a human-readable TUI display string for the status.
func (s BridgeStatus) Display() string {
	switch s {
	case StatusDisconnected:
		return "Disconnected"
	case StatusConnecting:
		return "Connecting"
	case StatusRegistering:
		return "Registering"
	case StatusPolling:
		return "Polling"
	case StatusRunning:
		return "Running"
	case StatusError:
		return "Error"
	case StatusStopping:
		return "Stopping"
	default:
		return string(s)
	}
}

// validTransitions defines the legal state transitions.
var validTransitions = map[BridgeStatus]map[BridgeStatus]bool{
	StatusDisconnected: {StatusConnecting: true},
	StatusConnecting:   {StatusRegistering: true, StatusError: true},
	StatusRegistering:  {StatusPolling: true, StatusError: true},
	StatusPolling:      {StatusRunning: true, StatusError: true, StatusStopping: true},
	StatusRunning:      {StatusPolling: true, StatusError: true, StatusStopping: true},
	StatusError:        {StatusConnecting: true, StatusDisconnected: true},
	StatusStopping:     {StatusDisconnected: true},
}

// ---------------------------------------------------------------------------
// StatusState — TUI display states (from TS StatusState)
// ---------------------------------------------------------------------------

// StatusState represents a TUI display state for the bridge.
type StatusState string

const (
	StateIdle         StatusState = "idle"
	StateAttached     StatusState = "attached"
	StateTitled       StatusState = "titled"
	StateReconnecting StatusState = "reconnecting"
	StateFailed       StatusState = "failed"
)

// ---------------------------------------------------------------------------
// StatusColor — abstract color names for theme mapping
// ---------------------------------------------------------------------------

// StatusColor is an abstract color name mapped to theme colors downstream.
type StatusColor string

const (
	ColorError   StatusColor = "error"
	ColorWarning StatusColor = "warning"
	ColorSuccess StatusColor = "success"
)

// ---------------------------------------------------------------------------
// BridgeStatusInfo — display label + color derived from connection state
// ---------------------------------------------------------------------------

// BridgeStatusInfo holds a computed display label and color.
type BridgeStatusInfo struct {
	Label string
	Color StatusColor
}

// BridgeConnectionState is the input to GetBridgeStatus.
type BridgeConnectionState struct {
	Error         string
	Connected     bool
	SessionActive bool
	Reconnecting  bool
}

// GetBridgeStatus derives a status label and color from the bridge connection
// state. Priority: error > reconnecting > (sessionActive||connected) > connecting.
func GetBridgeStatus(s BridgeConnectionState) BridgeStatusInfo {
	if s.Error != "" {
		return BridgeStatusInfo{Label: "Remote Control failed", Color: ColorError}
	}
	if s.Reconnecting {
		return BridgeStatusInfo{Label: "Remote Control reconnecting", Color: ColorWarning}
	}
	if s.SessionActive || s.Connected {
		return BridgeStatusInfo{Label: "Remote Control active", Color: ColorSuccess}
	}
	return BridgeStatusInfo{Label: "Remote Control connecting\u2026", Color: ColorWarning}
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// ToolDisplayExpiryMS is how long a tool activity line stays visible (ms).
const ToolDisplayExpiryMS = 30_000

// ShimmerIntervalMS is the shimmer animation tick interval (ms).
const ShimmerIntervalMS = 150

// FailedFooterText is shown when the bridge has failed.
const FailedFooterText = "Something went wrong, please try again"

// BuildIdleFooterText returns footer text for the idle (ready) state.
func BuildIdleFooterText(url string) string {
	return "Code everywhere with the Claude app or " + url
}

// BuildActiveFooterText returns footer text for the active (connected) state.
func BuildActiveFooterText(url string) string {
	return "Continue coding in the Claude app or " + url
}

// ---------------------------------------------------------------------------
// StatusMachine — thread-safe state machine with transition validation
// ---------------------------------------------------------------------------

// StatusChangeFunc is called after a successful status transition.
type StatusChangeFunc func(from, to BridgeStatus)

// StatusMachine manages bridge lifecycle state with transition validation.
type StatusMachine struct {
	mu          sync.RWMutex
	status      BridgeStatus
	errorReason string
	onChange    StatusChangeFunc
}

// NewStatusMachine creates a StatusMachine starting in StatusDisconnected.
func NewStatusMachine() *StatusMachine {
	return &StatusMachine{status: StatusDisconnected}
}

// Status returns the current bridge status.
func (m *StatusMachine) Status() BridgeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// ErrorReason returns the error reason if status is StatusError, else "".
func (m *StatusMachine) ErrorReason() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errorReason
}

// OnStatusChange registers a callback fired after every successful transition.
// Only one callback is supported; subsequent calls replace the previous one.
func (m *StatusMachine) OnStatusChange(fn StatusChangeFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// Transition attempts to move from the current status to next. Returns an
// error if the transition is not legal. On success, clears errorReason (unless
// transitioning to error — use TransitionToError for that).
func (m *StatusMachine) Transition(next BridgeStatus) error {
	m.mu.Lock()
	from := m.status
	allowed, ok := validTransitions[from]
	if !ok || !allowed[next] {
		m.mu.Unlock()
		return fmt.Errorf("bridge: illegal transition %s → %s", from, next)
	}
	m.status = next
	if next != StatusError {
		m.errorReason = ""
	}
	cb := m.onChange
	m.mu.Unlock()

	if cb != nil {
		cb(from, next)
	}
	return nil
}

// TransitionToError transitions to StatusError with a reason string. It follows
// the same transition rules as Transition but also records the error reason.
func (m *StatusMachine) TransitionToError(reason string) error {
	m.mu.Lock()
	from := m.status
	allowed, ok := validTransitions[from]
	if !ok || !allowed[StatusError] {
		m.mu.Unlock()
		return fmt.Errorf("bridge: illegal transition %s → error", from)
	}
	m.status = StatusError
	m.errorReason = reason
	cb := m.onChange
	m.mu.Unlock()

	if cb != nil {
		cb(from, StatusError)
	}
	return nil
}
