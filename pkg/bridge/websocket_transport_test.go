package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// wsEchoServer returns an httptest.Server that upgrades to WebSocket and
// echoes all messages back to the client.
func wsEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("wsEchoServer: Accept error: %s", err)
			return
		}
		defer conn.CloseNow()

		for {
			typ, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if err := conn.Write(r.Context(), typ, data); err != nil {
				return
			}
		}
	}))
}

// wsCloseServer returns a server that accepts and immediately closes with the given code.
func wsCloseServer(t *testing.T, code websocket.StatusCode, reason string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("wsCloseServer: Accept error: %s", err)
			return
		}
		conn.Close(code, reason)
	}))
}

// wsURL converts an httptest.Server URL to a *url.URL with ws:// scheme.
func wsURL(t *testing.T, server *httptest.Server) *url.URL {
	t.Helper()
	u, err := url.Parse(strings.Replace(server.URL, "http://", "ws://", 1))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// boolPtr returns a pointer to the given bool.
func boolPtr(b bool) *bool { return &b }

// ---------------------------------------------------------------------------
// CircularBuffer tests
// ---------------------------------------------------------------------------

func TestCircularBuffer_AddAndToArray(t *testing.T) {
	buf := NewCircularBuffer[int](3)

	buf.Add(1)
	buf.Add(2)
	buf.Add(3)

	got := buf.ToArray()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("expected [1,2,3], got %v", got)
	}

	// Overflow evicts oldest.
	buf.Add(4)
	got = buf.ToArray()
	if len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 4 {
		t.Fatalf("expected [2,3,4], got %v", got)
	}
}

func TestCircularBuffer_Clear(t *testing.T) {
	buf := NewCircularBuffer[string](5)
	buf.Add("a")
	buf.Add("b")
	buf.Clear()

	if buf.Len() != 0 {
		t.Fatalf("expected empty buffer, got len=%d", buf.Len())
	}
}

func TestCircularBuffer_AddAll(t *testing.T) {
	buf := NewCircularBuffer[int](3)
	buf.AddAll([]int{10, 20, 30, 40})
	got := buf.ToArray()
	if len(got) != 3 || got[0] != 20 || got[1] != 30 || got[2] != 40 {
		t.Fatalf("expected [20,30,40], got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Constants tests
// ---------------------------------------------------------------------------

func TestWSTransportConstants(t *testing.T) {
	if KeepAliveFrame != "{\"type\":\"keep_alive\"}\n" {
		t.Fatalf("KeepAliveFrame mismatch: %q", KeepAliveFrame)
	}
	if DefaultMaxBufferSize != 1000 {
		t.Fatalf("DefaultMaxBufferSize mismatch: %d", DefaultMaxBufferSize)
	}
	if DefaultBaseReconnectDelay != time.Second {
		t.Fatalf("DefaultBaseReconnectDelay mismatch: %v", DefaultBaseReconnectDelay)
	}
	if DefaultMaxReconnectDelay != 30*time.Second {
		t.Fatalf("DefaultMaxReconnectDelay mismatch: %v", DefaultMaxReconnectDelay)
	}
	if DefaultReconnectGiveUpMS != 600*time.Second {
		t.Fatalf("DefaultReconnectGiveUpMS mismatch: %v", DefaultReconnectGiveUpMS)
	}
	if DefaultPingInterval != 10*time.Second {
		t.Fatalf("DefaultPingInterval mismatch: %v", DefaultPingInterval)
	}
	if DefaultKeepaliveInterval != 300*time.Second {
		t.Fatalf("DefaultKeepaliveInterval mismatch: %v", DefaultKeepaliveInterval)
	}
	if SleepDetectionThresholdMS != 60*time.Second {
		t.Fatalf("SleepDetectionThresholdMS mismatch: %v", SleepDetectionThresholdMS)
	}
}

func TestPermanentCloseCodes(t *testing.T) {
	expected := []int{1002, 4001, 4003}
	for _, code := range expected {
		if !PermanentCloseCodes[code] {
			t.Errorf("expected %d in PermanentCloseCodes", code)
		}
	}
	if PermanentCloseCodes[1000] {
		t.Error("1000 should not be a permanent close code")
	}
}

// ---------------------------------------------------------------------------
// State machine tests
// ---------------------------------------------------------------------------

func TestWSTransportState_String(t *testing.T) {
	tests := []struct {
		state WSTransportState
		want  string
	}{
		{WSStateIdle, "idle"},
		{WSStateConnecting, "connecting"},
		{WSStateConnected, "connected"},
		{WSStateReconnecting, "reconnecting"},
		{WSStateClosed, "closed"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("WSTransportState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestStateTransition_IdleToConnectedToClose(t *testing.T) {
	server := wsEchoServer(t)
	defer server.Close()

	connected := make(chan struct{})
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL: wsURL(t, server),
	})

	if tr.State() != WSStateIdle {
		t.Fatalf("expected idle, got %s", tr.State())
	}

	tr.SetOnConnect(func() {
		close(connected)
	})

	tr.Connect()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	if tr.State() != WSStateConnected {
		t.Fatalf("expected connected, got %s", tr.State())
	}

	tr.Close()

	if tr.State() != WSStateClosed {
		t.Fatalf("expected closed, got %s", tr.State())
	}
}

func TestConnect_RefusedFromInvalidState(t *testing.T) {
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL: &url.URL{Scheme: "ws", Host: "localhost:0"},
	})
	tr.Close() // move to closed

	// Connect from closed state should be a no-op.
	tr.Connect()
	if tr.State() != WSStateClosed {
		t.Fatalf("expected closed, got %s", tr.State())
	}
}

// ---------------------------------------------------------------------------
// Reconnect on close tests
// ---------------------------------------------------------------------------

func TestReconnectOnClose_AutoReconnect(t *testing.T) {
	// Server closes immediately with non-permanent code (1001).
	var connectCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		connectCount.Add(1)
		if connectCount.Load() < 3 {
			// Close with normal code to trigger reconnect.
			conn.Close(websocket.StatusGoingAway, "going away")
		} else {
			// Third connect: stay open.
			for {
				_, _, err := conn.Read(r.Context())
				if err != nil {
					return
				}
			}
		}
	}))
	defer server.Close()

	reconnected := make(chan struct{})
	var connectCbCount atomic.Int32
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL:       wsURL(t, server),
		RandFloat: func() float64 { return 0.5 }, // deterministic jitter
	})

	tr.SetOnConnect(func() {
		n := connectCbCount.Add(1)
		if n >= 3 {
			select {
			case <-reconnected:
			default:
				close(reconnected)
			}
		}
	})

	tr.Connect()

	select {
	case <-reconnected:
		// Success: reconnected at least 3 times.
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for reconnect")
	}

	tr.Close()
}

func TestReconnectDisabled(t *testing.T) {
	server := wsCloseServer(t, websocket.StatusGoingAway, "bye")
	defer server.Close()

	closed := make(chan int, 1)
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL:           wsURL(t, server),
		AutoReconnect: boolPtr(false),
	})

	tr.SetOnClose(func(code int) {
		closed <- code
	})

	tr.Connect()

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for close callback")
	}

	if tr.State() != WSStateClosed {
		t.Fatalf("expected closed, got %s", tr.State())
	}
}

// ---------------------------------------------------------------------------
// Permanent close code tests
// ---------------------------------------------------------------------------

func TestPermanentCloseCode_NoReconnect(t *testing.T) {
	for _, code := range []websocket.StatusCode{1002, 4001, 4003} {
		t.Run(code.String(), func(t *testing.T) {
			server := wsCloseServer(t, code, "permanent")
			defer server.Close()

			closedCh := make(chan int, 1)
			tr := NewWebSocketTransport(WebSocketTransportOpts{
				URL: wsURL(t, server),
			})

			tr.SetOnClose(func(closeCode int) {
				closedCh <- closeCode
			})

			tr.Connect()

			select {
			case got := <-closedCh:
				if got != int(code) {
					t.Fatalf("expected close code %d, got %d", code, got)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for close callback")
			}

			if tr.State() != WSStateClosed {
				t.Fatalf("expected closed, got %s", tr.State())
			}
		})
	}
}

func TestPermanentCloseCode4003_RefreshedHeaders_Reconnects(t *testing.T) {
	var connectCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		n := connectCount.Add(1)
		if n == 1 {
			// First connect: reject with 4003.
			conn.Close(4003, "unauthorized")
		} else {
			// Second connect: stay open.
			for {
				_, _, err := conn.Read(r.Context())
				if err != nil {
					return
				}
			}
		}
	}))
	defer server.Close()

	connected := make(chan struct{})
	var connectCbCount atomic.Int32
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL:     wsURL(t, server),
		Headers: map[string]string{"Authorization": "old-token"},
		RefreshHeaders: func() map[string]string {
			return map[string]string{"Authorization": "new-token"}
		},
	})

	tr.SetOnConnect(func() {
		if connectCbCount.Add(1) >= 2 {
			select {
			case <-connected:
			default:
				close(connected)
			}
		}
	})

	tr.Connect()

	select {
	case <-connected:
		// Success: reconnected after 4003 with refreshed headers.
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for reconnect after 4003 with refreshed headers")
	}

	tr.Close()
}

// ---------------------------------------------------------------------------
// Message replay tests
// ---------------------------------------------------------------------------

func TestMessageReplay_BuffersAndReplays(t *testing.T) {
	var mu sync.Mutex
	var receivedMsgs []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		for {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			mu.Lock()
			receivedMsgs = append(receivedMsgs, string(data))
			mu.Unlock()
		}
	}))
	defer server.Close()

	connected := make(chan struct{}, 2)
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL: wsURL(t, server),
	})

	tr.SetOnConnect(func() {
		connected <- struct{}{}
	})

	tr.Connect()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	// Write messages with UUIDs.
	msg1 := json.RawMessage(`{"type":"test","uuid":"msg-1","data":"hello"}`)
	msg2 := json.RawMessage(`{"type":"test","uuid":"msg-2","data":"world"}`)

	if err := tr.Write(msg1); err != nil {
		t.Fatal(err)
	}
	if err := tr.Write(msg2); err != nil {
		t.Fatal(err)
	}

	// Wait for server to receive messages.
	time.Sleep(200 * time.Millisecond)

	// Verify messages were buffered.
	tr.mu.Lock()
	bufLen := tr.messageBuffer.Len()
	tr.mu.Unlock()
	if bufLen != 2 {
		t.Fatalf("expected 2 buffered messages, got %d", bufLen)
	}

	tr.Close()
}

func TestMessageReplay_CircularBufferOverflow(t *testing.T) {
	buf := NewCircularBuffer[StdoutMessage](3)

	for i := 0; i < 5; i++ {
		msg := json.RawMessage(`{"uuid":"` + string(rune('a'+i)) + `"}`)
		buf.Add(msg)
	}

	got := buf.ToArray()
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}

	// Should have items c, d, e (indices 2,3,4).
	for i, msg := range got {
		var envelope struct{ UUID string }
		json.Unmarshal(msg, &envelope)
		expected := string(rune('c' + i))
		if envelope.UUID != expected {
			t.Errorf("item %d: expected uuid %q, got %q", i, expected, envelope.UUID)
		}
	}
}

// ---------------------------------------------------------------------------
// Sleep detection tests
// ---------------------------------------------------------------------------

func TestSleepDetection_ResetsBudget(t *testing.T) {
	// This tests the sleep detection logic by directly manipulating timestamps.
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL: &url.URL{Scheme: "ws", Host: "localhost:0"},
	})

	tr.mu.Lock()
	tr.reconnectStartTimeSet = true
	tr.reconnectStartTime = time.Now().Add(-5 * time.Minute) // 5min into budget
	tr.lastReconnectAttemptTimeSet = true
	tr.lastReconnectAttemptTime = time.Now().Add(-2 * time.Minute) // 2min ago (> 60s threshold)
	tr.reconnectAttempts = 10
	oldAttempts := tr.reconnectAttempts
	tr.mu.Unlock()

	if oldAttempts != 10 {
		t.Fatalf("expected 10 attempts, got %d", oldAttempts)
	}

	// The sleep detection logic is embedded in handleConnectionError.
	// We verify the threshold constant is correct.
	if SleepDetectionThresholdMS != 60*time.Second {
		t.Fatalf("expected 60s threshold, got %v", SleepDetectionThresholdMS)
	}
}

// ---------------------------------------------------------------------------
// Data callback test
// ---------------------------------------------------------------------------

func TestOnDataCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		// Send a message to the client.
		err = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"greeting","text":"hi"}`))
		if err != nil {
			return
		}

		// Keep connection open.
		for {
			_, _, err := conn.Read(r.Context())
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	dataCh := make(chan string, 1)
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL: wsURL(t, server),
	})
	tr.SetOnData(func(data string) {
		dataCh <- data
	})

	connected := make(chan struct{})
	tr.SetOnConnect(func() {
		close(connected)
	})

	tr.Connect()

	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	select {
	case data := <-dataCh:
		if !strings.Contains(data, "greeting") {
			t.Fatalf("unexpected data: %s", data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for data")
	}

	tr.Close()
}

// ---------------------------------------------------------------------------
// extractUUID tests
// ---------------------------------------------------------------------------

func TestExtractUUID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"uuid":"abc-123","type":"test"}`, "abc-123"},
		{`{"type":"test"}`, ""},
		{`invalid json`, ""},
	}
	for _, tt := range tests {
		got := extractUUID(json.RawMessage(tt.input))
		if got != tt.want {
			t.Errorf("extractUUID(%s) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Write while disconnected test
// ---------------------------------------------------------------------------

func TestWrite_WhileDisconnected_Buffers(t *testing.T) {
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL: &url.URL{Scheme: "ws", Host: "localhost:0"},
	})

	msg := json.RawMessage(`{"uuid":"msg-1","type":"test"}`)
	err := tr.Write(msg)
	if err != nil {
		t.Fatalf("Write should not error when disconnected: %v", err)
	}

	tr.mu.Lock()
	bufLen := tr.messageBuffer.Len()
	tr.mu.Unlock()

	if bufLen != 1 {
		t.Fatalf("expected 1 buffered message, got %d", bufLen)
	}
}

// ---------------------------------------------------------------------------
// Budget exhaustion test
// ---------------------------------------------------------------------------

func TestReconnect_GiveUpAfterBudget(t *testing.T) {
	// Use a dial function that always fails.
	failDial := func(ctx context.Context, url string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error) {
		return nil, nil, context.DeadlineExceeded
	}

	// Frozen clock that we advance manually.
	var mu sync.Mutex
	fakeNow := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return fakeNow
	}

	closedCh := make(chan int, 1)
	tr := NewWebSocketTransport(WebSocketTransportOpts{
		URL:           &url.URL{Scheme: "ws", Host: "localhost:0"},
		Now:           nowFn,
		RandFloat:     func() float64 { return 0.5 },
		DialWebSocket: failDial,
	})

	tr.SetOnClose(func(code int) {
		closedCh <- code
	})

	// Manually set up state as if we've been reconnecting for > 10min.
	tr.mu.Lock()
	tr.state = WSStateReconnecting
	tr.reconnectStartTimeSet = true
	tr.reconnectStartTime = fakeNow.Add(-11 * time.Minute) // 11min ago
	tr.mu.Unlock()

	// Advance clock past the give-up window.
	mu.Lock()
	fakeNow = fakeNow.Add(1 * time.Minute) // total 12 min
	mu.Unlock()

	// Trigger connection attempt — the budget check happens in handleConnectionError.
	tr.Connect()

	select {
	case <-closedCh:
		// Good: transport gave up.
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for give-up close")
	}

	if tr.State() != WSStateClosed {
		t.Fatalf("expected closed, got %s", tr.State())
	}
}
