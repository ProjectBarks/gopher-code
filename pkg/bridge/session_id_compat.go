// Package bridge — session ID tag translation (cse_ ↔ session_).
// Source: src/bridge/sessionIdCompat.ts
package bridge

import (
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Prefix constants
// ---------------------------------------------------------------------------

const (
	// PrefixInfra is the infrastructure-layer session ID prefix.
	PrefixInfra = "cse_"

	// PrefixCompat is the client-facing compat session ID prefix.
	PrefixCompat = "session_"
)

// ---------------------------------------------------------------------------
// Kill-switch gate
// ---------------------------------------------------------------------------

var (
	cseShimMu   sync.RWMutex
	cseShimGate func() bool // nil = default-active
)

// SetCseShimGate registers the GrowthBook gate for the cse_ shim.
// Called from bridge init code. When the gate returns false the shim
// is disabled and ToCompatSessionID returns the id unchanged.
// A nil gate (or never calling SetCseShimGate) means the shim is active.
func SetCseShimGate(gate func() bool) {
	cseShimMu.Lock()
	defer cseShimMu.Unlock()
	cseShimGate = gate
}

// resetCseShimGate is for tests only.
func resetCseShimGate() {
	cseShimMu.Lock()
	defer cseShimMu.Unlock()
	cseShimGate = nil
}

// isShimActive returns true when the cse_ ↔ session_ shim should fire.
// Default-active: nil gate means active.
func isShimActive() bool {
	cseShimMu.RLock()
	defer cseShimMu.RUnlock()
	if cseShimGate != nil && !cseShimGate() {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// ToCompatSessionID / ToInfraSessionID
// ---------------------------------------------------------------------------

// ToCompatSessionID re-tags a cse_* session ID to session_* for the v1 compat API.
// No-op for IDs that don't start with "cse_". No-op when the shim gate is disabled.
func ToCompatSessionID(id string) string {
	if !strings.HasPrefix(id, PrefixInfra) {
		return id
	}
	if !isShimActive() {
		return id
	}
	return PrefixCompat + strings.TrimPrefix(id, PrefixInfra)
}

// ToInfraSessionID re-tags a session_* session ID to cse_* for infrastructure calls.
// No-op for IDs that don't start with "session_".
func ToInfraSessionID(id string) string {
	if !strings.HasPrefix(id, PrefixCompat) {
		return id
	}
	return PrefixInfra + strings.TrimPrefix(id, PrefixCompat)
}
