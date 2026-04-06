package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmaxmax/go-sse"
)

// ---------------------------------------------------------------------------
// Test: SSE event parsing — data: lines are delivered as JSON to onData
// ---------------------------------------------------------------------------

func TestV2Transport_SSEEventParsing(t *testing.T) {
	t.Parallel()

	// Three events, each a JSON object sent as SSE data: lines.
	events := []string{
		`{"type":"user_prompt","text":"hello"}`,
		`{"type":"control_request","id":"abc"}`,
		`{"type":"keepalive"}`,
	}

	sseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(200)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		for i, ev := range events {
			fmt.Fprintf(w, "id: %d\n", i+1)
			fmt.Fprintf(w, "data: %s\n\n", ev)
			flusher.Flush()
		}
		// Close stream after sending all events.
	}

	var initCalled atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", sseHandler)
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		initCalled.Store(true)
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	})
	// Accept delivery ACKs.
	mux.HandleFunc("/worker/events/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	var received []string
	var mu sync.Mutex
	done := make(chan struct{})

	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:        srv.URL,
		IngressToken:      "test-token",
		SessionID:         "sess-1",
		Epoch:             42,
		HeartbeatInterval: 1 * time.Hour, // don't fire during test
	})

	transport.SetOnData(func(data string) {
		mu.Lock()
		received = append(received, data)
		if len(received) == len(events) {
			close(done)
		}
		mu.Unlock()
	})

	transport.SetOnClose(func(code int) {
		// SSE stream closes after all events — that's expected.
	})

	transport.Connect()
	defer transport.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SSE events")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(received))
	}
	for i, want := range events {
		if received[i] != want {
			t.Errorf("event[%d]: got %q, want %q", i, received[i], want)
		}
	}

	// Verify sequence number tracking.
	if seq := transport.LastSequenceNum(); seq != 3 {
		t.Errorf("LastSequenceNum = %d, want 3", seq)
	}

	if !initCalled.Load() {
		t.Error("expected /worker/initialize to be called")
	}
}

// ---------------------------------------------------------------------------
// Test: Reconnection after disconnect — SSE exhaustion fires onClose(4092)
// ---------------------------------------------------------------------------

func TestV2Transport_ReconnectAfterDisconnect(t *testing.T) {
	t.Parallel()

	var connectCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", func(w http.ResponseWriter, r *http.Request) {
		count := connectCount.Add(1)
		if count <= 2 {
			// First two connections: send one event then close abruptly.
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(200)
			flusher := w.(http.Flusher)
			fmt.Fprintf(w, "id: %d\ndata: {\"n\":%d}\n\n", count, count)
			flusher.Flush()
			// Return to close the connection — triggers reconnect.
			return
		}
		// Third+ connection: return 500 to trigger permanent error (no more retries).
		w.WriteHeader(500)
	})
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/worker/events/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	var dataEvents []string
	var dataMu sync.Mutex
	closeCh := make(chan int, 1)

	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:        srv.URL,
		IngressToken:      "test-token",
		SessionID:         "sess-reconnect",
		Epoch:             1,
		HeartbeatInterval: 1 * time.Hour,
		SSEClient: &sse.Client{
			HTTPClient:        srv.Client(),
			ResponseValidator: sse.DefaultValidator,
			Backoff: sse.Backoff{
				InitialInterval: 10 * time.Millisecond,
				Multiplier:      1.1,
				Jitter:          0.1,
			},
		},
	})

	transport.SetOnData(func(data string) {
		dataMu.Lock()
		dataEvents = append(dataEvents, data)
		dataMu.Unlock()
	})

	transport.SetOnClose(func(code int) {
		closeCh <- code
	})

	transport.Connect()
	defer transport.Close()

	// Wait for close callback (SSE retries should exhaust after server returns 500).
	select {
	case code := <-closeCh:
		if code != CloseCodeSSEExhausted {
			t.Errorf("close code = %d, want %d (SSE exhausted)", code, CloseCodeSSEExhausted)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for SSE exhaustion close")
	}

	// Should have connected at least twice before the permanent error.
	if c := connectCount.Load(); c < 2 {
		t.Errorf("expected at least 2 SSE connections, got %d", c)
	}

	// Should have received events from the first two connections.
	dataMu.Lock()
	if len(dataEvents) < 1 {
		t.Error("expected at least 1 data event from reconnect attempts")
	}
	dataMu.Unlock()
}

// ---------------------------------------------------------------------------
// Test: Keep-alive (heartbeat) fires periodic POSTs to /worker/heartbeat
// ---------------------------------------------------------------------------

func TestV2Transport_HeartbeatKeepAlive(t *testing.T) {
	t.Parallel()

	var heartbeats atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		// Hold the connection open.
		<-r.Context().Done()
	})
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		// Verify the body contains worker_epoch.
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err == nil {
			if _, ok := payload["worker_epoch"]; ok {
				heartbeats.Add(1)
			}
		}
		w.WriteHeader(200)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:        srv.URL,
		IngressToken:      "test-token",
		SessionID:         "sess-hb",
		Epoch:             7,
		HeartbeatInterval: 50 * time.Millisecond, // fast for testing
	})

	transport.SetOnData(func(string) {})
	transport.SetOnClose(func(int) {})
	transport.Connect()

	// Wait for at least 3 heartbeats.
	deadline := time.After(5 * time.Second)
	for {
		if heartbeats.Load() >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out: only got %d heartbeats, want >= 3", heartbeats.Load())
		case <-time.After(10 * time.Millisecond):
		}
	}

	transport.Close()

	// After close, heartbeats should stop. Record count and verify no more arrive.
	countAtClose := heartbeats.Load()
	time.Sleep(200 * time.Millisecond)
	if heartbeats.Load() > countAtClose+1 {
		t.Error("heartbeats continued after Close()")
	}
}

// ---------------------------------------------------------------------------
// Test: Epoch mismatch (409) on write triggers close(4090)
// ---------------------------------------------------------------------------

func TestV2Transport_EpochMismatch409(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		<-r.Context().Done()
	})
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/worker/events", func(w http.ResponseWriter, r *http.Request) {
		// Return 409 Conflict to simulate epoch mismatch.
		w.WriteHeader(http.StatusConflict)
	})
	mux.HandleFunc("/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	closeCh := make(chan int, 1)
	connectedCh := make(chan struct{}, 1)

	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:        srv.URL,
		IngressToken:      "test-token",
		SessionID:         "sess-409",
		Epoch:             5,
		HeartbeatInterval: 1 * time.Hour,
	})

	transport.SetOnData(func(string) {})
	transport.SetOnClose(func(code int) {
		closeCh <- code
	})
	transport.SetOnConnect(func() {
		connectedCh <- struct{}{}
	})

	transport.Connect()
	defer transport.Close()

	// Wait for connection.
	select {
	case <-connectedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	// Write should fail with ErrEpochSuperseded.
	err := transport.WriteMessage(context.Background(), json.RawMessage(`{"type":"test"}`))
	if err == nil {
		t.Fatal("expected error from WriteMessage on 409")
	}
	if err != ErrEpochSuperseded {
		t.Errorf("error = %v, want ErrEpochSuperseded", err)
	}

	// onClose should fire with 4090.
	select {
	case code := <-closeCh:
		if code != CloseCodeEpochMismatch {
			t.Errorf("close code = %d, want %d", code, CloseCodeEpochMismatch)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for close callback")
	}
}

// ---------------------------------------------------------------------------
// Test: Init failure triggers close(4091)
// ---------------------------------------------------------------------------

func TestV2Transport_InitFailure(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		<-r.Context().Done()
	})
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal server error"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	closeCh := make(chan int, 1)

	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:        srv.URL,
		IngressToken:      "test-token",
		SessionID:         "sess-init-fail",
		Epoch:             1,
		HeartbeatInterval: 1 * time.Hour,
	})

	transport.SetOnData(func(string) {})
	transport.SetOnClose(func(code int) {
		closeCh <- code
	})

	transport.Connect()
	defer transport.Close()

	select {
	case code := <-closeCh:
		if code != CloseCodeInitFailure {
			t.Errorf("close code = %d, want %d", code, CloseCodeInitFailure)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for init failure close")
	}
}

// ---------------------------------------------------------------------------
// Test: V1 adapter — passthrough with no-op report methods
// ---------------------------------------------------------------------------

func TestV1Transport_Passthrough(t *testing.T) {
	t.Parallel()

	delegate := &mockV1Delegate{stateLabel: "connected"}
	transport := NewV1ReplTransport(delegate)

	// LastSequenceNum always 0 for v1.
	if seq := transport.LastSequenceNum(); seq != 0 {
		t.Errorf("v1 LastSequenceNum = %d, want 0", seq)
	}

	// DroppedBatchCount delegates.
	if cnt := transport.DroppedBatchCount(); cnt != 0 {
		t.Errorf("v1 DroppedBatchCount = %d, want 0", cnt)
	}

	// Report methods are no-ops (should not panic).
	transport.ReportState(SessionStateIdle)
	transport.ReportMetadata(map[string]any{"key": "val"})
	transport.ReportDelivery("evt-1", "processed")

	// Flush returns nil.
	if err := transport.Flush(); err != nil {
		t.Errorf("v1 Flush = %v, want nil", err)
	}

	// StateLabel delegates.
	if label := transport.StateLabel(); label != "connected" {
		t.Errorf("v1 StateLabel = %q, want %q", label, "connected")
	}
}

// ---------------------------------------------------------------------------
// Test: SSE URL derivation strips trailing slash
// ---------------------------------------------------------------------------

func TestV2Transport_SSEURLDerivation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		sessionURL string
		want       string
	}{
		{"https://api.example.com/v1/code/sessions/s1", "https://api.example.com/v1/code/sessions/s1/worker/events/stream"},
		{"https://api.example.com/v1/code/sessions/s1/", "https://api.example.com/v1/code/sessions/s1/worker/events/stream"},
	}

	for _, tc := range cases {
		tr := &v2ReplTransport{opts: V2TransportOpts{SessionURL: tc.sessionURL}}
		got := tr.sseURL()
		if got != tc.want {
			t.Errorf("sseURL(%q) = %q, want %q", tc.sessionURL, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: Auth headers — bearer token injection
// ---------------------------------------------------------------------------

func TestV2Transport_AuthHeaders(t *testing.T) {
	t.Parallel()

	// With IngressToken.
	tr1 := &v2ReplTransport{opts: V2TransportOpts{IngressToken: "tok-abc"}}
	h1 := tr1.authHeaders()
	if h1["Authorization"] != "Bearer tok-abc" {
		t.Errorf("authHeaders with IngressToken = %v", h1)
	}

	// With GetAuthToken override.
	tr2 := &v2ReplTransport{opts: V2TransportOpts{
		IngressToken: "should-not-use",
		GetAuthToken: func() string { return "override-tok" },
	}}
	h2 := tr2.authHeaders()
	if h2["Authorization"] != "Bearer override-tok" {
		t.Errorf("authHeaders with GetAuthToken = %v", h2)
	}

	// With empty GetAuthToken.
	tr3 := &v2ReplTransport{opts: V2TransportOpts{
		GetAuthToken: func() string { return "" },
	}}
	h3 := tr3.authHeaders()
	if h3 != nil {
		t.Errorf("authHeaders with empty GetAuthToken = %v, want nil", h3)
	}
}

// ---------------------------------------------------------------------------
// Test: WriteBatch stops on close
// ---------------------------------------------------------------------------

func TestV2Transport_WriteBatchStopsOnClose(t *testing.T) {
	t.Parallel()

	var postCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events", func(w http.ResponseWriter, r *http.Request) {
		postCount.Add(1)
		w.WriteHeader(200)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:        srv.URL,
		IngressToken:      "test-token",
		SessionID:         "sess-batch",
		Epoch:             1,
		OutboundOnly:      true,
		HeartbeatInterval: 1 * time.Hour,
	}).(*v2ReplTransport)

	// Mark as initialized to allow writes.
	transport.initialized.Store(true)

	// Close after first write to test early-exit in WriteBatch.
	msgs := make([]StdoutMessage, 5)
	for i := range msgs {
		msgs[i] = json.RawMessage(fmt.Sprintf(`{"i":%d}`, i))
	}

	// Write first message, then close.
	_ = transport.WriteMessage(context.Background(), msgs[0])
	transport.Close()

	// WriteBatch should not send all remaining messages.
	_ = transport.WriteBatch(context.Background(), msgs[1:])

	if postCount.Load() > 2 {
		t.Errorf("expected at most 2 POSTs (1 write + maybe 1 batch), got %d", postCount.Load())
	}
}

// ---------------------------------------------------------------------------
// Test: Close codes constants match TS source
// ---------------------------------------------------------------------------

func TestCloseCodes(t *testing.T) {
	t.Parallel()
	if CloseCodeEpochMismatch != 4090 {
		t.Errorf("CloseCodeEpochMismatch = %d, want 4090", CloseCodeEpochMismatch)
	}
	if CloseCodeInitFailure != 4091 {
		t.Errorf("CloseCodeInitFailure = %d, want 4091", CloseCodeInitFailure)
	}
	if CloseCodeSSEExhausted != 4092 {
		t.Errorf("CloseCodeSSEExhausted = %d, want 4092", CloseCodeSSEExhausted)
	}
}

// ---------------------------------------------------------------------------
// Test: StateLabel transitions
// ---------------------------------------------------------------------------

func TestV2Transport_StateLabel(t *testing.T) {
	t.Parallel()

	tr := &v2ReplTransport{
		opts:          V2TransportOpts{},
		heartbeatStop: make(chan struct{}),
	}

	if label := tr.StateLabel(); label != "connecting" {
		t.Errorf("initial state = %q, want %q", label, "connecting")
	}

	tr.initialized.Store(true)
	if label := tr.StateLabel(); label != "connected" {
		t.Errorf("after init = %q, want %q", label, "connected")
	}

	tr.closed.Store(true)
	if label := tr.StateLabel(); label != "closed" {
		t.Errorf("after close = %q, want %q", label, "closed")
	}
}

// ---------------------------------------------------------------------------
// Test: SSE Last-Event-ID resume header is set when initialSequenceNum > 0
// ---------------------------------------------------------------------------

func TestV2Transport_SSEResumeHeader(t *testing.T) {
	t.Parallel()

	var lastEventIDHeader atomic.Value
	headerCaptured := make(chan struct{}, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", func(w http.ResponseWriter, r *http.Request) {
		lastEventIDHeader.Store(r.Header.Get("Last-Event-ID"))
		select {
		case headerCaptured <- struct{}{}:
		default:
		}
		// Return 500 to stop reconnection attempts.
		w.WriteHeader(500)
	})
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:         srv.URL,
		IngressToken:       "test-token",
		SessionID:          "sess-resume",
		Epoch:              1,
		InitialSequenceNum: 42,
		HeartbeatInterval:  1 * time.Hour,
	})

	transport.SetOnData(func(string) {})
	transport.SetOnClose(func(int) {})

	transport.Connect()
	defer transport.Close()

	// Wait for the SSE request to arrive at the server.
	select {
	case <-headerCaptured:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SSE request")
	}

	if val, ok := lastEventIDHeader.Load().(string); !ok || val != "42" {
		t.Errorf("Last-Event-ID header = %q, want %q", val, "42")
	}
}

// ---------------------------------------------------------------------------
// Test: Outbound-only mode skips SSE connection
// ---------------------------------------------------------------------------

func TestV2Transport_OutboundOnly(t *testing.T) {
	t.Parallel()

	var sseCalled atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", func(w http.ResponseWriter, r *http.Request) {
		sseCalled.Store(true)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		<-r.Context().Done()
	})
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	connectedCh := make(chan struct{}, 1)
	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:        srv.URL,
		IngressToken:      "test-token",
		SessionID:         "sess-outbound",
		Epoch:             1,
		OutboundOnly:      true,
		HeartbeatInterval: 1 * time.Hour,
	})

	transport.SetOnData(func(string) {})
	transport.SetOnClose(func(int) {})
	transport.SetOnConnect(func() {
		connectedCh <- struct{}{}
	})

	transport.Connect()
	defer transport.Close()

	select {
	case <-connectedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect in outbound-only mode")
	}

	// SSE endpoint should not have been called.
	time.Sleep(100 * time.Millisecond)
	if sseCalled.Load() {
		t.Error("SSE endpoint was called in outbound-only mode")
	}
}

// ---------------------------------------------------------------------------
// Test: Debug log strings match TS source verbatim
// ---------------------------------------------------------------------------

func TestV2Transport_DebugLogStrings(t *testing.T) {
	t.Parallel()

	// Collect log output from transport lifecycle.
	var logs []string
	var logMu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("/worker/events/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		<-r.Context().Done()
	})
	mux.HandleFunc("/worker/initialize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	connectedCh := make(chan struct{}, 1)
	transport := NewV2ReplTransport(V2TransportOpts{
		SessionURL:        srv.URL,
		IngressToken:      "test-token",
		SessionID:         "sess-log",
		Epoch:             42,
		HeartbeatInterval: 1 * time.Hour,
		Logger: func(msg string) {
			logMu.Lock()
			logs = append(logs, msg)
			logMu.Unlock()
		},
	})

	transport.SetOnData(func(string) {})
	transport.SetOnClose(func(int) {})
	transport.SetOnConnect(func() {
		connectedCh <- struct{}{}
	})

	transport.Connect()
	defer transport.Close()

	select {
	case <-connectedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect")
	}

	logMu.Lock()
	defer logMu.Unlock()

	// Check for the "ready for writes" log.
	found := false
	for _, l := range logs {
		if strings.Contains(l, "[bridge:repl] v2 transport ready for writes (epoch=42, sse=") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'v2 transport ready for writes' log, got: %v", logs)
	}
}

// ---------------------------------------------------------------------------
// mockV1Delegate — stub for V1TransportDelegate
// ---------------------------------------------------------------------------

type mockV1Delegate struct {
	stateLabel string
}

func (m *mockV1Delegate) Write(msg StdoutMessage) error            { return nil }
func (m *mockV1Delegate) WriteBatch(msgs []StdoutMessage) error    { return nil }
func (m *mockV1Delegate) Close()                                   {}
func (m *mockV1Delegate) IsConnectedStatus() bool                  { return true }
func (m *mockV1Delegate) GetStateLabel() string                    { return m.stateLabel }
func (m *mockV1Delegate) SetOnData(cb func(data string))           {}
func (m *mockV1Delegate) SetOnClose(cb func(closeCode int))        {}
func (m *mockV1Delegate) SetOnConnect(cb func())                   {}
func (m *mockV1Delegate) Connect()                                 {}
func (m *mockV1Delegate) DroppedBatchCount() int64                 { return 0 }
