// Package remote — type definitions for remote CCR session management.
// Source: src/remote/RemoteSessionManager.ts
package remote

// ---------------------------------------------------------------------------
// T79: RemotePermissionResponse union
// Source: RemoteSessionManager.ts:40-49
// ---------------------------------------------------------------------------

// RemotePermissionResponse is the simplified permission result sent back to CCR.
// It is a tagged union discriminated by Behavior.
type RemotePermissionResponse struct {
	// Behavior is "allow" or "deny".
	Behavior string `json:"behavior"`
	// UpdatedInput is set when Behavior == "allow". The (possibly modified)
	// tool input to forward to the remote agent.
	UpdatedInput map[string]any `json:"updatedInput,omitempty"`
	// Message is set when Behavior == "deny". Human-readable denial reason.
	Message string `json:"message,omitempty"`
}

// AllowResponse creates an allow RemotePermissionResponse with the given input.
func AllowResponse(updatedInput map[string]any) RemotePermissionResponse {
	return RemotePermissionResponse{
		Behavior:     "allow",
		UpdatedInput: updatedInput,
	}
}

// DenyResponse creates a deny RemotePermissionResponse with the given message.
func DenyResponse(message string) RemotePermissionResponse {
	return RemotePermissionResponse{
		Behavior: "deny",
		Message:  message,
	}
}

// ---------------------------------------------------------------------------
// T80: RemoteSessionConfig + RemoteSessionCallbacks shapes
// Source: RemoteSessionManager.ts:50-85
// ---------------------------------------------------------------------------

// RemoteSessionConfig holds connection info for a remote CCR session.
// Source: RemoteSessionManager.ts:50-62
type RemoteSessionConfig struct {
	// SessionID is the unique remote session identifier.
	SessionID string
	// GetAccessToken returns the current OAuth access token.
	GetAccessToken func() string
	// OrgUUID is the organization UUID owning the session.
	OrgUUID string
	// HasInitialPrompt is true if the session was created with an initial
	// prompt that's already being processed on the remote.
	HasInitialPrompt bool
	// ViewerOnly when true makes this client a passive viewer. Ctrl+C/Escape
	// do NOT interrupt the remote agent; reconnect timeout is disabled;
	// session title is never updated. Used by `claude assistant`.
	ViewerOnly bool
}

// CreateRemoteSessionConfig is a convenience factory matching the TS export.
// Source: RemoteSessionManager.ts:329-343
func CreateRemoteSessionConfig(
	sessionID string,
	getAccessToken func() string,
	orgUUID string,
	hasInitialPrompt bool,
	viewerOnly bool,
) RemoteSessionConfig {
	return RemoteSessionConfig{
		SessionID:        sessionID,
		GetAccessToken:   getAccessToken,
		OrgUUID:          orgUUID,
		HasInitialPrompt: hasInitialPrompt,
		ViewerOnly:       viewerOnly,
	}
}

// RemoteSessionCallbacks defines event handlers for session lifecycle events.
// Source: RemoteSessionManager.ts:64-85
type RemoteSessionCallbacks struct {
	// OnMessage is called when an SDK message is received from the session.
	OnMessage func(msg any)
	// OnPermissionRequest is called when a permission request arrives from CCR.
	OnPermissionRequest func(request SDKControlPermissionRequest, requestID string)
	// OnPermissionCancelled is called when the server cancels a pending
	// permission request. toolUseID may be empty.
	OnPermissionCancelled func(requestID string, toolUseID string)
	// OnConnected is called when the WebSocket connection is established.
	OnConnected func()
	// OnDisconnected is called when the connection is lost and cannot be restored.
	OnDisconnected func()
	// OnReconnecting is called on transient WS drop while reconnect backoff is in progress.
	OnReconnecting func()
	// OnError is called on connection or protocol errors.
	OnError func(err error)
}

// ---------------------------------------------------------------------------
// SDK control protocol wire types
// Source: src/entrypoints/sdk/controlSchemas.ts
// ---------------------------------------------------------------------------

// SDKControlRequest is the wire envelope for control requests from CCR.
type SDKControlRequest struct {
	Type      string                     `json:"type"` // "control_request"
	RequestID string                     `json:"request_id"`
	Request   SDKControlRequestInner     `json:"request"`
}

// SDKControlRequestInner is the inner payload of a control request.
// Only the fields needed for permission routing are populated.
type SDKControlRequestInner struct {
	Subtype string `json:"subtype"` // "can_use_tool", "interrupt", etc.
	// Fields for can_use_tool:
	SDKControlPermissionRequest
}

// SDKControlResponse is the wire envelope for control responses to CCR.
type SDKControlResponse struct {
	Type     string                      `json:"type"` // "control_response"
	Response SDKControlResponseInner     `json:"response"`
}

// SDKControlResponseInner is the inner payload of a control response.
type SDKControlResponseInner struct {
	Subtype   string         `json:"subtype"`    // "success" or "error"
	RequestID string         `json:"request_id"`
	Response  map[string]any `json:"response,omitempty"` // for success
	Error     string         `json:"error,omitempty"`    // for error
}

// SDKControlCancelRequest is sent when the server cancels a pending control request.
type SDKControlCancelRequest struct {
	Type      string `json:"type"` // "control_cancel_request"
	RequestID string `json:"request_id"`
}
