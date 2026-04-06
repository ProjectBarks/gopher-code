package bridge

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test: URL conversion
// ---------------------------------------------------------------------------

func TestConvertWsURLToPostURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "wss to https with /ws/ path",
			in:   "wss://api.example.com/v2/session_ingress/ws/sess-123",
			want: "https://api.example.com/v2/session_ingress/session/sess-123/events",
		},
		{
			name: "ws to http with /ws/ path",
			in:   "ws://localhost:8080/v2/session_ingress/ws/sess-456",
			want: "http://localhost:8080/v2/session_ingress/session/sess-456/events",
		},
		{
			name: "preserves query string",
			in:   "wss://api.example.com/v2/session_ingress/ws/sess-789?token=abc",
			want: "https://api.example.com/v2/session_ingress/session/sess-789/events?token=abc",
		},
		{
			name: "no /ws/ in path — appends /events only",
			in:   "wss://api.example.com/v2/session_ingress/sess-000",
			want: "https://api.example.com/v2/session_ingress/sess-000/events",
		},
		{
			name: "trailing slash before events",
			in:   "wss://api.example.com/v2/session_ingress/ws/sess-111/",
			want: "https://api.example.com/v2/session_ingress/session/sess-111/events",
		},
		{
			name: "path already ends with /events",
			in:   "wss://api.example.com/v2/session_ingress/ws/sess-222/events",
			want: "https://api.example.com/v2/session_ingress/session/sess-222/events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.in)
			if err != nil {
				t.Fatalf("bad test URL: %v", err)
			}
			got := ConvertWsURLToPostURL(u)
			if got != tt.want {
				t.Errorf("ConvertWsURLToPostURL(%q)\n  got  %q\n  want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: order preservation — non-stream write flushes buffered stream_events
// ---------------------------------------------------------------------------

func TestHybridTransport_OrderPreservation(t *testing.T) {
	var mu sync.Mutex
	var received []json.RawMessage

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Events []json.RawMessage `json:"events"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("bad POST body: %v", err)
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		received = append(received, payload.Events...)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Build a URL that maps to our test server.
	srvURL, _ := url.Parse("ws://" + srv.Listener.Addr().String() + "/v2/session_ingress/ws/test-sess")

	// Override the postURL to point to our test server.
	transport := NewHybridTransport(HybridTransportOpts{
		URL:          srvURL,
		GetAuthToken: func() string { return "test-token" },
		HTTPClient:   srv.Client(),
	})
	// Patch postURL to use http:// since we can't do ws:// for posts.
	transport.postURL = srv.URL + "/v2/session_ingress/session/test-sess/events"

	// Write stream_events (these get buffered).
	stream1 := json.RawMessage(`{"type":"stream_event","data":"chunk1"}`)
	stream2 := json.RawMessage(`{"type":"stream_event","data":"chunk2"}`)
	_ = transport.Write(stream1)
	_ = transport.Write(stream2)

	// Write a non-stream event — should flush stream_events first.
	nonStream := json.RawMessage(`{"type":"result","data":"done"}`)
	_ = transport.Write(nonStream)

	// Wait for drain.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(received))
	}

	// Verify order: stream1, stream2, nonStream.
	types := make([]string, len(received))
	for i, raw := range received {
		types[i] = extractMessageType(raw)
	}

	// The first two must be stream_event, the third must be result.
	if types[0] != "stream_event" || types[1] != "stream_event" || types[2] != "result" {
		t.Errorf("wrong order: got types %v, want [stream_event stream_event result]", types)
	}

	transport.Close()
}

// ---------------------------------------------------------------------------
// Test: batch size cap — more than maxBatchSize events are split
// ---------------------------------------------------------------------------

func TestHybridTransport_BatchSizeCap(t *testing.T) {
	var mu sync.Mutex
	var batchSizes []int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Events []json.RawMessage `json:"events"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		batchSizes = append(batchSizes, len(payload.Events))
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	srvURL, _ := url.Parse("ws://" + srv.Listener.Addr().String() + "/ws/test")

	transport := NewHybridTransport(HybridTransportOpts{
		URL:          srvURL,
		GetAuthToken: func() string { return "test-token" },
		HTTPClient:   srv.Client(),
	})
	transport.postURL = srv.URL + "/session/test/events"

	// Enqueue more than maxBatchSize events at once.
	total := maxBatchSize + 100
	msgs := make([]StdoutMessage, total)
	for i := range msgs {
		msgs[i] = json.RawMessage(`{"type":"result","i":` + string(rune('0'+i%10)) + `}`)
	}

	_ = transport.WriteBatch(msgs)

	// Wait for drain.
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(batchSizes) < 2 {
		t.Fatalf("expected at least 2 POSTs (total=%d > maxBatchSize=%d), got %d POSTs", total, maxBatchSize, len(batchSizes))
	}

	for _, sz := range batchSizes {
		if sz > maxBatchSize {
			t.Errorf("batch size %d exceeds cap %d", sz, maxBatchSize)
		}
	}

	totalSent := 0
	for _, sz := range batchSizes {
		totalSent += sz
	}
	if totalSent != total {
		t.Errorf("total events sent = %d, want %d", totalSent, total)
	}

	transport.Close()
}

// ---------------------------------------------------------------------------
// Test: onBatchDropped fires after max consecutive failures
// ---------------------------------------------------------------------------

func TestHybridTransport_OnBatchDropped(t *testing.T) {
	var mu sync.Mutex
	var droppedSize, droppedFailures int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return 500 — retryable failure.
		io.ReadAll(r.Body)
		w.WriteHeader(500)
	}))
	defer srv.Close()

	srvURL, _ := url.Parse("ws://" + srv.Listener.Addr().String() + "/ws/test")

	transport := NewHybridTransport(HybridTransportOpts{
		URL:                    srvURL,
		GetAuthToken:           func() string { return "test-token" },
		HTTPClient:             srv.Client(),
		MaxConsecutiveFailures: 2,
		OnBatchDropped: func(batchSize int, failures int) {
			mu.Lock()
			droppedSize = batchSize
			droppedFailures = failures
			mu.Unlock()
		},
	})
	transport.postURL = srv.URL + "/session/test/events"

	_ = transport.Write(json.RawMessage(`{"type":"result"}`))

	// Wait for retries + drop.
	time.Sleep(3 * time.Second)

	mu.Lock()
	defer mu.Unlock()

	if droppedSize == 0 {
		t.Error("onBatchDropped was never called")
	}
	if droppedFailures < 2 {
		t.Errorf("expected failures >= 2, got %d", droppedFailures)
	}
	if transport.DroppedBatchCount() < 1 {
		t.Errorf("DroppedBatchCount() = %d, want >= 1", transport.DroppedBatchCount())
	}

	transport.Close()
}

// ---------------------------------------------------------------------------
// Test: extractMessageType
// ---------------------------------------------------------------------------

func TestExtractMessageType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`{"type":"stream_event","data":"x"}`, "stream_event"},
		{`{"type":"result"}`, "result"},
		{`{"no_type":true}`, ""},
		{`invalid json`, ""},
	}
	for _, tt := range tests {
		got := extractMessageType(json.RawMessage(tt.in))
		if got != tt.want {
			t.Errorf("extractMessageType(%s) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: stream_event buffering — events coalesce within flush interval
// ---------------------------------------------------------------------------

func TestHybridTransport_StreamEventBuffering(t *testing.T) {
	var mu sync.Mutex
	var postCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		mu.Lock()
		postCount++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	srvURL, _ := url.Parse("ws://" + srv.Listener.Addr().String() + "/ws/test")

	transport := NewHybridTransport(HybridTransportOpts{
		URL:          srvURL,
		GetAuthToken: func() string { return "test-token" },
		HTTPClient:   srv.Client(),
	})
	transport.postURL = srv.URL + "/session/test/events"

	// Write multiple stream_events rapidly — they should coalesce into one POST.
	for i := 0; i < 10; i++ {
		_ = transport.Write(json.RawMessage(`{"type":"stream_event","i":` + string(rune('0'+i)) + `}`))
	}

	// Wait for the flush timer to fire (100ms) + drain.
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// All 10 stream_events should coalesce into a single POST (or very few).
	if postCount > 2 {
		t.Errorf("expected <= 2 POSTs for coalesced stream_events, got %d", postCount)
	}

	transport.Close()
}
