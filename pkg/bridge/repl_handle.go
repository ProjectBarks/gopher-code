// Package bridge — global singleton pointer to the active ReplBridgeHandle.
// Source: src/bridge/replBridgeHandle.ts
package bridge

import "sync"

// ---------------------------------------------------------------------------
// Session bridge ID updater — injectable dependency
// ---------------------------------------------------------------------------

// SessionBridgeIDUpdater is called (fire-and-forget) when the handle is set or
// cleared, passing the compat session ID (or "" on clear). Consumers register
// via SetSessionBridgeIDUpdater; the default is a no-op.
type SessionBridgeIDUpdater func(compatID string)

var (
	bridgeIDUpdaterMu sync.RWMutex
	bridgeIDUpdater   SessionBridgeIDUpdater
)

// SetSessionBridgeIDUpdater registers the function that publishes (or clears)
// our bridge session ID in the concurrent-session record for local-peer dedup.
func SetSessionBridgeIDUpdater(fn SessionBridgeIDUpdater) {
	bridgeIDUpdaterMu.Lock()
	defer bridgeIDUpdaterMu.Unlock()
	bridgeIDUpdater = fn
}

func fireSessionBridgeIDUpdate(compatID string) {
	bridgeIDUpdaterMu.RLock()
	fn := bridgeIDUpdater
	bridgeIDUpdaterMu.RUnlock()
	if fn != nil {
		go func() { fn(compatID) }()
	}
}

// ---------------------------------------------------------------------------
// ReplBridgeHandle — simplified handle for out-of-tree callers
// ---------------------------------------------------------------------------

// ReplBridgeHandle wraps a ReplBridge and exposes a simplified interface for
// tools, slash commands, and other callers outside the REPL React-equivalent
// tree. It captures the session ID and access token that created the session,
// preventing staging/prod token divergence.
type ReplBridgeHandle struct {
	bridge *ReplBridge
}

// NewReplBridgeHandle creates a handle wrapping the given ReplBridge.
func NewReplBridgeHandle(rb *ReplBridge) *ReplBridgeHandle {
	return &ReplBridgeHandle{bridge: rb}
}

// Bridge returns the underlying ReplBridge.
func (h *ReplBridgeHandle) Bridge() *ReplBridge {
	return h.bridge
}

// BridgeSessionID returns the bridge session ID.
func (h *ReplBridgeHandle) BridgeSessionID() string {
	return h.bridge.SessionID()
}

// WriteMessages forwards outbound SDK messages to the bridge.
func (h *ReplBridgeHandle) WriteMessages(msgs []SDKMessage) {
	h.bridge.WriteMessages(msgs)
}

// SendControlResponse forwards a permission control_response to the bridge.
func (h *ReplBridgeHandle) SendControlResponse(resp SDKMessage) {
	h.bridge.SendControlResponse(resp)
}

// Close tears down the underlying bridge and clears the global handle.
func (h *ReplBridgeHandle) Close() {
	h.bridge.Teardown()
	SetReplBridgeHandle(nil)
}

// ---------------------------------------------------------------------------
// Global singleton — guarded by sync.RWMutex
// ---------------------------------------------------------------------------

var (
	handleMu sync.RWMutex
	handle   *ReplBridgeHandle
)

// SetReplBridgeHandle sets (or clears) the global REPL bridge handle.
// On set: publishes our bridge session ID in the concurrent-session record.
// On clear (nil): removes the session ID from the record.
// Fire-and-forget: errors from updateSessionBridgeId are swallowed.
func SetReplBridgeHandle(h *ReplBridgeHandle) {
	handleMu.Lock()
	handle = h
	handleMu.Unlock()

	// Publish (or clear) our bridge session ID so other local peers can
	// dedup us out of their bridge list — local is preferred.
	compatID := GetSelfBridgeCompatID()
	fireSessionBridgeIDUpdate(compatID)
}

// GetReplBridgeHandle returns the current global REPL bridge handle, or nil.
func GetReplBridgeHandle() *ReplBridgeHandle {
	handleMu.RLock()
	defer handleMu.RUnlock()
	return handle
}

// GetSelfBridgeCompatID returns our own bridge session ID in the session_*
// compat format the API returns in /v1/sessions responses, or "" if the
// bridge isn't connected.
func GetSelfBridgeCompatID() string {
	h := GetReplBridgeHandle()
	if h == nil {
		return ""
	}
	return ToCompatSessionID(h.BridgeSessionID())
}
