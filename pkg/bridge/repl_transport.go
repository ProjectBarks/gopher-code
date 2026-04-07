// Package bridge — ReplBridgeTransport: SSE/HTTP transport layer for CCR v2.
// Source: src/bridge/replBridgeTransport.ts
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmaxmax/go-sse"
)

// ---------------------------------------------------------------------------
// Close codes — reserved wire values matching TS source
// ---------------------------------------------------------------------------

const (
	// CloseCodeEpochMismatch signals that the server returned 409 (epoch superseded).
	CloseCodeEpochMismatch = 4090
	// CloseCodeInitFailure signals that CCRClient.initialize failed.
	CloseCodeInitFailure = 4091
	// CloseCodeSSEExhausted signals that the SSE reconnect budget was exhausted.
	CloseCodeSSEExhausted = 4092
)

// ---------------------------------------------------------------------------
// SessionState — worker state reported to the backend
// ---------------------------------------------------------------------------

// SessionState represents the state of a bridge worker session.
type SessionState string

const (
	SessionStateIdle           SessionState = "idle"
	SessionStateProcessing     SessionState = "processing"
	SessionStateRequiresAction SessionState = "requires_action"
)

// ---------------------------------------------------------------------------
// StdoutMessage — minimal wire type for outbound messages
// ---------------------------------------------------------------------------

// StdoutMessage is the outbound message written through the transport.
// It is JSON-serialised for HTTP POST to the /worker/events endpoint.
type StdoutMessage = json.RawMessage

// ---------------------------------------------------------------------------
// ReplBridgeTransport — interface consumed by ReplBridge
// Source: ReplBridgeTransport type in replBridgeTransport.ts
// ---------------------------------------------------------------------------

// ReplBridgeTransport abstracts the v1 (HybridTransport) and v2
// (SSETransport+CCRClient) paths behind a single interface.
type ReplBridgeTransport interface {
	// WriteMessage sends a single outbound message.
	WriteMessage(ctx context.Context, msg StdoutMessage) error
	// WriteBatch sends a batch of outbound messages in order.
	WriteBatch(ctx context.Context, msgs []StdoutMessage) error
	// Close tears down the transport.
	Close()
	// IsConnected reports whether the write path is ready.
	IsConnected() bool
	// StateLabel returns a human-readable connection state for debug logging.
	StateLabel() string
	// SetOnData registers a callback for inbound SSE data frames.
	SetOnData(callback func(data string))
	// SetOnClose registers a callback fired when the transport closes.
	SetOnClose(callback func(closeCode int))
	// SetOnConnect registers a callback fired when the write path is ready.
	SetOnConnect(callback func())
	// Connect opens the SSE read stream and initialises the CCR write path.
	Connect()
	// LastSequenceNum returns the high-water mark of SSE event sequence numbers.
	LastSequenceNum() int64
	// DroppedBatchCount returns the number of silently dropped batches (v2 = 0).
	DroppedBatchCount() int64
	// ReportState sends worker state to the backend (v2 only; v1 is no-op).
	ReportState(state SessionState)
	// ReportMetadata sends external_metadata to the backend (v2 only; v1 is no-op).
	ReportMetadata(metadata map[string]any)
	// ReportDelivery posts event delivery status (v2 only; v1 is no-op).
	ReportDelivery(eventID string, status string)
	// Flush drains the write queue before close (v2 only; v1 returns nil).
	Flush() error
}

// ---------------------------------------------------------------------------
// V2TransportOpts — configuration for createV2ReplTransport
// ---------------------------------------------------------------------------

// V2TransportOpts configures a v2 (SSE+CCRClient) transport.
type V2TransportOpts struct {
	// SessionURL is the base HTTP(S) URL for the session (no trailing slash).
	SessionURL string
	// IngressToken is the bearer token for auth headers.
	IngressToken string
	// SessionID identifies the session (for debug logging).
	SessionID string
	// InitialSequenceNum is the SSE resume cursor from the previous transport.
	InitialSequenceNum int64
	// Epoch is the worker epoch from POST /bridge. When zero, registerWorker is called.
	Epoch int64
	// HeartbeatInterval is the keep-alive cadence. Defaults to 20s.
	HeartbeatInterval time.Duration
	// HeartbeatJitterFraction is the +/- fraction per beat. Defaults to 0.
	HeartbeatJitterFraction float64
	// OutboundOnly skips the SSE read stream (mirror mode).
	OutboundOnly bool
	// GetAuthToken is a per-instance auth source. When nil, IngressToken is used directly.
	GetAuthToken func() string
	// HTTPClient is the HTTP client for outbound POSTs. Defaults to http.DefaultClient.
	HTTPClient *http.Client
	// SSEClient is the go-sse client for the inbound stream. Defaults to a
	// pre-configured client with reconnect backoff.
	SSEClient *sse.Client
	// Now is a clock function for testing. Defaults to time.Now.
	Now func() time.Time
	// Logger receives debug log lines. Nil = discard.
	Logger func(msg string)
}

// ---------------------------------------------------------------------------
// ErrEpochSuperseded — returned when the server 409s
// ---------------------------------------------------------------------------

// ErrEpochSuperseded is returned when the server indicates the worker epoch
// has been superseded (HTTP 409). Matches TS throw "epoch superseded".
var ErrEpochSuperseded = errors.New("epoch superseded")

// ---------------------------------------------------------------------------
// v2ReplTransport — SSE reads + HTTP POST writes
// ---------------------------------------------------------------------------

type v2ReplTransport struct {
	opts V2TransportOpts

	mu          sync.Mutex
	onDataCb    func(data string)
	onCloseCb   func(closeCode int)
	onConnectCb func()

	epoch          int64
	initialized    atomic.Bool
	closed         atomic.Bool
	lastSeqNum     atomic.Int64
	cancelSSE      context.CancelFunc
	heartbeatStop  chan struct{}

	log func(string)
}

// NewV2ReplTransport creates a v2 transport. It does NOT call Connect —
// the caller must wire callbacks then call Connect().
// If opts.Epoch is zero, the caller must have called RegisterWorker first
// and set opts.Epoch.
func NewV2ReplTransport(opts V2TransportOpts) ReplBridgeTransport {
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = 20 * time.Second
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}

	t := &v2ReplTransport{
		opts:          opts,
		epoch:         opts.Epoch,
		heartbeatStop: make(chan struct{}),
		log:           opts.Logger,
	}
	t.lastSeqNum.Store(opts.InitialSequenceNum)
	return t
}

func (t *v2ReplTransport) authHeaders() map[string]string {
	var token string
	if t.opts.GetAuthToken != nil {
		token = t.opts.GetAuthToken()
	} else {
		token = t.opts.IngressToken
	}
	if token == "" {
		return nil
	}
	return map[string]string{"Authorization": "Bearer " + token}
}

func (t *v2ReplTransport) sseURL() string {
	base := strings.TrimRight(t.opts.SessionURL, "/")
	return base + "/worker/events/stream"
}

// ---------------------------------------------------------------------------
// Interface — write path
// ---------------------------------------------------------------------------

func (t *v2ReplTransport) WriteMessage(ctx context.Context, msg StdoutMessage) error {
	if t.closed.Load() {
		return errors.New("transport closed")
	}
	return t.postEvent(ctx, msg)
}

func (t *v2ReplTransport) WriteBatch(ctx context.Context, msgs []StdoutMessage) error {
	for _, m := range msgs {
		if t.closed.Load() {
			break
		}
		if err := t.postEvent(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func (t *v2ReplTransport) Close() {
	if t.closed.Swap(true) {
		return // already closed
	}
	if t.cancelSSE != nil {
		t.cancelSSE()
	}
	select {
	case <-t.heartbeatStop:
	default:
		close(t.heartbeatStop)
	}
}

func (t *v2ReplTransport) IsConnected() bool {
	return t.initialized.Load() && !t.closed.Load()
}

func (t *v2ReplTransport) StateLabel() string {
	if t.closed.Load() {
		return "closed"
	}
	if t.initialized.Load() {
		return "connected"
	}
	return "connecting"
}

func (t *v2ReplTransport) SetOnData(cb func(data string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDataCb = cb
}

func (t *v2ReplTransport) SetOnClose(cb func(closeCode int)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onCloseCb = cb
}

func (t *v2ReplTransport) SetOnConnect(cb func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onConnectCb = cb
}

func (t *v2ReplTransport) LastSequenceNum() int64 {
	return t.lastSeqNum.Load()
}

func (t *v2ReplTransport) DroppedBatchCount() int64 { return 0 }

func (t *v2ReplTransport) ReportState(state SessionState) {
	t.fireAndForget("PUT", "/worker/state", map[string]any{"state": string(state)})
}

func (t *v2ReplTransport) ReportMetadata(metadata map[string]any) {
	t.fireAndForget("PUT", "/worker/external_metadata", metadata)
}

func (t *v2ReplTransport) ReportDelivery(eventID string, status string) {
	path := fmt.Sprintf("/worker/events/%s/delivery", eventID)
	t.fireAndForget("POST", path, map[string]any{"status": status})
}

func (t *v2ReplTransport) Flush() error { return nil }

// ---------------------------------------------------------------------------
// Connect — opens SSE + initialises write path
// ---------------------------------------------------------------------------

func (t *v2ReplTransport) Connect() {
	// Start SSE read stream (unless outbound-only).
	if !t.opts.OutboundOnly {
		ctx, cancel := context.WithCancel(context.Background())
		t.cancelSSE = cancel
		go t.runSSE(ctx)
	}

	// Initialize CCR write path (set epoch, start heartbeat).
	go t.initialize()
}

func (t *v2ReplTransport) initialize() {
	err := t.postInitialize()
	if err != nil {
		t.log(fmt.Sprintf("[bridge:repl] CCR v2 initialize failed: %s", err))
		t.Close()
		t.mu.Lock()
		cb := t.onCloseCb
		t.mu.Unlock()
		if cb != nil {
			cb(CloseCodeInitFailure)
		}
		return
	}

	t.initialized.Store(true)

	sseState := "opening"
	if !t.opts.OutboundOnly && !t.closed.Load() {
		// Approximate: we can't know if SSE is fully open yet.
		sseState = "open"
	}
	t.log(fmt.Sprintf(
		"[bridge:repl] v2 transport ready for writes (epoch=%d, sse=%s)",
		t.epoch, sseState,
	))

	// Start heartbeat.
	go t.runHeartbeat()

	t.mu.Lock()
	cb := t.onConnectCb
	t.mu.Unlock()
	if cb != nil {
		cb()
	}
}

func (t *v2ReplTransport) postInitialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]any{"worker_epoch": t.epoch})
	url := strings.TrimRight(t.opts.SessionURL, "/") + "/worker/initialize"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, v := range t.authHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.opts.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusConflict {
		return ErrEpochSuperseded
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("initialize: HTTP %d", resp.StatusCode)
	}
	return nil
}

// ---------------------------------------------------------------------------
// SSE reader — uses go-sse Client + Connection
// ---------------------------------------------------------------------------

func (t *v2ReplTransport) runSSE(ctx context.Context) {
	sseURL := t.sseURL()

	// Build SSE client with reconnect backoff.
	client := t.opts.SSEClient
	if client == nil {
		client = &sse.Client{
			HTTPClient: t.opts.HTTPClient,
			Backoff: sse.Backoff{
				InitialInterval: 500 * time.Millisecond,
				Multiplier:      1.5,
				Jitter:          0.5,
				MaxInterval:     30 * time.Second,
				MaxRetries:      10,
			},
			ResponseValidator: sse.DefaultValidator,
		}
	}

	// Inject auth headers into the request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseURL, nil)
	if err != nil {
		t.log(fmt.Sprintf("[bridge:repl] SSE request creation failed: %s", err))
		return
	}
	for k, v := range t.authHeaders() {
		req.Header.Set(k, v)
	}

	// Set Last-Event-ID for resume.
	if seq := t.lastSeqNum.Load(); seq > 0 {
		req.Header.Set("Last-Event-ID", fmt.Sprintf("%d", seq))
	}

	conn := client.NewConnection(req)
	conn.SubscribeToAll(func(ev sse.Event) {
		t.handleSSEEvent(ev)
	})

	// Connect blocks until ctx cancelled or retries exhausted.
	connErr := conn.Connect()

	if ctx.Err() != nil {
		// Normal shutdown via Close().
		return
	}

	// Reconnect budget exhausted.
	t.log("[bridge:repl] SSE reconnect budget exhausted")
	t.Close()
	t.mu.Lock()
	cb := t.onCloseCb
	t.mu.Unlock()
	if cb != nil {
		cb(CloseCodeSSEExhausted)
	}
	_ = connErr
}

func (t *v2ReplTransport) handleSSEEvent(ev sse.Event) {
	// Track sequence number from LastEventID.
	if ev.LastEventID != "" {
		var seq int64
		if _, err := fmt.Sscanf(ev.LastEventID, "%d", &seq); err == nil {
			t.lastSeqNum.Store(seq)
		}
	}

	// ACK both received + processed immediately (matches TS setOnEvent override).
	if ev.LastEventID != "" {
		t.ReportDelivery(ev.LastEventID, "received")
		t.ReportDelivery(ev.LastEventID, "processed")
	}

	// Deliver data to the onData callback.
	if ev.Data != "" {
		t.mu.Lock()
		cb := t.onDataCb
		t.mu.Unlock()
		if cb != nil {
			cb(ev.Data)
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP POST — outbound event writes
// ---------------------------------------------------------------------------

func (t *v2ReplTransport) postEvent(ctx context.Context, msg StdoutMessage) error {
	url := strings.TrimRight(t.opts.SessionURL, "/") + "/worker/events"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(msg))
	if err != nil {
		return err
	}
	for k, v := range t.authHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.opts.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusConflict {
		t.log("[bridge:repl] CCR v2: epoch superseded (409) — closing for poll-loop recovery")
		t.Close()
		t.mu.Lock()
		cb := t.onCloseCb
		t.mu.Unlock()
		if cb != nil {
			cb(CloseCodeEpochMismatch)
		}
		return ErrEpochSuperseded
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("postEvent: HTTP %d", resp.StatusCode)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Heartbeat — periodic keep-alive POST
// ---------------------------------------------------------------------------

func (t *v2ReplTransport) runHeartbeat() {
	interval := t.opts.HeartbeatInterval
	jitter := t.opts.HeartbeatJitterFraction
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		wait := interval
		if jitter > 0 {
			delta := float64(interval) * jitter
			wait = time.Duration(float64(interval) + (rng.Float64()*2-1)*delta)
		}

		select {
		case <-time.After(wait):
			t.fireAndForget("POST", "/worker/heartbeat", map[string]any{"worker_epoch": t.epoch})
		case <-t.heartbeatStop:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// fireAndForget — helper for non-critical background POSTs/PUTs
// ---------------------------------------------------------------------------

func (t *v2ReplTransport) fireAndForget(method, path string, payload map[string]any) {
	if t.closed.Load() {
		return
	}
	body, _ := json.Marshal(payload)
	url := strings.TrimRight(t.opts.SessionURL, "/") + path

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	for k, v := range t.authHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.opts.HTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

// ---------------------------------------------------------------------------
// V1 adapter — passthrough to a HybridTransport-shaped delegate
// ---------------------------------------------------------------------------

// V1TransportDelegate is the minimal surface of HybridTransport that the
// v1 adapter wraps. Implemented by the HybridTransport struct (future task).
type V1TransportDelegate interface {
	Write(msg StdoutMessage) error
	WriteBatch(msgs []StdoutMessage) error
	Close()
	IsConnectedStatus() bool
	GetStateLabel() string
	SetOnData(cb func(data string))
	SetOnClose(cb func(closeCode int))
	SetOnConnect(cb func())
	Connect()
	DroppedBatchCount() int64
}

type v1ReplTransport struct {
	delegate V1TransportDelegate
}

// NewV1ReplTransport wraps a HybridTransport in the ReplBridgeTransport interface.
func NewV1ReplTransport(delegate V1TransportDelegate) ReplBridgeTransport {
	return &v1ReplTransport{delegate: delegate}
}

func (t *v1ReplTransport) WriteMessage(_ context.Context, msg StdoutMessage) error {
	return t.delegate.Write(msg)
}
func (t *v1ReplTransport) WriteBatch(_ context.Context, msgs []StdoutMessage) error {
	return t.delegate.WriteBatch(msgs)
}
func (t *v1ReplTransport) Close()                         { t.delegate.Close() }
func (t *v1ReplTransport) IsConnected() bool              { return t.delegate.IsConnectedStatus() }
func (t *v1ReplTransport) StateLabel() string             { return t.delegate.GetStateLabel() }
func (t *v1ReplTransport) SetOnData(cb func(string))      { t.delegate.SetOnData(cb) }
func (t *v1ReplTransport) SetOnClose(cb func(int))        { t.delegate.SetOnClose(cb) }
func (t *v1ReplTransport) SetOnConnect(cb func())         { t.delegate.SetOnConnect(cb) }
func (t *v1ReplTransport) Connect()                       { t.delegate.Connect() }
func (t *v1ReplTransport) LastSequenceNum() int64         { return 0 }
func (t *v1ReplTransport) DroppedBatchCount() int64       { return t.delegate.DroppedBatchCount() }
func (t *v1ReplTransport) ReportState(SessionState)       {}
func (t *v1ReplTransport) ReportMetadata(map[string]any)  {}
func (t *v1ReplTransport) ReportDelivery(string, string)  {}
func (t *v1ReplTransport) Flush() error                   { return nil }

// ---------------------------------------------------------------------------
// SSE adapter — wraps SSETransport in the ReplBridgeTransport interface
// ---------------------------------------------------------------------------

type sseReplTransport struct {
	delegate    *SSETransport
	mu          sync.Mutex
	onConnectCb func()
}

// NewSSEReplTransport wraps an SSETransport in the ReplBridgeTransport interface.
func NewSSEReplTransport(delegate *SSETransport) ReplBridgeTransport {
	return &sseReplTransport{delegate: delegate}
}

func (t *sseReplTransport) WriteMessage(ctx context.Context, msg StdoutMessage) error {
	return t.delegate.Write(ctx, msg)
}

func (t *sseReplTransport) WriteBatch(ctx context.Context, msgs []StdoutMessage) error {
	for _, m := range msgs {
		if err := t.delegate.Write(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func (t *sseReplTransport) Close() { t.delegate.Close() }

func (t *sseReplTransport) IsConnected() bool { return t.delegate.IsConnected() }

func (t *sseReplTransport) StateLabel() string {
	if t.delegate.IsClosed() {
		return "closed"
	}
	if t.delegate.IsConnected() {
		return "connected"
	}
	return "connecting"
}

func (t *sseReplTransport) SetOnData(cb func(string)) { t.delegate.SetOnData(cb) }

func (t *sseReplTransport) SetOnClose(cb func(int)) { t.delegate.SetOnClose(cb) }

func (t *sseReplTransport) SetOnConnect(cb func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onConnectCb = cb
}

func (t *sseReplTransport) Connect() {
	go func() {
		t.delegate.Connect(context.Background())
		// Fire onConnect once the SSE stream is established.
		t.mu.Lock()
		cb := t.onConnectCb
		t.mu.Unlock()
		if cb != nil {
			cb()
		}
	}()
}

func (t *sseReplTransport) LastSequenceNum() int64         { return t.delegate.GetLastSequenceNum() }
func (t *sseReplTransport) DroppedBatchCount() int64       { return 0 }
func (t *sseReplTransport) ReportState(SessionState)       {}
func (t *sseReplTransport) ReportMetadata(map[string]any)  {}
func (t *sseReplTransport) ReportDelivery(string, string)  {}
func (t *sseReplTransport) Flush() error                   { return nil }
