// Package bridge — SSETransport: client-side SSE read + HTTP POST write transport.
// Source: src/cli/transports/SSETransport.ts
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmaxmax/go-sse"
)

// ---------------------------------------------------------------------------
// Configuration — verbatim match with TS SSETransport.ts constants
// ---------------------------------------------------------------------------

const (
	// ReconnectBaseDelayMS is the initial backoff delay for SSE reconnection.
	ReconnectBaseDelayMS = 1000
	// ReconnectMaxDelayMS is the maximum backoff delay for SSE reconnection.
	ReconnectMaxDelayMS = 30_000
	// ReconnectGiveUpMS is the total time budget before giving up (10 minutes).
	ReconnectGiveUpMS = 600_000
	// LivenessTimeoutMS is the silence threshold before treating connection as dead.
	// Server sends keepalives every 15s; treat as dead after 45s of silence.
	LivenessTimeoutMS = 45_000
	// PostMaxRetries is the maximum number of retry attempts for POST writes.
	PostMaxRetries = 10
	// PostBaseDelayMS is the initial backoff delay for POST retries.
	PostBaseDelayMS = 500
	// PostMaxDelayMS is the maximum backoff delay for POST retries.
	PostMaxDelayMS = 8000
)

// PermanentHTTPCodes are HTTP status codes that indicate permanent server-side
// rejection. The transport transitions to closed immediately without retrying.
var PermanentHTTPCodes = map[int]bool{
	401: true,
	403: true,
	404: true,
}

// ---------------------------------------------------------------------------
// StreamClientEvent — payload for event: client_event frames
// Source: StreamClientEvent type in SSETransport.ts
// ---------------------------------------------------------------------------

// StreamClientEvent is the payload for `event: client_event` SSE frames,
// matching the StreamClientEvent proto message in session_stream.proto.
type StreamClientEvent struct {
	EventID     string         `json:"event_id"`
	SequenceNum int64          `json:"sequence_num"`
	EventType   string         `json:"event_type"`
	Source      string         `json:"source"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   string         `json:"created_at"`
}

// ---------------------------------------------------------------------------
// SSETransportState
// ---------------------------------------------------------------------------

type sseTransportState int32

const (
	sseStateIdle sseTransportState = iota
	sseStateConnected
	sseStateReconnecting
	sseStateClosing
	sseStateClosed
)

func (s sseTransportState) String() string {
	switch s {
	case sseStateIdle:
		return "idle"
	case sseStateConnected:
		return "connected"
	case sseStateReconnecting:
		return "reconnecting"
	case sseStateClosing:
		return "closing"
	case sseStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// SSETransportOpts — constructor options
// ---------------------------------------------------------------------------

// SSETransportOpts configures an SSETransport.
type SSETransportOpts struct {
	// URL is the SSE stream endpoint (e.g. .../events/stream).
	URL string
	// Headers are additional headers sent on every SSE and POST request.
	Headers map[string]string
	// SessionID identifies the session (for debug logging).
	SessionID string
	// InitialSequenceNum seeds the resume cursor.
	InitialSequenceNum int64
	// GetAuthHeaders returns per-request auth headers.
	// If nil, no auth headers are added.
	GetAuthHeaders func() map[string]string
	// RefreshHeaders is called before each reconnect to refresh headers.
	RefreshHeaders func() map[string]string
	// HTTPClient is the HTTP client for POST writes. Defaults to http.DefaultClient.
	HTTPClient *http.Client
	// Logger receives debug log lines. Nil = discard.
	Logger func(msg string)
	// Now is a clock function for testing. Defaults to time.Now.
	Now func() time.Time
	// RandFloat returns a random float64 in [0,1) for jitter. Defaults to rand.Float64.
	RandFloat func() float64
	// Sleep is a testable sleep function. Defaults to time.Sleep.
	Sleep func(time.Duration)
}

// ---------------------------------------------------------------------------
// SSETransport — SSE read stream + HTTP POST write
// Source: class SSETransport in SSETransport.ts
// ---------------------------------------------------------------------------

// SSETransport implements a Transport that reads events via Server-Sent Events
// and writes events via HTTP POST with retry logic. Supports automatic
// reconnection with exponential backoff and Last-Event-ID for resumption.
type SSETransport struct {
	url     *url.URL
	postURL string
	opts    SSETransportOpts

	state atomic.Int32 // sseTransportState

	mu            sync.Mutex
	onDataCb      func(data string)
	onCloseCb     func(closeCode int)
	onEventCb     func(event StreamClientEvent)
	headers       map[string]string

	lastSeqNum atomic.Int64

	// Reconnection state (guarded by mu).
	reconnectAttempts   int
	reconnectStartTime  time.Time
	reconnectCancel     context.CancelFunc

	// Liveness timer (guarded by livenessMu).
	livenessMu    sync.Mutex
	livenessTimer *time.Timer

	// Connection cancel.
	cancelConn context.CancelFunc

	log       func(string)
	now       func() time.Time
	randFloat func() float64
	sleepFn   func(time.Duration)
}

// NewSSETransport creates a new SSETransport. Call Connect() to start reading.
func NewSSETransport(opts SSETransportOpts) (*SSETransport, error) {
	u, err := url.Parse(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid SSE URL: %w", err)
	}

	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.RandFloat == nil {
		opts.RandFloat = rand.Float64
	}
	if opts.Sleep == nil {
		opts.Sleep = time.Sleep
	}

	headers := make(map[string]string)
	for k, v := range opts.Headers {
		headers[k] = v
	}

	t := &SSETransport{
		url:       u,
		postURL:   convertSSEURLToPostURL(u),
		opts:      opts,
		headers:   headers,
		log:       opts.Logger,
		now:       opts.Now,
		randFloat: opts.RandFloat,
		sleepFn:   opts.Sleep,
	}

	t.state.Store(int32(sseStateIdle))

	if opts.InitialSequenceNum > 0 {
		t.lastSeqNum.Store(opts.InitialSequenceNum)
	}

	return t, nil
}

// GetLastSequenceNum returns the high-water mark of SSE event sequence numbers.
func (t *SSETransport) GetLastSequenceNum() int64 {
	return t.lastSeqNum.Load()
}

// IsConnected reports whether the transport is in the connected state.
func (t *SSETransport) IsConnected() bool {
	return sseTransportState(t.state.Load()) == sseStateConnected
}

// IsClosed reports whether the transport is in the closed state.
func (t *SSETransport) IsClosed() bool {
	return sseTransportState(t.state.Load()) == sseStateClosed
}

// SetOnData registers a callback for inbound data.
func (t *SSETransport) SetOnData(cb func(data string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDataCb = cb
}

// SetOnClose registers a callback fired when the transport closes.
func (t *SSETransport) SetOnClose(cb func(closeCode int)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onCloseCb = cb
}

// SetOnEvent registers a callback fired for each StreamClientEvent.
func (t *SSETransport) SetOnEvent(cb func(event StreamClientEvent)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onEventCb = cb
}

// ---------------------------------------------------------------------------
// Connect — opens the SSE read stream
// ---------------------------------------------------------------------------

// Connect opens the SSE read stream. It blocks until the connection is
// established or a permanent error occurs.
func (t *SSETransport) Connect(ctx context.Context) {
	st := sseTransportState(t.state.Load())
	if st != sseStateIdle && st != sseStateReconnecting {
		t.log(fmt.Sprintf("SSETransport: Cannot connect, current state is %s", st))
		return
	}

	t.state.Store(int32(sseStateReconnecting))
	t.connectOnce(ctx)
}

func (t *SSETransport) connectOnce(ctx context.Context) {
	// Build SSE URL with sequence number for resumption.
	sseURL := *t.url
	if seq := t.lastSeqNum.Load(); seq > 0 {
		q := sseURL.Query()
		q.Set("from_sequence_num", strconv.FormatInt(seq, 10))
		sseURL.RawQuery = q.Encode()
	}

	// Build headers.
	mergedHeaders := make(map[string]string)
	t.mu.Lock()
	for k, v := range t.headers {
		mergedHeaders[k] = v
	}
	t.mu.Unlock()

	if t.opts.GetAuthHeaders != nil {
		for k, v := range t.opts.GetAuthHeaders() {
			mergedHeaders[k] = v
		}
	}
	mergedHeaders["Accept"] = "text/event-stream"

	if seq := t.lastSeqNum.Load(); seq > 0 {
		mergedHeaders["Last-Event-ID"] = strconv.FormatInt(seq, 10)
	}

	// Create a custom ResponseValidator that detects permanent HTTP codes.
	validator := func(resp *http.Response) error {
		if PermanentHTTPCodes[resp.StatusCode] {
			return &permanentHTTPError{StatusCode: resp.StatusCode}
		}
		// Use default validation for everything else.
		return sse.DefaultValidator(resp)
	}

	client := &sse.Client{
		HTTPClient:        t.opts.HTTPClient,
		ResponseValidator: validator,
		Backoff: sse.Backoff{
			InitialInterval: time.Duration(ReconnectBaseDelayMS) * time.Millisecond,
			Multiplier:      2.0,
			Jitter:          0.25,
			MaxInterval:     time.Duration(ReconnectMaxDelayMS) * time.Millisecond,
			MaxElapsedTime:  time.Duration(ReconnectGiveUpMS) * time.Millisecond,
		},
	}

	connCtx, cancel := context.WithCancel(ctx)
	t.mu.Lock()
	t.cancelConn = cancel
	t.mu.Unlock()

	req, err := http.NewRequestWithContext(connCtx, http.MethodGet, sseURL.String(), nil)
	if err != nil {
		t.log(fmt.Sprintf("SSETransport: Request creation failed: %s", err))
		cancel()
		return
	}
	for k, v := range mergedHeaders {
		req.Header.Set(k, v)
	}

	conn := client.NewConnection(req)

	// Subscribe to all events.
	conn.SubscribeToAll(func(ev sse.Event) {
		t.handleEvent(ev)
	})

	t.state.Store(int32(sseStateConnected))
	t.resetLivenessTimer()

	// Connect blocks until ctx cancelled, permanent error, or retries exhausted.
	connErr := conn.Connect()

	t.clearLivenessTimer()

	if connCtx.Err() != nil {
		// Intentional close via Close().
		return
	}

	// Check if it was a permanent HTTP error.
	var permErr *permanentHTTPError
	var connError *sse.ConnectionError
	if errors.As(connErr, &connError) {
		if errors.As(connError.Err, &permErr) {
			t.state.Store(int32(sseStateClosed))
			t.mu.Lock()
			cb := t.onCloseCb
			t.mu.Unlock()
			if cb != nil {
				cb(permErr.StatusCode)
			}
			cancel()
			return
		}
	}
	if errors.As(connErr, &permErr) {
		t.state.Store(int32(sseStateClosed))
		t.mu.Lock()
		cb := t.onCloseCb
		t.mu.Unlock()
		if cb != nil {
			cb(permErr.StatusCode)
		}
		cancel()
		return
	}

	// Reconnect budget exhausted or other error.
	st := sseTransportState(t.state.Load())
	if st != sseStateClosing && st != sseStateClosed {
		t.log("SSETransport: Reconnection budget exhausted")
		t.state.Store(int32(sseStateClosed))
		t.mu.Lock()
		cb := t.onCloseCb
		t.mu.Unlock()
		if cb != nil {
			cb(0)
		}
	}
	cancel()
}

// handleEvent processes a single SSE event from go-sse.
func (t *SSETransport) handleEvent(ev sse.Event) {
	// Any event proves the connection is alive.
	t.resetLivenessTimer()

	// Track sequence number.
	if ev.LastEventID != "" {
		if seq, err := strconv.ParseInt(ev.LastEventID, 10, 64); err == nil {
			// Monotonic high-water mark.
			for {
				cur := t.lastSeqNum.Load()
				if seq <= cur {
					break
				}
				if t.lastSeqNum.CompareAndSwap(cur, seq) {
					break
				}
			}
		}
	}

	// Only process client_event frames.
	if ev.Type != "client_event" {
		if ev.Type != "" {
			t.log(fmt.Sprintf("SSETransport: Unexpected SSE event type '%s'", ev.Type))
		}
		return
	}

	var clientEvent StreamClientEvent
	if err := json.Unmarshal([]byte(ev.Data), &clientEvent); err != nil {
		t.log(fmt.Sprintf("SSETransport: Failed to parse client_event data: %s", err))
		return
	}

	// Pass unwrapped payload as newline-delimited JSON.
	payload := clientEvent.Payload
	if payload != nil {
		if _, hasType := payload["type"]; hasType {
			payloadJSON, err := json.Marshal(payload)
			if err == nil {
				t.mu.Lock()
				cb := t.onDataCb
				t.mu.Unlock()
				if cb != nil {
					cb(string(payloadJSON) + "\n")
				}
			}
		}
	}

	t.mu.Lock()
	cb := t.onEventCb
	t.mu.Unlock()
	if cb != nil {
		cb(clientEvent)
	}
}

// ---------------------------------------------------------------------------
// Liveness timer — 45s no-bytes → reconnect
// ---------------------------------------------------------------------------

func (t *SSETransport) resetLivenessTimer() {
	t.livenessMu.Lock()
	defer t.livenessMu.Unlock()

	if t.livenessTimer != nil {
		t.livenessTimer.Stop()
	}
	t.livenessTimer = time.AfterFunc(
		time.Duration(LivenessTimeoutMS)*time.Millisecond,
		t.onLivenessTimeout,
	)
}

func (t *SSETransport) clearLivenessTimer() {
	t.livenessMu.Lock()
	defer t.livenessMu.Unlock()

	if t.livenessTimer != nil {
		t.livenessTimer.Stop()
		t.livenessTimer = nil
	}
}

func (t *SSETransport) onLivenessTimeout() {
	t.log("SSETransport: Liveness timeout, reconnecting")

	t.mu.Lock()
	cancel := t.cancelConn
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

// Close tears down the SSETransport.
func (t *SSETransport) Close() {
	t.state.Store(int32(sseStateClosing))
	t.clearLivenessTimer()

	t.mu.Lock()
	cancel := t.cancelConn
	t.cancelConn = nil
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	t.state.Store(int32(sseStateClosed))
}

// ---------------------------------------------------------------------------
// Write (HTTP POST) — same retry pattern as TS SSETransport
// ---------------------------------------------------------------------------

// Write sends a message via HTTP POST with retry logic.
func (t *SSETransport) Write(ctx context.Context, message json.RawMessage) error {
	var authHeaders map[string]string
	if t.opts.GetAuthHeaders != nil {
		authHeaders = t.opts.GetAuthHeaders()
	}
	if len(authHeaders) == 0 {
		t.log("SSETransport: No session token available for POST")
		return nil
	}

	headers := make(map[string]string)
	for k, v := range authHeaders {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"

	for attempt := 1; attempt <= PostMaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.postURL, bytes.NewReader(message))
		if err != nil {
			return err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := t.opts.HTTPClient.Do(req)
		if err != nil {
			t.log(fmt.Sprintf("SSETransport: POST error: %s, attempt %d/%d", err, attempt, PostMaxRetries))
			if attempt == PostMaxRetries {
				t.log(fmt.Sprintf("SSETransport: POST failed after %d attempts", PostMaxRetries))
				return nil
			}
			delay := postDelay(attempt)
			t.sleepFn(delay)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 || resp.StatusCode == 201 {
			return nil
		}

		// 4xx (except 429) are permanent — don't retry.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			t.log(fmt.Sprintf("SSETransport: POST returned %d (client error), not retrying", resp.StatusCode))
			return nil
		}

		// 429 or 5xx — retry.
		t.log(fmt.Sprintf("SSETransport: POST returned %d, attempt %d/%d", resp.StatusCode, attempt, PostMaxRetries))

		if attempt == PostMaxRetries {
			t.log(fmt.Sprintf("SSETransport: POST failed after %d attempts", PostMaxRetries))
			return nil
		}

		delay := postDelay(attempt)
		t.sleepFn(delay)
	}

	return nil
}

// postDelay returns the backoff delay for a given POST attempt (1-indexed).
func postDelay(attempt int) time.Duration {
	ms := float64(PostBaseDelayMS) * math.Pow(2, float64(attempt-1))
	if ms > float64(PostMaxDelayMS) {
		ms = float64(PostMaxDelayMS)
	}
	return time.Duration(ms) * time.Millisecond
}

// ---------------------------------------------------------------------------
// permanentHTTPError — signals no-retry for permanent status codes
// ---------------------------------------------------------------------------

type permanentHTTPError struct {
	StatusCode int
}

func (e *permanentHTTPError) Error() string {
	return fmt.Sprintf("permanent HTTP %d", e.StatusCode)
}

// Temporary returns false, signalling go-sse to not retry.
func (e *permanentHTTPError) Temporary() bool { return false }

// ---------------------------------------------------------------------------
// URL conversion — SSE stream URL → POST events URL
// ---------------------------------------------------------------------------

// convertSSEURLToPostURL strips /stream from the SSE URL to get the POST endpoint.
// From: .../events/stream → To: .../events
func convertSSEURLToPostURL(sseURL *url.URL) string {
	u := *sseURL
	u.RawQuery = ""
	if strings.HasSuffix(u.Path, "/stream") {
		u.Path = u.Path[:len(u.Path)-len("/stream")]
	}
	return u.String()
}

// ---------------------------------------------------------------------------
// ParseSSEFrames — exported for testing, matches TS parseSSEFrames
// ---------------------------------------------------------------------------

// SSEFrame represents a single parsed SSE frame.
type SSEFrame struct {
	Event string
	ID    string
	Data  string
}

// ParseSSEFrames incrementally parses SSE frames from a text buffer.
// Returns parsed frames and the remaining (incomplete) buffer.
// This is exported for testing compatibility with the TS implementation;
// the actual transport uses go-sse's built-in parser.
func ParseSSEFrames(buffer string) (frames []SSEFrame, remaining string) {
	pos := 0

	for {
		idx := strings.Index(buffer[pos:], "\n\n")
		if idx == -1 {
			break
		}
		rawFrame := buffer[pos : pos+idx]
		pos += idx + 2

		if strings.TrimSpace(rawFrame) == "" {
			continue
		}

		var frame SSEFrame
		isComment := false

		for _, line := range strings.Split(rawFrame, "\n") {
			if strings.HasPrefix(line, ":") {
				isComment = true
				continue
			}

			colonIdx := strings.Index(line, ":")
			if colonIdx == -1 {
				continue
			}

			field := line[:colonIdx]
			value := line[colonIdx+1:]
			// Per SSE spec, strip one leading space after colon if present.
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}

			switch field {
			case "event":
				frame.Event = value
			case "id":
				frame.ID = value
			case "data":
				if frame.Data != "" {
					frame.Data += "\n" + value
				} else {
					frame.Data = value
				}
			}
		}

		if frame.Data != "" || isComment {
			frames = append(frames, frame)
		}
	}

	return frames, buffer[pos:]
}
