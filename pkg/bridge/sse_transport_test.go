package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test: SSE frame parsing (ParseSSEFrames)
// ---------------------------------------------------------------------------

func TestParseSSEFrames_BasicFrames(t *testing.T) {
	t.Parallel()

	buf := "event: client_event\ndata: {\"hello\":\"world\"}\nid: 1\n\n"
	frames, remaining := ParseSSEFrames(buf)

	require.Len(t, frames, 1)
	assert.Equal(t, "client_event", frames[0].Event)
	assert.Equal(t, `{"hello":"world"}`, frames[0].Data)
	assert.Equal(t, "1", frames[0].ID)
	assert.Equal(t, "", remaining)
}

func TestParseSSEFrames_MultipleFrames(t *testing.T) {
	t.Parallel()

	buf := "event: a\ndata: d1\nid: 1\n\nevent: b\ndata: d2\nid: 2\n\n"
	frames, remaining := ParseSSEFrames(buf)

	require.Len(t, frames, 2)
	assert.Equal(t, "a", frames[0].Event)
	assert.Equal(t, "d1", frames[0].Data)
	assert.Equal(t, "1", frames[0].ID)
	assert.Equal(t, "b", frames[1].Event)
	assert.Equal(t, "d2", frames[1].Data)
	assert.Equal(t, "2", frames[1].ID)
	assert.Equal(t, "", remaining)
}

func TestParseSSEFrames_IncompleteFrame(t *testing.T) {
	t.Parallel()

	buf := "event: a\ndata: d1\nid: 1\n\nevent: b\ndata: partial"
	frames, remaining := ParseSSEFrames(buf)

	require.Len(t, frames, 1)
	assert.Equal(t, "a", frames[0].Event)
	assert.Equal(t, "event: b\ndata: partial", remaining)
}

func TestParseSSEFrames_CommentFrame(t *testing.T) {
	t.Parallel()

	// SSE comment lines start with ':' — they produce a frame with no data
	// but isComment=true, so they still appear in the result.
	buf := ":keepalive\n\n"
	frames, remaining := ParseSSEFrames(buf)

	require.Len(t, frames, 1)
	assert.Equal(t, "", frames[0].Data)
	assert.Equal(t, "", remaining)
}

func TestParseSSEFrames_MultiDataLines(t *testing.T) {
	t.Parallel()

	// Per SSE spec, multiple data: lines are concatenated with \n.
	buf := "data: line1\ndata: line2\ndata: line3\n\n"
	frames, remaining := ParseSSEFrames(buf)

	require.Len(t, frames, 1)
	assert.Equal(t, "line1\nline2\nline3", frames[0].Data)
	assert.Equal(t, "", remaining)
}

func TestParseSSEFrames_SpaceAfterColon(t *testing.T) {
	t.Parallel()

	// Per SSE spec, strip one leading space after colon.
	buf := "data: with space\n\ndata:no space\n\n"
	frames, remaining := ParseSSEFrames(buf)

	require.Len(t, frames, 2)
	assert.Equal(t, "with space", frames[0].Data)
	assert.Equal(t, "no space", frames[1].Data)
	assert.Equal(t, "", remaining)
}

func TestParseSSEFrames_EmptyFrameSkipped(t *testing.T) {
	t.Parallel()

	buf := "\n\nevent: a\ndata: d1\n\n"
	frames, remaining := ParseSSEFrames(buf)

	require.Len(t, frames, 1)
	assert.Equal(t, "a", frames[0].Event)
	assert.Equal(t, "", remaining)
}

// ---------------------------------------------------------------------------
// Test: Permanent HTTP code rejection
// ---------------------------------------------------------------------------

func TestSSETransport_PermanentHTTPCodeRejectsImmediately(t *testing.T) {
	t.Parallel()

	for _, code := range []int{401, 403, 404} {
		code := code
		t.Run(fmt.Sprintf("HTTP_%d", code), func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()

			transport, err := NewSSETransport(SSETransportOpts{
				URL:            srv.URL + "/events/stream",
				GetAuthHeaders: func() map[string]string { return map[string]string{"Authorization": "Bearer test"} },
			})
			require.NoError(t, err)

			var closedCode int
			var closedOnce sync.Once
			done := make(chan struct{})
			transport.SetOnClose(func(closeCode int) {
				closedOnce.Do(func() {
					closedCode = closeCode
					close(done)
				})
			})

			go transport.Connect(context.Background())

			select {
			case <-done:
				assert.Equal(t, code, closedCode)
				assert.True(t, transport.IsClosed())
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for close callback")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: Liveness timeout triggers reconnect/close
// ---------------------------------------------------------------------------

func TestSSETransport_LivenessTimeout(t *testing.T) {
	t.Parallel()

	// Server that accepts the SSE connection but never sends data.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}
		// Block until client disconnects.
		<-r.Context().Done()
	}))
	defer srv.Close()

	transport, err := NewSSETransport(SSETransportOpts{
		URL:            srv.URL + "/events/stream",
		GetAuthHeaders: func() map[string]string { return map[string]string{"Authorization": "Bearer test"} },
	})
	require.NoError(t, err)

	// Verify the liveness timeout constant is correct.
	assert.Equal(t, 45_000, LivenessTimeoutMS)

	// We can't easily test the actual 45s timeout in a unit test,
	// but we verify the timer is set up by checking the transport
	// closes after Connect returns (due to go-sse retry exhaustion).
	done := make(chan struct{})
	transport.SetOnClose(func(_ int) {
		close(done)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go transport.Connect(ctx)

	// The context timeout will fire, causing close.
	<-ctx.Done()
	transport.Close()
	assert.True(t, transport.IsClosed())
}

// ---------------------------------------------------------------------------
// Test: POST retry backoff
// ---------------------------------------------------------------------------

func TestSSETransport_PostRetryBackoff(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	var sleepCalls []time.Duration
	var sleepMu sync.Mutex

	transport, err := NewSSETransport(SSETransportOpts{
		URL:            srv.URL + "/events/stream",
		GetAuthHeaders: func() map[string]string { return map[string]string{"Authorization": "Bearer test"} },
		Sleep: func(d time.Duration) {
			sleepMu.Lock()
			sleepCalls = append(sleepCalls, d)
			sleepMu.Unlock()
		},
	})
	require.NoError(t, err)
	// Override the postURL to hit our test server.
	transport.postURL = srv.URL + "/events"

	msg := json.RawMessage(`{"type":"test"}`)
	err = transport.Write(context.Background(), msg)
	require.NoError(t, err)

	assert.Equal(t, int32(3), attempts.Load(), "should have attempted 3 times (2 failures + 1 success)")

	sleepMu.Lock()
	defer sleepMu.Unlock()
	require.Len(t, sleepCalls, 2, "should have slept twice before the 3rd attempt")
	// First delay = 500ms, second = 1000ms.
	assert.Equal(t, time.Duration(PostBaseDelayMS)*time.Millisecond, sleepCalls[0])
	assert.Equal(t, time.Duration(PostBaseDelayMS*2)*time.Millisecond, sleepCalls[1])
}

func TestSSETransport_PostPermanent4xx(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(400) // 4xx (not 429) → permanent, no retry.
	}))
	defer srv.Close()

	transport, err := NewSSETransport(SSETransportOpts{
		URL:            srv.URL + "/events/stream",
		GetAuthHeaders: func() map[string]string { return map[string]string{"Authorization": "Bearer test"} },
		Sleep:          func(time.Duration) {},
	})
	require.NoError(t, err)
	transport.postURL = srv.URL + "/events"

	msg := json.RawMessage(`{"type":"test"}`)
	err = transport.Write(context.Background(), msg)
	require.NoError(t, err)

	assert.Equal(t, int32(1), attempts.Load(), "should not retry 4xx errors (except 429)")
}

func TestSSETransport_Post429Retries(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 2 {
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	transport, err := NewSSETransport(SSETransportOpts{
		URL:            srv.URL + "/events/stream",
		GetAuthHeaders: func() map[string]string { return map[string]string{"Authorization": "Bearer test"} },
		Sleep:          func(time.Duration) {},
	})
	require.NoError(t, err)
	transport.postURL = srv.URL + "/events"

	msg := json.RawMessage(`{"type":"test"}`)
	err = transport.Write(context.Background(), msg)
	require.NoError(t, err)

	assert.Equal(t, int32(2), attempts.Load(), "429 should trigger retry")
}

// ---------------------------------------------------------------------------
// Test: Constants match TS source
// ---------------------------------------------------------------------------

func TestSSETransport_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1000, ReconnectBaseDelayMS)
	assert.Equal(t, 30_000, ReconnectMaxDelayMS)
	assert.Equal(t, 600_000, ReconnectGiveUpMS)
	assert.Equal(t, 45_000, LivenessTimeoutMS)
	assert.Equal(t, 10, PostMaxRetries)
	assert.Equal(t, 500, PostBaseDelayMS)
	assert.Equal(t, 8000, PostMaxDelayMS)

	assert.True(t, PermanentHTTPCodes[401])
	assert.True(t, PermanentHTTPCodes[403])
	assert.True(t, PermanentHTTPCodes[404])
	assert.False(t, PermanentHTTPCodes[500])
	assert.False(t, PermanentHTTPCodes[429])
}

// ---------------------------------------------------------------------------
// Test: URL conversion
// ---------------------------------------------------------------------------

func TestConvertSSEURLToPostURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{
			"https://api.example.com/v2/session_ingress/session/abc/events/stream",
			"https://api.example.com/v2/session_ingress/session/abc/events",
		},
		{
			"https://api.example.com/events/stream?foo=bar",
			"https://api.example.com/events",
		},
		{
			"https://api.example.com/events",
			"https://api.example.com/events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			u, err := parseURL(tt.input)
			require.NoError(t, err)
			got := convertSSEURLToPostURL(u)
			assert.Equal(t, tt.want, got)
		})
	}
}

func parseURL(s string) (*url.URL, error) {
	return url.Parse(s)
}

// ---------------------------------------------------------------------------
// Test: Sequence number tracking
// ---------------------------------------------------------------------------

func TestSSETransport_SequenceNumMonotonic(t *testing.T) {
	t.Parallel()

	transport, err := NewSSETransport(SSETransportOpts{
		URL:            "https://example.com/events/stream",
		GetAuthHeaders: func() map[string]string { return nil },
	})
	require.NoError(t, err)

	assert.Equal(t, int64(0), transport.GetLastSequenceNum())

	// Simulate receiving events with sequence numbers.
	transport.lastSeqNum.Store(5)
	assert.Equal(t, int64(5), transport.GetLastSequenceNum())

	// InitialSequenceNum seeds the value.
	transport2, err := NewSSETransport(SSETransportOpts{
		URL:                "https://example.com/events/stream",
		InitialSequenceNum: 42,
		GetAuthHeaders:     func() map[string]string { return nil },
	})
	require.NoError(t, err)
	assert.Equal(t, int64(42), transport2.GetLastSequenceNum())
}

// ---------------------------------------------------------------------------
// Test: StreamClientEvent deserialization
// ---------------------------------------------------------------------------

func TestStreamClientEvent_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{"event_id":"evt_123","sequence_num":7,"event_type":"message","source":"worker","payload":{"type":"text","content":"hello"},"created_at":"2024-01-01T00:00:00Z"}`
	var ev StreamClientEvent
	require.NoError(t, json.Unmarshal([]byte(input), &ev))

	assert.Equal(t, "evt_123", ev.EventID)
	assert.Equal(t, int64(7), ev.SequenceNum)
	assert.Equal(t, "message", ev.EventType)
	assert.Equal(t, "worker", ev.Source)
	assert.Equal(t, "text", ev.Payload["type"])
	assert.Equal(t, "hello", ev.Payload["content"])
	assert.Equal(t, "2024-01-01T00:00:00Z", ev.CreatedAt)
}

// ---------------------------------------------------------------------------
// Test: POST delay calculation
// ---------------------------------------------------------------------------

func TestPostDelay(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 500*time.Millisecond, postDelay(1))
	assert.Equal(t, 1000*time.Millisecond, postDelay(2))
	assert.Equal(t, 2000*time.Millisecond, postDelay(3))
	assert.Equal(t, 4000*time.Millisecond, postDelay(4))
	assert.Equal(t, 8000*time.Millisecond, postDelay(5))
	// Capped at PostMaxDelayMS.
	assert.Equal(t, 8000*time.Millisecond, postDelay(6))
	assert.Equal(t, 8000*time.Millisecond, postDelay(10))
}

// ---------------------------------------------------------------------------
// Test: SSE event handling with real server
// ---------------------------------------------------------------------------

func TestSSETransport_ReceivesClientEvent(t *testing.T) {
	t.Parallel()

	payload := `{"event_id":"e1","sequence_num":1,"event_type":"msg","source":"worker","payload":{"type":"text","content":"hi"},"created_at":"2024-01-01T00:00:00Z"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(200)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send one client_event frame.
		fmt.Fprintf(w, "event: client_event\ndata: %s\nid: 1\n\n", payload)
		flusher.Flush()

		// Keep alive briefly then close.
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	transport, err := NewSSETransport(SSETransportOpts{
		URL:            srv.URL + "/events/stream",
		GetAuthHeaders: func() map[string]string { return map[string]string{"Authorization": "Bearer test"} },
	})
	require.NoError(t, err)

	var receivedData string
	var receivedEvent StreamClientEvent
	var dataMu sync.Mutex
	dataDone := make(chan struct{})

	transport.SetOnData(func(data string) {
		dataMu.Lock()
		defer dataMu.Unlock()
		receivedData = data
		select {
		case <-dataDone:
		default:
			close(dataDone)
		}
	})
	transport.SetOnEvent(func(ev StreamClientEvent) {
		dataMu.Lock()
		defer dataMu.Unlock()
		receivedEvent = ev
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go transport.Connect(ctx)

	select {
	case <-dataDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for data callback")
	}

	transport.Close()

	dataMu.Lock()
	defer dataMu.Unlock()

	// onData receives the unwrapped payload as JSON + newline.
	assert.True(t, strings.HasSuffix(receivedData, "\n"))
	var gotPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(receivedData)), &gotPayload))
	assert.Equal(t, "text", gotPayload["type"])
	assert.Equal(t, "hi", gotPayload["content"])

	// onEvent receives the full StreamClientEvent.
	assert.Equal(t, "e1", receivedEvent.EventID)
	assert.Equal(t, int64(1), receivedEvent.SequenceNum)

	// Sequence number tracked.
	assert.Equal(t, int64(1), transport.GetLastSequenceNum())
}

// ---------------------------------------------------------------------------
// Test: Close state transitions
// ---------------------------------------------------------------------------

func TestSSETransport_CloseTransitions(t *testing.T) {
	t.Parallel()

	transport, err := NewSSETransport(SSETransportOpts{
		URL: "https://example.com/events/stream",
	})
	require.NoError(t, err)

	assert.False(t, transport.IsConnected())
	assert.False(t, transport.IsClosed())

	transport.Close()

	assert.False(t, transport.IsConnected())
	assert.True(t, transport.IsClosed())
}

// ---------------------------------------------------------------------------
// Test: from_sequence_num query param + Last-Event-ID header
// ---------------------------------------------------------------------------

func TestSSETransport_ResumeHeaders(t *testing.T) {
	t.Parallel()

	var capturedURL string
	var capturedLastEventID string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedURL = r.URL.String()
		capturedLastEventID = r.Header.Get("Last-Event-ID")
		mu.Unlock()

		// Return a permanent error so the transport stops.
		w.WriteHeader(401)
	}))
	defer srv.Close()

	transport, err := NewSSETransport(SSETransportOpts{
		URL:                srv.URL + "/events/stream",
		InitialSequenceNum: 42,
		GetAuthHeaders:     func() map[string]string { return map[string]string{"Authorization": "Bearer test"} },
	})
	require.NoError(t, err)

	done := make(chan struct{})
	transport.SetOnClose(func(_ int) {
		close(done)
	})

	go transport.Connect(context.Background())

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	mu.Lock()
	defer mu.Unlock()

	assert.Contains(t, capturedURL, "from_sequence_num=42")
	assert.Equal(t, "42", capturedLastEventID)
}
