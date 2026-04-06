package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// ---------------------------------------------------------------------------
// T82: WebSocket client tests — integration via httptest WS server
// ---------------------------------------------------------------------------

// testWSServer creates an httptest server that upgrades to WebSocket,
// sends the given messages, then closes with the given code.
func testWSServer(t *testing.T, messages []string, closeCode websocket.StatusCode) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("accept error: %v", err)
			return
		}

		// Verify auth header.
		auth := r.Header.Get("Authorization")
		if auth == "" {
			conn.Close(websocket.StatusCode(4003), "unauthorized")
			return
		}

		for _, msg := range messages {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := conn.Write(ctx, websocket.MessageText, []byte(msg))
			cancel()
			if err != nil {
				return
			}
		}

		if closeCode != 0 {
			conn.Close(closeCode, "")
		} else {
			// Wait for client to close.
			for {
				_, _, err := conn.Read(context.Background())
				if err != nil {
					return
				}
			}
		}
	}))
}

func TestSessionsWebSocket_ConnectAndReceiveMessages(t *testing.T) {
	msgs := []string{
		`{"type":"system","subtype":"init","model":"claude-3","uuid":"u1","session_id":"s1"}`,
		`{"type":"assistant","uuid":"u2","session_id":"s1","message":{}}`,
	}
	// Use permanent close code so the client doesn't try to reconnect.
	srv := testWSServer(t, msgs, 4003)
	defer srv.Close()

	var mu sync.Mutex
	var received []json.RawMessage
	closed := make(chan struct{}, 1)

	ws := NewSessionsWebSocket(SessionsWebSocketOpts{
		SessionID:      "test-session",
		OrgUUID:        "org-1",
		GetAccessToken: func() string { return "test-token" },
		BaseAPIURL:     srv.URL,
		Callbacks: SessionsWSCallbacks{
			OnMessage: func(raw json.RawMessage) {
				mu.Lock()
				received = append(received, raw)
				mu.Unlock()
			},
			OnClose: func() {
				select {
				case closed <- struct{}{}:
				default:
				}
			},
		},
	})

	err := ws.Connect()
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Wait for permanent close (server sends messages then closes with 4003).
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for close")
	}

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 2 {
		t.Errorf("received %d messages, want 2", count)
	}
}

func TestSessionsWebSocket_IsConnected(t *testing.T) {
	// Server that stays open.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		// Echo loop — keeps connection alive.
		for {
			_, _, err := conn.Read(context.Background())
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	connected := make(chan struct{}, 1)
	ws := NewSessionsWebSocket(SessionsWebSocketOpts{
		SessionID:      "s1",
		OrgUUID:        "o1",
		GetAccessToken: func() string { return "tok" },
		BaseAPIURL:     srv.URL,
		Callbacks: SessionsWSCallbacks{
			OnConnected: func() {
				select {
				case connected <- struct{}{}:
				default:
				}
			},
		},
	})

	if ws.IsConnected() {
		t.Error("should not be connected before Connect()")
	}

	err := ws.Connect()
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	if !ws.IsConnected() {
		t.Error("should be connected after Connect()")
	}

	ws.Close()

	if ws.IsConnected() {
		t.Error("should not be connected after Close()")
	}
}

func TestSessionsWebSocket_SendControlResponse(t *testing.T) {
	var mu sync.Mutex
	var serverReceived []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		// Read one message from client.
		_, data, err := conn.Read(context.Background())
		if err != nil {
			return
		}
		mu.Lock()
		serverReceived = data
		mu.Unlock()
		// Keep alive briefly.
		time.Sleep(100 * time.Millisecond)
		conn.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	connected := make(chan struct{}, 1)
	ws := NewSessionsWebSocket(SessionsWebSocketOpts{
		SessionID:      "s1",
		OrgUUID:        "o1",
		GetAccessToken: func() string { return "tok" },
		BaseAPIURL:     srv.URL,
		Callbacks: SessionsWSCallbacks{
			OnConnected: func() {
				select {
				case connected <- struct{}{}:
				default:
				}
			},
		},
	})

	if err := ws.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer ws.Close()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	resp := SDKControlResponse{
		Type: "control_response",
		Response: SDKControlResponseInner{
			Subtype:   "success",
			RequestID: "req-1",
			Response:  map[string]any{"behavior": "allow"},
		},
	}
	if err := ws.SendControlResponse(resp); err != nil {
		t.Fatalf("SendControlResponse: %v", err)
	}

	// Give the server time to receive.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	data := serverReceived
	mu.Unlock()

	if data == nil {
		t.Fatal("server did not receive message")
	}

	var got SDKControlResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Response.RequestID != "req-1" {
		t.Errorf("request_id = %q", got.Response.RequestID)
	}
}

func TestSessionsWebSocket_SendControlRequest(t *testing.T) {
	var mu sync.Mutex
	var serverReceived []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		_, data, err := conn.Read(context.Background())
		if err != nil {
			return
		}
		mu.Lock()
		serverReceived = data
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		conn.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	connected := make(chan struct{}, 1)
	ws := NewSessionsWebSocket(SessionsWebSocketOpts{
		SessionID:      "s1",
		OrgUUID:        "o1",
		GetAccessToken: func() string { return "tok" },
		BaseAPIURL:     srv.URL,
		Callbacks: SessionsWSCallbacks{
			OnConnected: func() {
				select {
				case connected <- struct{}{}:
				default:
				}
			},
		},
	})

	if err := ws.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer ws.Close()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	if err := ws.SendControlRequest(map[string]any{"subtype": "interrupt"}); err != nil {
		t.Fatalf("SendControlRequest: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	data := serverReceived
	mu.Unlock()

	if data == nil {
		t.Fatal("server did not receive message")
	}

	var got SDKControlRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != "control_request" {
		t.Errorf("type = %q", got.Type)
	}
	if got.Request.Subtype != "interrupt" {
		t.Errorf("subtype = %q", got.Request.Subtype)
	}
	if got.RequestID == "" {
		t.Error("request_id should be generated")
	}
}

func TestSessionsWebSocket_PermanentCloseCode(t *testing.T) {
	// Server immediately closes with permanent code 4003.
	srv := testWSServer(t, nil, 4003)
	defer srv.Close()

	closeCh := make(chan struct{}, 1)
	reconnecting := false

	ws := NewSessionsWebSocket(SessionsWebSocketOpts{
		SessionID:      "s1",
		OrgUUID:        "o1",
		GetAccessToken: func() string { return "tok" },
		BaseAPIURL:     srv.URL,
		Callbacks: SessionsWSCallbacks{
			OnClose: func() {
				select {
				case closeCh <- struct{}{}:
				default:
				}
			},
			OnReconnecting: func() {
				reconnecting = true
			},
		},
	})

	_ = ws.Connect()

	select {
	case <-closeCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for close")
	}

	if reconnecting {
		t.Error("should not attempt reconnection for permanent close code")
	}
}

func TestSessionsWebSocket_SendWhenNotConnected(t *testing.T) {
	ws := NewSessionsWebSocket(SessionsWebSocketOpts{
		SessionID:      "s1",
		OrgUUID:        "o1",
		GetAccessToken: func() string { return "tok" },
		Callbacks:      SessionsWSCallbacks{},
	})

	err := ws.SendControlResponse(SDKControlResponse{})
	if err == nil {
		t.Error("expected error when sending while not connected")
	}

	err = ws.SendControlRequest(map[string]any{"subtype": "interrupt"})
	if err == nil {
		t.Error("expected error when sending while not connected")
	}
}

func TestSessionsWebSocket_ReplaceScheme(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://api.example.com", "wss://api.example.com"},
		{"http://localhost:8080", "ws://localhost:8080"},
		{"wss://already", "wss://already"},
	}
	for _, tt := range tests {
		got := replaceScheme(tt.in)
		if got != tt.want {
			t.Errorf("replaceScheme(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestSessionsWebSocket_IntegrationWithManager verifies the client satisfies
// the WebSocketClient interface and works with RemoteSessionManager.
func TestSessionsWebSocket_IntegrationWithManager(t *testing.T) {
	// Server sends a permission request, expects a response.
	permReq := SDKControlRequest{
		Type:      "control_request",
		RequestID: "req-perm",
		Request: SDKControlRequestInner{
			Subtype: "can_use_tool",
			SDKControlPermissionRequest: SDKControlPermissionRequest{
				ToolName:  "Bash",
				ToolUseID: "tu-1",
				Input:     map[string]any{"command": "ls"},
			},
		},
	}
	permReqJSON, _ := json.Marshal(permReq)

	var mu sync.Mutex
	var serverGotResponse []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}

		// Send permission request.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = conn.Write(ctx, websocket.MessageText, permReqJSON)
		cancel()
		if err != nil {
			return
		}

		// Read client response.
		_, data, err := conn.Read(context.Background())
		if err != nil {
			return
		}
		mu.Lock()
		serverGotResponse = data
		mu.Unlock()

		time.Sleep(100 * time.Millisecond)
		conn.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	permCh := make(chan string, 1)
	connectedCh := make(chan struct{}, 1)

	mgr := NewRemoteSessionManager(
		RemoteSessionConfig{SessionID: "s1"},
		RemoteSessionCallbacks{
			OnPermissionRequest: func(req SDKControlPermissionRequest, id string) {
				select {
				case permCh <- id:
				default:
				}
			},
			OnConnected: func() {
				select {
				case connectedCh <- struct{}{}:
				default:
				}
			},
		},
	)

	ws := NewSessionsWebSocket(SessionsWebSocketOpts{
		SessionID:      "s1",
		OrgUUID:        "o1",
		GetAccessToken: func() string { return "tok" },
		BaseAPIURL:     srv.URL,
		Callbacks: SessionsWSCallbacks{
			OnMessage: func(raw json.RawMessage) {
				mgr.HandleMessage(raw)
			},
			OnConnected: func() {
				if mgr.callbacks.OnConnected != nil {
					mgr.callbacks.OnConnected()
				}
			},
		},
	})

	// Wire the real WS client into the manager.
	mgr.SetWebSocket(ws)

	if err := mgr.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer mgr.Disconnect()

	// Wait for connected.
	select {
	case <-connectedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	// Wait for permission request.
	var requestID string
	select {
	case requestID = <-permCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for permission request")
	}

	if requestID != "req-perm" {
		t.Errorf("request ID = %q, want %q", requestID, "req-perm")
	}

	// Respond via manager.
	err := mgr.RespondToPermissionRequest(requestID, AllowResponse(map[string]any{"command": "ls"}))
	if err != nil {
		t.Fatalf("RespondToPermissionRequest: %v", err)
	}

	// Verify server received the response.
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	respData := serverGotResponse
	mu.Unlock()

	if respData == nil {
		t.Fatal("server did not receive permission response")
	}

	var resp SDKControlResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Response.RequestID != "req-perm" {
		t.Errorf("response request_id = %q", resp.Response.RequestID)
	}
	if resp.Response.Response["behavior"] != "allow" {
		t.Errorf("response behavior = %v", resp.Response.Response["behavior"])
	}
}
