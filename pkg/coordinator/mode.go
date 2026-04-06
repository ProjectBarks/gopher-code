// Package coordinator implements coordinator-mode detection and session
// reconciliation. Coordinator mode enables Claude Code to orchestrate
// multiple worker agents for parallel task execution.
//
// Source: src/coordinator/coordinatorMode.ts
package coordinator

import (
	"os"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/analytics"
)

// SessionMode represents the stored mode of a session.
type SessionMode string

const (
	// SessionModeCoordinator indicates the session was started in coordinator mode.
	SessionModeCoordinator SessionMode = "coordinator"
	// SessionModeNormal indicates a normal (non-coordinator) session.
	SessionModeNormal SessionMode = "normal"
)

// coordinatorModeEnv is the environment variable that activates coordinator mode.
const coordinatorModeEnv = "CLAUDE_CODE_COORDINATOR_MODE"

// coordinatorModeGate is the feature gate name that must be enabled for
// coordinator mode to be available. When the gate is disabled, IsCoordinatorMode
// always returns false regardless of the env var.
// T15: COORDINATOR_MODE feature gate.
const coordinatorModeGate = "COORDINATOR_MODE"

// FeatureGateChecker is a function that checks whether a named feature gate
// is enabled. Injected at init time; defaults to disabled (returns false).
var FeatureGateChecker func(gate string) bool

// IsCoordinatorMode reports whether coordinator mode is active.
// Coordinator mode requires both:
//  1. The COORDINATOR_MODE feature gate is enabled
//  2. The CLAUDE_CODE_COORDINATOR_MODE env var is truthy (1, true, yes)
//
// Source: coordinatorMode.ts — isCoordinatorMode()
func IsCoordinatorMode() bool {
	if !isFeatureGateEnabled(coordinatorModeGate) {
		return false
	}
	return isEnvTruthy(os.Getenv(coordinatorModeEnv))
}

// MatchSessionMode reconciles the current coordinator mode with a resumed
// session's stored mode. If there is a mismatch, it flips the environment
// variable so that IsCoordinatorMode returns the correct value for the
// resumed session, logs an analytics event, and returns a user-visible
// warning message. Returns "" if no switch was needed.
//
// Source: coordinatorMode.ts — matchSessionMode()
func MatchSessionMode(sessionMode SessionMode) string {
	// No stored mode (old session before mode tracking) — do nothing.
	if sessionMode == "" {
		return ""
	}

	currentIsCoordinator := IsCoordinatorMode()
	sessionIsCoordinator := sessionMode == SessionModeCoordinator

	if currentIsCoordinator == sessionIsCoordinator {
		return ""
	}

	// Flip the env var — IsCoordinatorMode() reads it live, no caching.
	if sessionIsCoordinator {
		os.Setenv(coordinatorModeEnv, "1")
	} else {
		os.Unsetenv(coordinatorModeEnv)
	}

	// T18: tengu_coordinator_mode_switched analytics event.
	analytics.LogEvent("tengu_coordinator_mode_switched", analytics.EventMetadata{
		"to": string(sessionMode),
	})

	// T17: Verbatim enter/exit messages.
	if sessionIsCoordinator {
		return "Entered coordinator mode to match resumed session."
	}
	return "Exited coordinator mode to match resumed session."
}

// isFeatureGateEnabled checks the injected feature gate checker.
// Returns false if no checker has been set.
func isFeatureGateEnabled(gate string) bool {
	if FeatureGateChecker == nil {
		return false
	}
	return FeatureGateChecker(gate)
}

// isEnvTruthy checks if a string looks truthy (1, true, yes).
func isEnvTruthy(val string) bool {
	switch strings.ToLower(val) {
	case "1", "true", "yes":
		return true
	}
	return false
}
