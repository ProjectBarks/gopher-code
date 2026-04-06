// Package bridge — permission callback protocol for remote control sessions.
// Source: src/bridge/bridgePermissionCallbacks.ts
package bridge

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/projectbarks/gopher-code/pkg/permissions"
)

// ---------------------------------------------------------------------------
// Behavior enum
// ---------------------------------------------------------------------------

// PermissionBehavior is the allow/deny discriminant on a bridge permission response.
type PermissionBehavior string

const (
	BehaviorAllow PermissionBehavior = "allow"
	BehaviorDeny  PermissionBehavior = "deny"
)

// ---------------------------------------------------------------------------
// BridgePermissionResponse — wire type for control_response payloads
// ---------------------------------------------------------------------------

// BridgePermissionResponse is the response payload for a bridge permission request.
// Source: bridgePermissionCallbacks.ts:3-8
type BridgePermissionResponse struct {
	Behavior           PermissionBehavior          `json:"behavior"`
	UpdatedInput       map[string]any              `json:"updatedInput,omitempty"`
	UpdatedPermissions []permissions.PermissionUpdate `json:"updatedPermissions,omitempty"`
	Message            string                      `json:"message,omitempty"`
}

// ParseBridgePermissionResponse validates and decodes raw JSON into a
// BridgePermissionResponse. Returns an error if the payload is not a valid
// response (mirrors the TS isBridgePermissionResponse type predicate).
func ParseBridgePermissionResponse(raw []byte) (*BridgePermissionResponse, error) {
	var resp BridgePermissionResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("invalid permission response JSON: %w", err)
	}
	if resp.Behavior != BehaviorAllow && resp.Behavior != BehaviorDeny {
		return nil, fmt.Errorf("invalid permission response: behavior must be %q or %q, got %q", BehaviorAllow, BehaviorDeny, resp.Behavior)
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// BridgePermissionRequest — the outbound request sent to the web app
// ---------------------------------------------------------------------------

// BridgePermissionRequest carries a tool-permission prompt to the remote web app.
type BridgePermissionRequest struct {
	RequestID             string                         `json:"request_id"`
	ToolName              string                         `json:"tool_name"`
	Input                 map[string]any                 `json:"input"`
	ToolUseID             string                         `json:"tool_use_id"`
	Description           string                         `json:"description"`
	PermissionSuggestions []permissions.PermissionUpdate  `json:"permission_suggestions,omitempty"`
	BlockedPath           string                         `json:"blocked_path,omitempty"`
}

// ---------------------------------------------------------------------------
// PermissionCallbacks — per-requestId handler registry
// ---------------------------------------------------------------------------

// PermissionCallbacks manages the bridge permission request/response lifecycle.
// It maintains a per-requestId handler registry for subscribing to responses.
type PermissionCallbacks struct {
	mu       sync.Mutex
	handlers map[string]func(BridgePermissionResponse)

	// SendFunc is called when sendRequest is invoked. Wire this to the bridge API.
	SendFunc func(req BridgePermissionRequest) error
	// CancelFunc is called when cancelRequest is invoked.
	CancelFunc func(requestID string) error
}

// NewPermissionCallbacks creates a new PermissionCallbacks instance.
func NewPermissionCallbacks() *PermissionCallbacks {
	return &PermissionCallbacks{
		handlers: make(map[string]func(BridgePermissionResponse)),
	}
}

// SendRequest sends a permission request to the remote web app.
func (pc *PermissionCallbacks) SendRequest(req BridgePermissionRequest) error {
	if pc.SendFunc != nil {
		return pc.SendFunc(req)
	}
	return nil
}

// SendResponse dispatches a response to the registered handler for requestID.
func (pc *PermissionCallbacks) SendResponse(requestID string, resp BridgePermissionResponse) {
	pc.mu.Lock()
	h := pc.handlers[requestID]
	pc.mu.Unlock()
	if h != nil {
		h(resp)
	}
}

// CancelRequest cancels a pending permission request.
func (pc *PermissionCallbacks) CancelRequest(requestID string) error {
	pc.mu.Lock()
	delete(pc.handlers, requestID)
	pc.mu.Unlock()
	if pc.CancelFunc != nil {
		return pc.CancelFunc(requestID)
	}
	return nil
}

// OnResponse registers a handler for a specific requestID and returns an
// unsubscribe function. Only one handler per requestID is supported.
func (pc *PermissionCallbacks) OnResponse(requestID string, handler func(BridgePermissionResponse)) func() {
	pc.mu.Lock()
	pc.handlers[requestID] = handler
	pc.mu.Unlock()
	return func() {
		pc.mu.Lock()
		delete(pc.handlers, requestID)
		pc.mu.Unlock()
	}
}

// WaitForResponse registers a handler for requestID and blocks until a response
// is received or the timeout elapses. Returns nil on timeout.
func (pc *PermissionCallbacks) WaitForResponse(requestID string, timeout time.Duration) *BridgePermissionResponse {
	ch := make(chan BridgePermissionResponse, 1)
	unsub := pc.OnResponse(requestID, func(resp BridgePermissionResponse) {
		select {
		case ch <- resp:
		default:
		}
	})
	defer unsub()

	select {
	case resp := <-ch:
		return &resp
	case <-time.After(timeout):
		return nil
	}
}
