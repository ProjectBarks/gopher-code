// Package bridge — HybridTransport: WS read + HTTP POST write (v1 session-ingress).
// Source: src/cli/transports/HybridTransport.ts
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// ---------------------------------------------------------------------------
// Constants — match TS source verbatim
// ---------------------------------------------------------------------------

const (
	// BatchFlushInterval is the stream_event buffer coalesce cadence.
	BatchFlushInterval = 100 * time.Millisecond

	// PostTimeout bounds a single stuck POST.
	PostTimeout = 15 * time.Second

	// CloseGrace is the best-effort drain window on Close().
	CloseGrace = 3 * time.Second

	// maxBatchSize caps events per POST.
	maxBatchSize = 500

	// defaultMaxQueueSize is the backpressure ceiling.
	defaultMaxQueueSize = 100_000

	// defaultBaseDelay is the initial retry backoff.
	defaultBaseDelay = 500 * time.Millisecond

	// defaultMaxDelay caps exponential backoff.
	defaultMaxDelay = 8 * time.Second

	// defaultJitter is the random jitter range.
	defaultJitter = 1 * time.Second
)

// ---------------------------------------------------------------------------
// HybridTransportOpts
// ---------------------------------------------------------------------------

// HybridTransportOpts configures a HybridTransport.
type HybridTransportOpts struct {
	// URL is the WebSocket endpoint (ws:// or wss://).
	URL *url.URL
	// Headers are sent on both the WS handshake and every POST.
	Headers map[string]string
	// SessionID identifies the session (debug logging).
	SessionID string

	// GetAuthToken returns the current session-ingress bearer token.
	// Called per-POST so rotated tokens are picked up automatically.
	GetAuthToken func() string

	// MaxConsecutiveFailures caps retries before dropping a batch.
	// Zero means retry indefinitely.
	MaxConsecutiveFailures int

	// OnBatchDropped is called when a batch is dropped after max failures.
	OnBatchDropped func(batchSize int, failures int)

	// HTTPClient is the HTTP client for outbound POSTs.
	// Defaults to a client with PostTimeout.
	HTTPClient *http.Client

	// Logger receives debug log lines. Nil = discard.
	Logger func(msg string)

	// now is a clock for testing. Defaults to time.Now.
	now func() time.Time
}

// ---------------------------------------------------------------------------
// HybridTransport — implements V1TransportDelegate
// ---------------------------------------------------------------------------

// Compile-time interface assertion.
var _ V1TransportDelegate = (*HybridTransport)(nil)

// HybridTransport reads from a WebSocket and writes via HTTP POST with
// serial batching, stream_event coalescing, and order preservation.
type HybridTransport struct {
	opts    HybridTransportOpts
	postURL string

	mu          sync.Mutex
	onDataCb    func(data string)
	onCloseCb   func(closeCode int)
	onConnectCb func()

	// Stream-event delay buffer.
	streamBuf   []StdoutMessage
	streamTimer *time.Timer

	// Serial batch uploader state.
	pending  []StdoutMessage
	draining atomic.Bool

	flushMu      sync.Mutex
	flushWaiters []chan struct{}

	droppedBatches atomic.Int64
	closed         atomic.Bool

	cancelWS context.CancelFunc
	log      func(string)
	http     *http.Client
}

// NewHybridTransport creates a HybridTransport. Call Connect() to start.
func NewHybridTransport(opts HybridTransportOpts) *HybridTransport {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: PostTimeout}
	}
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}
	if opts.now == nil {
		opts.now = time.Now
	}

	t := &HybridTransport{
		opts:    opts,
		postURL: ConvertWsURLToPostURL(opts.URL),
		log:     opts.Logger,
		http:    opts.HTTPClient,
	}

	t.log(fmt.Sprintf("HybridTransport: POST URL = %s", t.postURL))
	return t
}

// ---------------------------------------------------------------------------
// V1TransportDelegate — interface implementation
// ---------------------------------------------------------------------------

// Write sends a single outbound message. stream_event messages are buffered
// for up to BatchFlushInterval; all others flush any buffered stream_events
// first to preserve ordering.
func (t *HybridTransport) Write(msg StdoutMessage) error {
	if t.closed.Load() {
		return fmt.Errorf("transport closed")
	}

	msgType := extractMessageType(msg)

	if msgType == "stream_event" {
		t.mu.Lock()
		t.streamBuf = append(t.streamBuf, msg)
		if t.streamTimer == nil {
			t.streamTimer = time.AfterFunc(BatchFlushInterval, t.flushStreamEvents)
		}
		t.mu.Unlock()
		return nil
	}

	// Non-stream: flush buffered stream_events first (order preservation),
	// then enqueue this event.
	buffered := t.takeStreamEvents()
	batch := make([]StdoutMessage, 0, len(buffered)+1)
	batch = append(batch, buffered...)
	batch = append(batch, msg)
	t.enqueue(batch)
	return nil
}

// WriteBatch sends a batch of outbound messages.
func (t *HybridTransport) WriteBatch(msgs []StdoutMessage) error {
	if t.closed.Load() {
		return fmt.Errorf("transport closed")
	}
	buffered := t.takeStreamEvents()
	batch := make([]StdoutMessage, 0, len(buffered)+len(msgs))
	batch = append(batch, buffered...)
	batch = append(batch, msgs...)
	t.enqueue(batch)
	return nil
}

// Close tears down the transport with a grace period for queued writes.
func (t *HybridTransport) Close() {
	if t.closed.Swap(true) {
		return
	}

	// Clear stream buffer.
	t.mu.Lock()
	if t.streamTimer != nil {
		t.streamTimer.Stop()
		t.streamTimer = nil
	}
	t.streamBuf = nil
	t.mu.Unlock()

	// Grace period for queued writes.
	done := make(chan struct{})
	go func() {
		defer close(done)
		t.flushUploader()
	}()

	select {
	case <-done:
	case <-time.After(CloseGrace):
	}

	// Cancel WS.
	if t.cancelWS != nil {
		t.cancelWS()
	}
}

// IsConnectedStatus reports whether the transport is open.
func (t *HybridTransport) IsConnectedStatus() bool {
	return !t.closed.Load()
}

// GetStateLabel returns a human-readable state.
func (t *HybridTransport) GetStateLabel() string {
	if t.closed.Load() {
		return "closed"
	}
	return "connected"
}

// SetOnData registers the inbound data callback.
func (t *HybridTransport) SetOnData(cb func(data string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDataCb = cb
}

// SetOnClose registers the close callback.
func (t *HybridTransport) SetOnClose(cb func(closeCode int)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onCloseCb = cb
}

// SetOnConnect registers the connect callback.
func (t *HybridTransport) SetOnConnect(cb func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onConnectCb = cb
}

// DroppedBatchCount returns the number of silently dropped batches.
func (t *HybridTransport) DroppedBatchCount() int64 {
	return t.droppedBatches.Load()
}

// Connect opens the WebSocket read stream.
func (t *HybridTransport) Connect() {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancelWS = cancel
	go t.runWebSocket(ctx)
}

// ---------------------------------------------------------------------------
// WebSocket read loop
// ---------------------------------------------------------------------------

func (t *HybridTransport) runWebSocket(ctx context.Context) {
	wsURL := t.opts.URL.String()

	// Build dial options with headers.
	dialOpts := &websocket.DialOptions{
		HTTPHeader: http.Header{},
	}
	for k, v := range t.opts.Headers {
		dialOpts.HTTPHeader.Set(k, v)
	}

	conn, _, err := websocket.Dial(ctx, wsURL, dialOpts)
	if err != nil {
		t.log(fmt.Sprintf("HybridTransport: WS dial failed: %s", err))
		t.mu.Lock()
		cb := t.onCloseCb
		t.mu.Unlock()
		if cb != nil {
			cb(0)
		}
		return
	}

	// Fire connect callback.
	t.mu.Lock()
	cb := t.onConnectCb
	t.mu.Unlock()
	if cb != nil {
		cb()
	}

	// Read loop.
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// Normal shutdown.
				return
			}
			t.log(fmt.Sprintf("HybridTransport: WS read error: %s", err))
			t.mu.Lock()
			closeCb := t.onCloseCb
			t.mu.Unlock()
			if closeCb != nil {
				closeCb(0)
			}
			return
		}

		t.mu.Lock()
		dataCb := t.onDataCb
		t.mu.Unlock()
		if dataCb != nil {
			dataCb(string(data))
		}
	}
}

// ---------------------------------------------------------------------------
// Stream-event buffer management
// ---------------------------------------------------------------------------

// takeStreamEvents takes ownership of buffered stream_events and clears the timer.
func (t *HybridTransport) takeStreamEvents() []StdoutMessage {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.streamTimer != nil {
		t.streamTimer.Stop()
		t.streamTimer = nil
	}
	buf := t.streamBuf
	t.streamBuf = nil
	return buf
}

// flushStreamEvents is fired by the delay timer — enqueue accumulated stream_events.
func (t *HybridTransport) flushStreamEvents() {
	t.mu.Lock()
	t.streamTimer = nil
	buf := t.streamBuf
	t.streamBuf = nil
	t.mu.Unlock()

	if len(buf) > 0 {
		t.enqueue(buf)
	}
}

// ---------------------------------------------------------------------------
// Serial batch uploader — embedded, simplified from SerialBatchEventUploader
// ---------------------------------------------------------------------------

// enqueue adds events to the pending buffer and kicks the drain loop.
func (t *HybridTransport) enqueue(events []StdoutMessage) {
	if t.closed.Load() || len(events) == 0 {
		return
	}

	t.mu.Lock()
	t.pending = append(t.pending, events...)
	t.mu.Unlock()

	t.triggerDrain()
}

func (t *HybridTransport) triggerDrain() {
	if t.draining.CompareAndSwap(false, true) {
		go t.drain()
	}
}

func (t *HybridTransport) drain() {
	defer t.draining.Store(false)
	failures := 0

	for !t.closed.Load() {
		batch := t.takeBatch()
		if len(batch) == 0 {
			break
		}

		err := t.postOnce(batch)
		if err == nil {
			failures = 0
			t.releaseFlushWaiters()
			continue
		}

		failures++

		// Drop batch if max consecutive failures exceeded.
		if t.opts.MaxConsecutiveFailures > 0 && failures >= t.opts.MaxConsecutiveFailures {
			t.droppedBatches.Add(1)
			if t.opts.OnBatchDropped != nil {
				t.opts.OnBatchDropped(len(batch), failures)
			}
			failures = 0
			continue
		}

		// Re-queue failed batch at front.
		t.mu.Lock()
		t.pending = append(batch, t.pending...)
		t.mu.Unlock()

		// Exponential backoff with jitter.
		delay := t.retryDelay(failures)
		timer := time.NewTimer(delay)
		<-timer.C
		timer.Stop()
	}

	// Notify flush waiters if drained.
	t.mu.Lock()
	empty := len(t.pending) == 0
	t.mu.Unlock()
	if empty {
		t.releaseFlushWaiters()
	}
}

func (t *HybridTransport) takeBatch() []StdoutMessage {
	t.mu.Lock()
	defer t.mu.Unlock()

	n := len(t.pending)
	if n == 0 {
		return nil
	}
	if n > maxBatchSize {
		n = maxBatchSize
	}
	batch := make([]StdoutMessage, n)
	copy(batch, t.pending[:n])
	t.pending = t.pending[n:]
	return batch
}

func (t *HybridTransport) retryDelay(failures int) time.Duration {
	exp := float64(defaultBaseDelay) * math.Pow(2, float64(failures-1))
	if exp > float64(defaultMaxDelay) {
		exp = float64(defaultMaxDelay)
	}
	jitter := time.Duration(rand.Float64() * float64(defaultJitter))
	return time.Duration(exp) + jitter
}

func (t *HybridTransport) flushUploader() {
	t.flushMu.Lock()
	t.mu.Lock()
	empty := len(t.pending) == 0
	draining := t.draining.Load()
	t.mu.Unlock()

	if empty && !draining {
		t.flushMu.Unlock()
		return
	}

	ch := make(chan struct{}, 1)
	t.flushWaiters = append(t.flushWaiters, ch)
	t.flushMu.Unlock()

	t.triggerDrain()
	<-ch
}

func (t *HybridTransport) releaseFlushWaiters() {
	t.flushMu.Lock()
	defer t.flushMu.Unlock()

	t.mu.Lock()
	empty := len(t.pending) == 0
	t.mu.Unlock()

	if !empty {
		return
	}

	for _, ch := range t.flushWaiters {
		close(ch)
	}
	t.flushWaiters = nil
}

// ---------------------------------------------------------------------------
// HTTP POST — single-attempt send
// ---------------------------------------------------------------------------

func (t *HybridTransport) postOnce(events []StdoutMessage) error {
	var token string
	if t.opts.GetAuthToken != nil {
		token = t.opts.GetAuthToken()
	}
	if token == "" {
		t.log("HybridTransport: No session token available for POST")
		return nil // permanent — don't retry
	}

	body, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		return nil // permanent — can't serialize
	}

	ctx, cancel := context.WithTimeout(context.Background(), PostTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.postURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.http.Do(req)
	if err != nil {
		t.log(fmt.Sprintf("HybridTransport: POST error: %s", err))
		return err // retryable
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t.log(fmt.Sprintf("HybridTransport: POST success count=%d", len(events)))
		return nil
	}

	// 4xx (except 429) — permanent, don't retry.
	if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
		t.log(fmt.Sprintf("HybridTransport: POST returned %d (permanent), dropping", resp.StatusCode))
		return nil
	}

	// 429 / 5xx — retryable.
	t.log(fmt.Sprintf("HybridTransport: POST returned %d (retryable)", resp.StatusCode))
	return fmt.Errorf("POST failed with %d", resp.StatusCode)
}

// ---------------------------------------------------------------------------
// URL conversion — exported for testing
// ---------------------------------------------------------------------------

// ConvertWsURLToPostURL maps a WebSocket URL to the HTTP POST endpoint URL.
//
//	wss://api.example.com/v2/session_ingress/ws/<session_id>
//	  → https://api.example.com/v2/session_ingress/session/<session_id>/events
func ConvertWsURLToPostURL(wsURL *url.URL) string {
	protocol := "http:"
	if wsURL.Scheme == "wss" {
		protocol = "https:"
	}

	pathname := wsURL.Path
	pathname = strings.Replace(pathname, "/ws/", "/session/", 1)
	if !strings.HasSuffix(pathname, "/events") {
		if strings.HasSuffix(pathname, "/") {
			pathname += "events"
		} else {
			pathname += "/events"
		}
	}

	query := ""
	if wsURL.RawQuery != "" {
		query = "?" + wsURL.RawQuery
	}

	return fmt.Sprintf("%s//%s%s%s", protocol, wsURL.Host, pathname, query)
}

// extractMessageType pulls the "type" field from a JSON message without
// full deserialization.
func extractMessageType(msg StdoutMessage) string {
	var envelope struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(msg, &envelope) == nil {
		return envelope.Type
	}
	return ""
}
