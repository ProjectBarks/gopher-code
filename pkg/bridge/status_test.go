package bridge

import (
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// BridgeStatus enum values
// ---------------------------------------------------------------------------

func TestBridgeStatusStringValues(t *testing.T) {
	tests := []struct {
		status BridgeStatus
		want   string
	}{
		{StatusDisconnected, "disconnected"},
		{StatusConnecting, "connecting"},
		{StatusRegistering, "registering"},
		{StatusPolling, "polling"},
		{StatusRunning, "running"},
		{StatusError, "error"},
		{StatusStopping, "stopping"},
	}
	for _, tt := range tests {
		if got := string(tt.status); got != tt.want {
			t.Errorf("BridgeStatus = %q, want %q", got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// StatusState enum values (TUI display states from TS StatusState)
// ---------------------------------------------------------------------------

func TestStatusStateStringValues(t *testing.T) {
	tests := []struct {
		state StatusState
		want  string
	}{
		{StateIdle, "idle"},
		{StateAttached, "attached"},
		{StateTitled, "titled"},
		{StateReconnecting, "reconnecting"},
		{StateFailed, "failed"},
	}
	for _, tt := range tests {
		if got := string(tt.state); got != tt.want {
			t.Errorf("StatusState = %q, want %q", got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Valid transitions
// ---------------------------------------------------------------------------

func TestValidTransitions(t *testing.T) {
	// Each entry: from → to that should succeed.
	valid := []struct {
		from, to BridgeStatus
	}{
		// disconnected → connecting
		{StatusDisconnected, StatusConnecting},
		// connecting → registering, error
		{StatusConnecting, StatusRegistering},
		{StatusConnecting, StatusError},
		// registering → polling, error
		{StatusRegistering, StatusPolling},
		{StatusRegistering, StatusError},
		// polling → running, error, stopping
		{StatusPolling, StatusRunning},
		{StatusPolling, StatusError},
		{StatusPolling, StatusStopping},
		// running → polling, error, stopping
		{StatusRunning, StatusPolling},
		{StatusRunning, StatusError},
		{StatusRunning, StatusStopping},
		// error → connecting, disconnected
		{StatusError, StatusConnecting},
		{StatusError, StatusDisconnected},
		// stopping → disconnected
		{StatusStopping, StatusDisconnected},
	}

	for _, tt := range valid {
		sm := NewStatusMachine()
		sm.status = tt.from // seed directly for transition test
		if err := sm.Transition(tt.to); err != nil {
			t.Errorf("Transition(%s → %s) should be valid, got error: %v", tt.from, tt.to, err)
		}
		if sm.Status() != tt.to {
			t.Errorf("after Transition(%s → %s), Status() = %s", tt.from, tt.to, sm.Status())
		}
	}
}

// ---------------------------------------------------------------------------
// Illegal transitions
// ---------------------------------------------------------------------------

func TestIllegalTransitions(t *testing.T) {
	illegal := []struct {
		from, to BridgeStatus
	}{
		// disconnected can only go to connecting
		{StatusDisconnected, StatusPolling},
		{StatusDisconnected, StatusRunning},
		{StatusDisconnected, StatusRegistering},
		{StatusDisconnected, StatusError},
		{StatusDisconnected, StatusStopping},
		// connecting cannot go to polling, running, stopping, disconnected
		{StatusConnecting, StatusPolling},
		{StatusConnecting, StatusRunning},
		{StatusConnecting, StatusStopping},
		{StatusConnecting, StatusDisconnected},
		// registering cannot go to connecting, running, disconnected, stopping
		{StatusRegistering, StatusConnecting},
		{StatusRegistering, StatusRunning},
		{StatusRegistering, StatusDisconnected},
		{StatusRegistering, StatusStopping},
		// polling cannot go to connecting, registering, disconnected
		{StatusPolling, StatusConnecting},
		{StatusPolling, StatusRegistering},
		{StatusPolling, StatusDisconnected},
		// running cannot go to connecting, registering, disconnected
		{StatusRunning, StatusConnecting},
		{StatusRunning, StatusRegistering},
		{StatusRunning, StatusDisconnected},
		// error cannot go to polling, running, registering, stopping
		{StatusError, StatusPolling},
		{StatusError, StatusRunning},
		{StatusError, StatusRegistering},
		{StatusError, StatusStopping},
		// stopping cannot go anywhere except disconnected
		{StatusStopping, StatusConnecting},
		{StatusStopping, StatusRegistering},
		{StatusStopping, StatusPolling},
		{StatusStopping, StatusRunning},
		{StatusStopping, StatusError},
		{StatusStopping, StatusStopping},
	}

	for _, tt := range illegal {
		sm := NewStatusMachine()
		sm.status = tt.from
		if err := sm.Transition(tt.to); err == nil {
			t.Errorf("Transition(%s → %s) should be illegal, but succeeded", tt.from, tt.to)
		}
		// Status must not have changed.
		if sm.Status() != tt.from {
			t.Errorf("after illegal Transition(%s → %s), Status() = %s, want %s", tt.from, tt.to, sm.Status(), tt.from)
		}
	}
}

// ---------------------------------------------------------------------------
// Self-transitions are illegal
// ---------------------------------------------------------------------------

func TestSelfTransitionIsIllegal(t *testing.T) {
	for _, s := range []BridgeStatus{
		StatusDisconnected, StatusConnecting, StatusRegistering,
		StatusPolling, StatusRunning, StatusError, StatusStopping,
	} {
		sm := NewStatusMachine()
		sm.status = s
		if err := sm.Transition(s); err == nil {
			t.Errorf("self-transition %s → %s should be illegal", s, s)
		}
	}
}

// ---------------------------------------------------------------------------
// Display strings (BridgeStatusInfo)
// ---------------------------------------------------------------------------

func TestGetBridgeStatus(t *testing.T) {
	tests := []struct {
		name  string
		input BridgeConnectionState
		label string
		color StatusColor
	}{
		{
			name:  "error takes priority",
			input: BridgeConnectionState{Error: "timeout", Connected: true, SessionActive: true, Reconnecting: true},
			label: "Remote Control failed",
			color: ColorError,
		},
		{
			name:  "reconnecting second priority",
			input: BridgeConnectionState{Reconnecting: true, Connected: true},
			label: "Remote Control reconnecting",
			color: ColorWarning,
		},
		{
			name:  "sessionActive means active",
			input: BridgeConnectionState{SessionActive: true},
			label: "Remote Control active",
			color: ColorSuccess,
		},
		{
			name:  "connected means active",
			input: BridgeConnectionState{Connected: true},
			label: "Remote Control active",
			color: ColorSuccess,
		},
		{
			name:  "default is connecting",
			input: BridgeConnectionState{},
			label: "Remote Control connecting\u2026",
			color: ColorWarning,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetBridgeStatus(tt.input)
			if info.Label != tt.label {
				t.Errorf("Label = %q, want %q", info.Label, tt.label)
			}
			if info.Color != tt.color {
				t.Errorf("Color = %q, want %q", info.Color, tt.color)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// StatusColor string values
// ---------------------------------------------------------------------------

func TestStatusColorValues(t *testing.T) {
	if ColorError != "error" {
		t.Errorf("ColorError = %q", ColorError)
	}
	if ColorWarning != "warning" {
		t.Errorf("ColorWarning = %q", ColorWarning)
	}
	if ColorSuccess != "success" {
		t.Errorf("ColorSuccess = %q", ColorSuccess)
	}
}

// ---------------------------------------------------------------------------
// Display strings for BridgeStatus
// ---------------------------------------------------------------------------

func TestBridgeStatusDisplay(t *testing.T) {
	tests := []struct {
		status BridgeStatus
		want   string
	}{
		{StatusDisconnected, "Disconnected"},
		{StatusConnecting, "Connecting"},
		{StatusRegistering, "Registering"},
		{StatusPolling, "Polling"},
		{StatusRunning, "Running"},
		{StatusError, "Error"},
		{StatusStopping, "Stopping"},
	}
	for _, tt := range tests {
		if got := tt.status.Display(); got != tt.want {
			t.Errorf("%s.Display() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Footer and label constants
// ---------------------------------------------------------------------------

func TestFooterStrings(t *testing.T) {
	if FailedFooterText != "Something went wrong, please try again" {
		t.Errorf("FailedFooterText = %q", FailedFooterText)
	}

	idle := BuildIdleFooterText("https://example.com")
	if idle != "Code everywhere with the Claude app or https://example.com" {
		t.Errorf("BuildIdleFooterText = %q", idle)
	}

	active := BuildActiveFooterText("https://example.com")
	if active != "Continue coding in the Claude app or https://example.com" {
		t.Errorf("BuildActiveFooterText = %q", active)
	}
}

// ---------------------------------------------------------------------------
// Error status with reason
// ---------------------------------------------------------------------------

func TestErrorStatusWithReason(t *testing.T) {
	sm := NewStatusMachine()
	sm.status = StatusConnecting
	sm.TransitionToError("connection refused")
	if sm.Status() != StatusError {
		t.Fatalf("expected error status, got %s", sm.Status())
	}
	if sm.ErrorReason() != "connection refused" {
		t.Errorf("ErrorReason() = %q, want %q", sm.ErrorReason(), "connection refused")
	}
}

func TestErrorReasonClearedOnTransition(t *testing.T) {
	sm := NewStatusMachine()
	sm.status = StatusConnecting
	sm.TransitionToError("some error")
	// Now transition out of error
	if err := sm.Transition(StatusConnecting); err != nil {
		t.Fatal(err)
	}
	if sm.ErrorReason() != "" {
		t.Errorf("ErrorReason() should be empty after leaving error, got %q", sm.ErrorReason())
	}
}

// ---------------------------------------------------------------------------
// Status change callback
// ---------------------------------------------------------------------------

func TestOnStatusChange(t *testing.T) {
	sm := NewStatusMachine()
	var mu sync.Mutex
	var transitions []struct{ from, to BridgeStatus }

	sm.OnStatusChange(func(from, to BridgeStatus) {
		mu.Lock()
		transitions = append(transitions, struct{ from, to BridgeStatus }{from, to})
		mu.Unlock()
	})

	sm.Transition(StatusConnecting)
	sm.Transition(StatusRegistering)
	sm.Transition(StatusPolling)

	mu.Lock()
	defer mu.Unlock()
	if len(transitions) != 3 {
		t.Fatalf("expected 3 callbacks, got %d", len(transitions))
	}
	if transitions[0].from != StatusDisconnected || transitions[0].to != StatusConnecting {
		t.Errorf("transition[0] = %s→%s", transitions[0].from, transitions[0].to)
	}
	if transitions[1].from != StatusConnecting || transitions[1].to != StatusRegistering {
		t.Errorf("transition[1] = %s→%s", transitions[1].from, transitions[1].to)
	}
	if transitions[2].from != StatusRegistering || transitions[2].to != StatusPolling {
		t.Errorf("transition[2] = %s→%s", transitions[2].from, transitions[2].to)
	}
}

func TestCallbackNotCalledOnIllegalTransition(t *testing.T) {
	sm := NewStatusMachine()
	called := false
	sm.OnStatusChange(func(from, to BridgeStatus) {
		called = true
	})

	// disconnected → running is illegal
	sm.Transition(StatusRunning)
	if called {
		t.Error("callback should not fire on illegal transition")
	}
}

// ---------------------------------------------------------------------------
// NewStatusMachine starts in disconnected state
// ---------------------------------------------------------------------------

func TestNewStatusMachineStartsDisconnected(t *testing.T) {
	sm := NewStatusMachine()
	if sm.Status() != StatusDisconnected {
		t.Errorf("initial status = %s, want disconnected", sm.Status())
	}
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestStatusConstants(t *testing.T) {
	if ToolDisplayExpiryMS != 30000 {
		t.Errorf("ToolDisplayExpiryMS = %d", ToolDisplayExpiryMS)
	}
	if ShimmerIntervalMS != 150 {
		t.Errorf("ShimmerIntervalMS = %d", ShimmerIntervalMS)
	}
}
