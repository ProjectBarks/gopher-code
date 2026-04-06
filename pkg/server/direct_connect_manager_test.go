package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// wsEchoServer creates an httptest server that upgrades to WebSocket and sends
// the provided JSONL lines as a single text frame, then reads back any client
// messages into receivedCh.
func wsEchoServer(t *testing.T, lines []string, receivedCh chan []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("ws accept: %v", err)
			return
		}
		defer conn.CloseNow()

		// Send all lines as a single text frame (JSONL)
		if len(lines) > 0 {
			payload := strings.Join(lines, "\n")
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(payload)); err != nil {
				t.Logf("ws write: %v", err)
				return
			}
		}

		// Read any messages the client sends back
		for {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if receivedCh != nil {
				receivedCh <- data
			}
		}
	}))
}

// wsAuthServer creates a server that checks the Authorization header and
// writes it back as a JSONL message.
func wsAuthServer(t *testing.T, gotAuthCh chan string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if gotAuthCh != nil {
			gotAuthCh <- auth
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		// Send a simple message so the client read loop has something to process
		msg := `{"type":"keep_alive"}`
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(msg))

		// Keep connection open briefly
		<-r.Context().Done()
	}))
}

// ---------------------------------------------------------------------------
// T96: DirectConnectSessionManager — connect / disconnect / IsConnected
// ---------------------------------------------------------------------------

func TestDirectConnectSessionManager_ConnectDisconnect(t *testing.T) {
	var connected, disconnected sync.WaitGroup
	connected.Add(1)
	disconnected.Add(1)

	srv := wsEchoServer(t, nil, nil)
	defer srv.Close()

	mgr := NewDirectConnectSessionManager(
		DirectConnectConfig{
			WSURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		},
		DirectConnectCallbacks{
			OnConnected: func() {
				connected.Done()
			},
			OnDisconnected: func() {
				disconnected.Done()
			},
		},
	)

	ctx := context.Background()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Wait for onConnected
	waitCh(t, &connected, 2*time.Second)

	if !mgr.IsConnected() {
		t.Error("expected IsConnected() == true after connect")
	}

	mgr.Disconnect()

	waitCh(t, &disconnected, 2*time.Second)

	if mgr.IsConnected() {
		t.Error("expected IsConnected() == false after disconnect")
	}
}

// ---------------------------------------------------------------------------
// T97: JSONL-over-WebSocket message parsing
// ---------------------------------------------------------------------------

func TestDirectConnectSessionManager_JSONLParsing(t *testing.T) {
	var msgMu sync.Mutex
	var messages []json.RawMessage

	lines := []string{
		`{"type":"assistant","content":"hello"}`,
		`{"type":"result","status":"ok"}`,
		`not valid json`,
		`{"type":"keep_alive"}`, // filtered
		``,                      // empty line
	}

	srv := wsEchoServer(t, lines, nil)
	defer srv.Close()

	var wg sync.WaitGroup
	wg.Add(2) // expect 2 SDK messages (assistant + result)

	mgr := NewDirectConnectSessionManager(
		DirectConnectConfig{
			WSURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		},
		DirectConnectCallbacks{
			OnMessage: func(msg json.RawMessage) {
				msgMu.Lock()
				messages = append(messages, msg)
				msgMu.Unlock()
				wg.Done()
			},
		},
	)

	ctx := context.Background()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer mgr.Disconnect()

	waitCh(t, &wg, 2*time.Second)

	msgMu.Lock()
	defer msgMu.Unlock()

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Verify the messages contain the expected types
	for _, raw := range messages {
		var env struct{ Type string }
		json.Unmarshal(raw, &env)
		if env.Type != "assistant" && env.Type != "result" {
			t.Errorf("unexpected message type: %s", env.Type)
		}
	}
}

func TestDirectConnectSessionManager_FilteredMessageTypes(t *testing.T) {
	// All of these types should be filtered out
	lines := []string{
		`{"type":"control_response","data":"x"}`,
		`{"type":"keep_alive"}`,
		`{"type":"control_cancel_request"}`,
		`{"type":"streamlined_text"}`,
		`{"type":"streamlined_tool_use_summary"}`,
		`{"type":"system","subtype":"post_turn_summary"}`,
	}

	srv := wsEchoServer(t, lines, nil)
	defer srv.Close()

	gotMessage := make(chan struct{}, 10)

	mgr := NewDirectConnectSessionManager(
		DirectConnectConfig{
			WSURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		},
		DirectConnectCallbacks{
			OnMessage: func(msg json.RawMessage) {
				gotMessage <- struct{}{}
			},
		},
	)

	ctx := context.Background()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer mgr.Disconnect()

	// Wait a bit — no messages should arrive
	select {
	case <-gotMessage:
		t.Error("received message that should have been filtered")
	case <-time.After(200 * time.Millisecond):
		// Good — no messages received
	}
}

func TestDirectConnectSessionManager_ControlRequest(t *testing.T) {
	lines := []string{
		`{"type":"control_request","request_id":"req-1","request":{"subtype":"can_use_tool","tool":"bash"}}`,
	}

	var gotReqID string
	var wg sync.WaitGroup
	wg.Add(1)

	srv := wsEchoServer(t, lines, nil)
	defer srv.Close()

	mgr := NewDirectConnectSessionManager(
		DirectConnectConfig{
			WSURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		},
		DirectConnectCallbacks{
			OnPermissionRequest: func(request json.RawMessage, requestID string) {
				gotReqID = requestID
				wg.Done()
			},
		},
	)

	ctx := context.Background()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer mgr.Disconnect()

	waitCh(t, &wg, 2*time.Second)

	if gotReqID != "req-1" {
		t.Errorf("requestID = %q, want %q", gotReqID, "req-1")
	}
}

// ---------------------------------------------------------------------------
// T98: Bearer auth header in WS connection
// ---------------------------------------------------------------------------

func TestDirectConnectSessionManager_BearerAuth(t *testing.T) {
	authCh := make(chan string, 1)
	srv := wsAuthServer(t, authCh)
	defer srv.Close()

	mgr := NewDirectConnectSessionManager(
		DirectConnectConfig{
			WSURL:     "ws" + strings.TrimPrefix(srv.URL, "http"),
			AuthToken: "my-secret-token",
		},
		DirectConnectCallbacks{},
	)

	ctx := context.Background()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer mgr.Disconnect()

	select {
	case auth := <-authCh:
		if auth != "Bearer my-secret-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer my-secret-token")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for auth header")
	}
}

func TestDirectConnectSessionManager_NoAuthWhenEmpty(t *testing.T) {
	authCh := make(chan string, 1)
	srv := wsAuthServer(t, authCh)
	defer srv.Close()

	mgr := NewDirectConnectSessionManager(
		DirectConnectConfig{
			WSURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		},
		DirectConnectCallbacks{},
	)

	ctx := context.Background()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer mgr.Disconnect()

	select {
	case auth := <-authCh:
		if auth != "" {
			t.Errorf("expected no Authorization header, got %q", auth)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for auth header check")
	}
}

// ---------------------------------------------------------------------------
// T96: SendMessage
// ---------------------------------------------------------------------------

func TestDirectConnectSessionManager_SendMessage(t *testing.T) {
	receivedCh := make(chan []byte, 1)
	srv := wsEchoServer(t, nil, receivedCh)
	defer srv.Close()

	mgr := NewDirectConnectSessionManager(
		DirectConnectConfig{
			WSURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		},
		DirectConnectCallbacks{},
	)

	ctx := context.Background()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer mgr.Disconnect()

	// Give the connection a moment to establish
	time.Sleep(50 * time.Millisecond)

	content := json.RawMessage(`[{"type":"text","text":"hello world"}]`)
	ok := mgr.SendMessage(ctx, content)
	if !ok {
		t.Fatal("SendMessage returned false")
	}

	select {
	case data := <-receivedCh:
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal sent message: %v", err)
		}
		if msg["type"] != "user" {
			t.Errorf("type = %v, want user", msg["type"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for sent message")
	}
}

func TestDirectConnectSessionManager_SendMessageWhenDisconnected(t *testing.T) {
	mgr := NewDirectConnectSessionManager(
		DirectConnectConfig{WSURL: "ws://localhost:0"},
		DirectConnectCallbacks{},
	)
	// Not connected — should return false
	ok := mgr.SendMessage(context.Background(), json.RawMessage(`"hello"`))
	if ok {
		t.Error("expected SendMessage to return false when not connected")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func waitCh(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal("timeout waiting for WaitGroup")
	}
}
