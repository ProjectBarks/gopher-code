// Package remote — RemoteSessionManager manages a remote CCR session lifecycle.
// Source: src/remote/RemoteSessionManager.ts
package remote

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// ---------------------------------------------------------------------------
// T78: RemoteSessionManager
// Source: RemoteSessionManager.ts:95-324
// ---------------------------------------------------------------------------

// WebSocketClient is the interface that the session manager uses to
// communicate over the wire. This allows the real SessionsWebSocket (T86)
// to be injected later without circular dependencies.
type WebSocketClient interface {
	// Connect opens the connection. Blocks until handshake completes or fails.
	Connect() error
	// Close shuts down the connection.
	Close()
	// Reconnect forces a reconnect (e.g. after container shutdown).
	Reconnect()
	// IsConnected returns true if the WebSocket is healthy.
	IsConnected() bool
	// SendControlResponse sends a control response back to CCR.
	SendControlResponse(resp SDKControlResponse) error
	// SendControlRequest sends a control request to CCR (e.g. interrupt).
	SendControlRequest(inner map[string]any) error
}

// RemoteSessionManager coordinates a remote CCR session:
//   - WebSocket subscription for receiving messages from CCR
//   - HTTP POST for sending user messages to CCR
//   - Permission request/response flow
//
// Source: RemoteSessionManager.ts:95-324
type RemoteSessionManager struct {
	config    RemoteSessionConfig
	callbacks RemoteSessionCallbacks

	mu                        sync.Mutex
	ws                        WebSocketClient
	pendingPermissionRequests map[string]SDKControlPermissionRequest
}

// NewRemoteSessionManager creates a new manager. The WebSocketClient may be
// nil initially and set later via SetWebSocket.
func NewRemoteSessionManager(config RemoteSessionConfig, callbacks RemoteSessionCallbacks) *RemoteSessionManager {
	return &RemoteSessionManager{
		config:                    config,
		callbacks:                 callbacks,
		pendingPermissionRequests: make(map[string]SDKControlPermissionRequest),
	}
}

// SetWebSocket injects the WebSocket client. This is separate from the
// constructor because the WebSocket implementation (T86) may be created
// after the manager.
func (m *RemoteSessionManager) SetWebSocket(ws WebSocketClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ws = ws
}

// Connect opens the WebSocket connection to the remote session.
// Source: RemoteSessionManager.ts:108-141
func (m *RemoteSessionManager) Connect() error {
	m.mu.Lock()
	ws := m.ws
	m.mu.Unlock()

	if ws == nil {
		return fmt.Errorf("remote session manager: no WebSocket client configured")
	}

	slog.Debug("remote session manager: connecting",
		"session_id", m.config.SessionID)

	return ws.Connect()
}

// HandleMessage dispatches an incoming message from the WebSocket.
// Source: RemoteSessionManager.ts:146-184
func (m *RemoteSessionManager) HandleMessage(raw json.RawMessage) {
	// Peek at the "type" field to determine message kind.
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		slog.Error("remote session manager: failed to decode message type", "err", err)
		if m.callbacks.OnError != nil {
			m.callbacks.OnError(fmt.Errorf("decode message type: %w", err))
		}
		return
	}

	switch envelope.Type {
	case "control_request":
		var req SDKControlRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			slog.Error("remote session manager: failed to decode control request", "err", err)
			return
		}
		m.handleControlRequest(req)

	case "control_cancel_request":
		var cancel SDKControlCancelRequest
		if err := json.Unmarshal(raw, &cancel); err != nil {
			slog.Error("remote session manager: failed to decode control cancel", "err", err)
			return
		}
		m.handleControlCancelRequest(cancel)

	case "control_response":
		slog.Debug("remote session manager: received control response")

	default:
		// SDK message — forward to callback.
		if m.callbacks.OnMessage != nil {
			m.callbacks.OnMessage(raw)
		}
	}
}

// handleControlRequest processes permission and other control requests from CCR.
// Source: RemoteSessionManager.ts:189-214
func (m *RemoteSessionManager) handleControlRequest(req SDKControlRequest) {
	requestID := req.RequestID

	switch req.Request.Subtype {
	case "can_use_tool":
		slog.Debug("remote session manager: permission request",
			"tool", req.Request.ToolName, "request_id", requestID)

		m.mu.Lock()
		m.pendingPermissionRequests[requestID] = req.Request.SDKControlPermissionRequest
		m.mu.Unlock()

		if m.callbacks.OnPermissionRequest != nil {
			m.callbacks.OnPermissionRequest(req.Request.SDKControlPermissionRequest, requestID)
		}

	default:
		// Unsupported subtype — send error response so server doesn't hang.
		slog.Debug("remote session manager: unsupported control request",
			"subtype", req.Request.Subtype)

		errResp := SDKControlResponse{
			Type: "control_response",
			Response: SDKControlResponseInner{
				Subtype:   "error",
				RequestID: requestID,
				Error:     fmt.Sprintf("Unsupported control request subtype: %s", req.Request.Subtype),
			},
		}

		m.mu.Lock()
		ws := m.ws
		m.mu.Unlock()

		if ws != nil {
			if err := ws.SendControlResponse(errResp); err != nil {
				slog.Error("remote session manager: failed to send error response", "err", err)
			}
		}
	}
}

// handleControlCancelRequest processes cancellation of a pending permission.
// Source: RemoteSessionManager.ts:160-172
func (m *RemoteSessionManager) handleControlCancelRequest(cancel SDKControlCancelRequest) {
	requestID := cancel.RequestID

	m.mu.Lock()
	pending, ok := m.pendingPermissionRequests[requestID]
	delete(m.pendingPermissionRequests, requestID)
	m.mu.Unlock()

	slog.Debug("remote session manager: permission cancelled", "request_id", requestID)

	if m.callbacks.OnPermissionCancelled != nil {
		toolUseID := ""
		if ok {
			toolUseID = pending.ToolUseID
		}
		m.callbacks.OnPermissionCancelled(requestID, toolUseID)
	}
}

// RespondToPermissionRequest sends the user's permission decision back to CCR.
// Source: RemoteSessionManager.ts:247-282
func (m *RemoteSessionManager) RespondToPermissionRequest(requestID string, result RemotePermissionResponse) error {
	m.mu.Lock()
	_, ok := m.pendingPermissionRequests[requestID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("no pending permission request with ID: %s", requestID)
	}
	delete(m.pendingPermissionRequests, requestID)
	ws := m.ws
	m.mu.Unlock()

	// Build the response payload.
	respPayload := map[string]any{
		"behavior": result.Behavior,
	}
	if result.Behavior == "allow" {
		respPayload["updatedInput"] = result.UpdatedInput
	} else {
		respPayload["message"] = result.Message
	}

	resp := SDKControlResponse{
		Type: "control_response",
		Response: SDKControlResponseInner{
			Subtype:   "success",
			RequestID: requestID,
			Response:  respPayload,
		},
	}

	slog.Debug("remote session manager: sending permission response",
		"request_id", requestID, "behavior", result.Behavior)

	if ws == nil {
		return fmt.Errorf("remote session manager: no WebSocket client")
	}
	return ws.SendControlResponse(resp)
}

// IsConnected returns whether the WebSocket is currently healthy.
// Source: RemoteSessionManager.ts:287-289
func (m *RemoteSessionManager) IsConnected() bool {
	m.mu.Lock()
	ws := m.ws
	m.mu.Unlock()
	if ws == nil {
		return false
	}
	return ws.IsConnected()
}

// CancelSession sends an interrupt signal to the remote agent.
// Source: RemoteSessionManager.ts:294-297
func (m *RemoteSessionManager) CancelSession() error {
	m.mu.Lock()
	ws := m.ws
	m.mu.Unlock()

	if ws == nil {
		return fmt.Errorf("remote session manager: no WebSocket client")
	}

	slog.Debug("remote session manager: sending interrupt")
	return ws.SendControlRequest(map[string]any{"subtype": "interrupt"})
}

// GetSessionID returns the session ID.
// Source: RemoteSessionManager.ts:302-304
func (m *RemoteSessionManager) GetSessionID() string {
	return m.config.SessionID
}

// Disconnect closes the WebSocket and clears pending state.
// Source: RemoteSessionManager.ts:309-314
func (m *RemoteSessionManager) Disconnect() {
	m.mu.Lock()
	ws := m.ws
	m.pendingPermissionRequests = make(map[string]SDKControlPermissionRequest)
	m.mu.Unlock()

	if ws != nil {
		slog.Debug("remote session manager: disconnecting")
		ws.Close()
	}
}

// Reconnect forces a WebSocket reconnect.
// Source: RemoteSessionManager.ts:320-323
func (m *RemoteSessionManager) Reconnect() {
	m.mu.Lock()
	ws := m.ws
	m.mu.Unlock()

	if ws != nil {
		slog.Debug("remote session manager: reconnecting")
		ws.Reconnect()
	}
}

// PendingPermissionCount returns the number of pending permission requests.
// Useful for testing.
func (m *RemoteSessionManager) PendingPermissionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pendingPermissionRequests)
}
