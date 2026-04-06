package remote

import (
	"encoding/json"
	"sync"
	"testing"
)

// Source: src/remote/RemoteSessionManager.ts

// mockWSClient implements WebSocketClient for testing.
type mockWSClient struct {
	mu            sync.Mutex
	connected     bool
	closed        bool
	reconnected   bool
	sentResponses []SDKControlResponse
	sentRequests  []map[string]any
}

func newMockWS() *mockWSClient {
	return &mockWSClient{}
}

func (m *mockWSClient) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	return nil
}

func (m *mockWSClient) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	m.closed = true
}

func (m *mockWSClient) Reconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconnected = true
}

func (m *mockWSClient) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *mockWSClient) SendControlResponse(resp SDKControlResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentResponses = append(m.sentResponses, resp)
	return nil
}

func (m *mockWSClient) SendControlRequest(inner map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentRequests = append(m.sentRequests, inner)
	return nil
}

func TestRemoteSessionManager_Connect(t *testing.T) {
	ws := newMockWS()
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{},
	)
	mgr.SetWebSocket(ws)

	if err := mgr.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !ws.connected {
		t.Error("WS should be connected")
	}
}

func TestRemoteSessionManager_Connect_NoWS(t *testing.T) {
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{},
	)
	if err := mgr.Connect(); err == nil {
		t.Error("expected error when no WS client")
	}
}

func TestRemoteSessionManager_PermissionRequestFlow(t *testing.T) {
	// Full round trip: receive permission request → respond
	ws := newMockWS()
	ws.connected = true

	var gotToolName, gotRequestID string
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{
			OnPermissionRequest: func(req SDKControlPermissionRequest, id string) {
				gotToolName = req.ToolName
				gotRequestID = id
			},
		},
	)
	mgr.SetWebSocket(ws)

	// Simulate incoming permission request.
	raw, _ := json.Marshal(SDKControlRequest{
		Type:      "control_request",
		RequestID: "req-1",
		Request: SDKControlRequestInner{
			Subtype: "can_use_tool",
			SDKControlPermissionRequest: SDKControlPermissionRequest{
				ToolName:  "Bash",
				ToolUseID: "tu-1",
				Input:     map[string]any{"command": "ls"},
			},
		},
	})
	mgr.HandleMessage(raw)

	if gotToolName != "Bash" {
		t.Errorf("tool = %q, want Bash", gotToolName)
	}
	if gotRequestID != "req-1" {
		t.Errorf("requestID = %q, want req-1", gotRequestID)
	}
	if mgr.PendingPermissionCount() != 1 {
		t.Errorf("pending = %d, want 1", mgr.PendingPermissionCount())
	}

	// Respond.
	err := mgr.RespondToPermissionRequest("req-1", AllowResponse(map[string]any{"command": "ls"}))
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}

	if mgr.PendingPermissionCount() != 0 {
		t.Errorf("pending = %d after respond, want 0", mgr.PendingPermissionCount())
	}

	ws.mu.Lock()
	if len(ws.sentResponses) != 1 {
		t.Fatalf("sent %d responses, want 1", len(ws.sentResponses))
	}
	resp := ws.sentResponses[0]
	ws.mu.Unlock()

	if resp.Response.Subtype != "success" {
		t.Errorf("subtype = %q", resp.Response.Subtype)
	}
	if resp.Response.RequestID != "req-1" {
		t.Errorf("request_id = %q", resp.Response.RequestID)
	}
	if resp.Response.Response["behavior"] != "allow" {
		t.Errorf("behavior = %v", resp.Response.Response["behavior"])
	}
}

func TestRemoteSessionManager_PermissionCancelled(t *testing.T) {
	var cancelledID, cancelledToolUseID string
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{
			OnPermissionRequest: func(req SDKControlPermissionRequest, id string) {},
			OnPermissionCancelled: func(id string, toolUseID string) {
				cancelledID = id
				cancelledToolUseID = toolUseID
			},
		},
	)
	ws := newMockWS()
	ws.connected = true
	mgr.SetWebSocket(ws)

	// Add a pending request first.
	reqRaw, _ := json.Marshal(SDKControlRequest{
		Type:      "control_request",
		RequestID: "req-2",
		Request: SDKControlRequestInner{
			Subtype: "can_use_tool",
			SDKControlPermissionRequest: SDKControlPermissionRequest{
				ToolName:  "Write",
				ToolUseID: "tu-2",
				Input:     map[string]any{},
			},
		},
	})
	mgr.HandleMessage(reqRaw)

	// Cancel it.
	cancelRaw, _ := json.Marshal(SDKControlCancelRequest{
		Type:      "control_cancel_request",
		RequestID: "req-2",
	})
	mgr.HandleMessage(cancelRaw)

	if cancelledID != "req-2" {
		t.Errorf("cancelled ID = %q", cancelledID)
	}
	if cancelledToolUseID != "tu-2" {
		t.Errorf("cancelled tool_use_id = %q", cancelledToolUseID)
	}
	if mgr.PendingPermissionCount() != 0 {
		t.Errorf("pending = %d, want 0", mgr.PendingPermissionCount())
	}
}

func TestRemoteSessionManager_SDKMessage(t *testing.T) {
	var gotMsg json.RawMessage
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{
			OnMessage: func(msg any) {
				gotMsg = msg.(json.RawMessage)
			},
		},
	)

	// An SDK message with an unrecognized type goes to OnMessage.
	raw := json.RawMessage(`{"type":"result","content":"hello"}`)
	mgr.HandleMessage(raw)

	if gotMsg == nil {
		t.Fatal("OnMessage was not called")
	}
}

func TestRemoteSessionManager_UnsupportedControlRequest(t *testing.T) {
	ws := newMockWS()
	ws.connected = true
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{},
	)
	mgr.SetWebSocket(ws)

	raw, _ := json.Marshal(SDKControlRequest{
		Type:      "control_request",
		RequestID: "req-unsupported",
		Request: SDKControlRequestInner{
			Subtype: "unknown_subtype",
		},
	})
	mgr.HandleMessage(raw)

	ws.mu.Lock()
	if len(ws.sentResponses) != 1 {
		t.Fatalf("expected 1 error response, got %d", len(ws.sentResponses))
	}
	resp := ws.sentResponses[0]
	ws.mu.Unlock()

	if resp.Response.Subtype != "error" {
		t.Errorf("subtype = %q, want error", resp.Response.Subtype)
	}
	if resp.Response.Error == "" {
		t.Error("error message should not be empty")
	}
}

func TestRemoteSessionManager_RespondToUnknownRequest(t *testing.T) {
	ws := newMockWS()
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{},
	)
	mgr.SetWebSocket(ws)

	err := mgr.RespondToPermissionRequest("nonexistent", DenyResponse("no"))
	if err == nil {
		t.Error("expected error for unknown request ID")
	}
}

func TestRemoteSessionManager_CancelSession(t *testing.T) {
	ws := newMockWS()
	ws.connected = true
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{},
	)
	mgr.SetWebSocket(ws)

	if err := mgr.CancelSession(); err != nil {
		t.Fatalf("CancelSession: %v", err)
	}

	ws.mu.Lock()
	if len(ws.sentRequests) != 1 {
		t.Fatalf("sent %d requests, want 1", len(ws.sentRequests))
	}
	if ws.sentRequests[0]["subtype"] != "interrupt" {
		t.Errorf("subtype = %v", ws.sentRequests[0]["subtype"])
	}
	ws.mu.Unlock()
}

func TestRemoteSessionManager_Disconnect(t *testing.T) {
	ws := newMockWS()
	ws.connected = true
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{
			OnPermissionRequest: func(req SDKControlPermissionRequest, id string) {},
		},
	)
	mgr.SetWebSocket(ws)

	// Add a pending request.
	raw, _ := json.Marshal(SDKControlRequest{
		Type:      "control_request",
		RequestID: "req-d",
		Request: SDKControlRequestInner{
			Subtype: "can_use_tool",
			SDKControlPermissionRequest: SDKControlPermissionRequest{
				ToolName: "X",
				Input:    map[string]any{},
			},
		},
	})
	mgr.HandleMessage(raw)

	mgr.Disconnect()

	if !ws.closed {
		t.Error("WS should be closed")
	}
	if mgr.PendingPermissionCount() != 0 {
		t.Error("pending requests should be cleared")
	}
}

func TestRemoteSessionManager_Reconnect(t *testing.T) {
	ws := newMockWS()
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{},
	)
	mgr.SetWebSocket(ws)

	mgr.Reconnect()

	if !ws.reconnected {
		t.Error("WS should have been reconnected")
	}
}

func TestRemoteSessionManager_GetSessionID(t *testing.T) {
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "test-id"},
		RemoteSessionCallbacks{},
	)
	if mgr.GetSessionID() != "test-id" {
		t.Errorf("session ID = %q", mgr.GetSessionID())
	}
}

func TestRemoteSessionManager_IsConnected(t *testing.T) {
	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{},
		RemoteSessionCallbacks{},
	)
	// No WS → not connected.
	if mgr.IsConnected() {
		t.Error("should not be connected without WS")
	}

	ws := newMockWS()
	mgr.SetWebSocket(ws)
	if mgr.IsConnected() {
		t.Error("should not be connected before Connect()")
	}

	ws.connected = true
	if !mgr.IsConnected() {
		t.Error("should be connected")
	}
}
