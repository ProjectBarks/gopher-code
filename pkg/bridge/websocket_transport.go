// Package bridge — WebSocketTransport: full-duplex WS for session ingress v1.
// Source: src/cli/transports/WebSocketTransport.ts
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/projectbarks/gopher-code/pkg/analytics"
	"github.com/coder/websocket"
)

// ---------------------------------------------------------------------------
// Constants — match TS source verbatim
// ---------------------------------------------------------------------------

const (
	// KeepAliveFrame is the data frame sent to reset proxy idle timers.
	KeepAliveFrame = "{\"type\":\"keep_alive\"}\n"

	// DefaultMaxBufferSize is the replay buffer capacity.
	DefaultMaxBufferSize = 1000

	// DefaultBaseReconnectDelay is the initial retry backoff.
	DefaultBaseReconnectDelay = 1000 * time.Millisecond

	// DefaultMaxReconnectDelay caps exponential backoff.
	DefaultMaxReconnectDelay = 30000 * time.Millisecond

	// DefaultReconnectGiveUpMS is the time budget for reconnection (10 minutes).
	DefaultReconnectGiveUpMS = 600_000 * time.Millisecond

	// DefaultPingInterval is the ping health-check cadence (10s).
	DefaultPingInterval = 10000 * time.Millisecond

	// DefaultKeepaliveInterval is the keep_alive data frame cadence (5min).
	DefaultKeepaliveInterval = 300_000 * time.Millisecond

	// SleepDetectionThresholdMS is 2x max reconnect delay (60s).
	// If the gap between consecutive reconnect attempts exceeds this, the
	// machine likely slept. We reset the reconnection budget and retry.
	SleepDetectionThresholdMS = 60_000 * time.Millisecond
)

// PermanentCloseCodes are WebSocket close codes that indicate a permanent
// server-side rejection. The transport transitions to closed immediately.
var PermanentCloseCodes = map[int]bool{
	1002: true, // protocol error — server rejected handshake
	4001: true, // session expired / not found
	4003: true, // unauthorized
}

// ---------------------------------------------------------------------------
// WebSocketTransportState — 5-state machine
// ---------------------------------------------------------------------------

// WSTransportState represents the connection state of a WebSocketTransport.
type WSTransportState int

const (
	// WSStateIdle is the initial disconnected state.
	WSStateIdle WSTransportState = iota
	// WSStateConnecting is the transitional connecting state (mapped from TS "reconnecting" on first connect).
	WSStateConnecting
	// WSStateConnected means the WebSocket is open and healthy.
	WSStateConnected
	// WSStateReconnecting means we're retrying after a drop.
	WSStateReconnecting
	// WSStateClosed is the terminal state.
	WSStateClosed
)

func (s WSTransportState) String() string {
	switch s {
	case WSStateIdle:
		return "idle"
	case WSStateConnecting:
		return "connecting"
	case WSStateConnected:
		return "connected"
	case WSStateReconnecting:
		return "reconnecting"
	case WSStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// CircularBuffer — fixed-capacity ring buffer for message replay
// ---------------------------------------------------------------------------

// CircularBuffer is a fixed-capacity ring buffer.
type CircularBuffer[T any] struct {
	items []T
	cap   int
}

// NewCircularBuffer creates a buffer with the given capacity.
func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
	return &CircularBuffer[T]{cap: capacity}
}

// Add appends an item, evicting the oldest if at capacity.
func (b *CircularBuffer[T]) Add(item T) {
	if len(b.items) >= b.cap {
		b.items = b.items[1:]
	}
	b.items = append(b.items, item)
}

// AddAll appends multiple items.
func (b *CircularBuffer[T]) AddAll(items []T) {
	for _, item := range items {
		b.Add(item)
	}
}

// ToArray returns a copy of all items in insertion order.
func (b *CircularBuffer[T]) ToArray() []T {
	out := make([]T, len(b.items))
	copy(out, b.items)
	return out
}

// Clear removes all items.
func (b *CircularBuffer[T]) Clear() {
	b.items = b.items[:0]
}

// Len returns the current number of items.
func (b *CircularBuffer[T]) Len() int {
	return len(b.items)
}

// ---------------------------------------------------------------------------
// WebSocketTransportOpts
// ---------------------------------------------------------------------------

// WebSocketTransportOpts configures a WebSocketTransport.
type WebSocketTransportOpts struct {
	// URL is the WebSocket endpoint (ws:// or wss://).
	URL *url.URL
	// Headers are sent on the WS handshake.
	Headers map[string]string
	// SessionID identifies the session (debug logging).
	SessionID string
	// AutoReconnect enables automatic reconnection on disconnect.
	// Defaults to true when zero value.
	AutoReconnect *bool
	// IsBridge gates tengu_ws_transport_* telemetry events.
	IsBridge bool
	// RefreshHeaders is called before reconnect to refresh auth headers.
	RefreshHeaders func() map[string]string

	// Logger receives debug log lines. Nil = discard.
	Logger func(msg string)
	// Now is a clock for testing. Defaults to time.Now.
	Now func() time.Time
	// RandFloat returns a random float64 in [0,1) for jitter.
	RandFloat func() float64
	// DialWebSocket is an optional override for WebSocket dialing (for testing).
	DialWebSocket func(ctx context.Context, url string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error)
}

func (o *WebSocketTransportOpts) autoReconnect() bool {
	if o.AutoReconnect == nil {
		return true
	}
	return *o.AutoReconnect
}

// ---------------------------------------------------------------------------
// WebSocketTransport
// ---------------------------------------------------------------------------

// WebSocketTransport implements a full-duplex WebSocket transport with
// automatic reconnection, exponential backoff, ping/pong health checks,
// keep-alive frames, message replay buffer, and sleep detection.
type WebSocketTransport struct {
	opts WebSocketTransportOpts

	mu    sync.Mutex
	state WSTransportState
	conn  *websocket.Conn

	onDataCb    func(data string)
	onCloseCb   func(closeCode int)
	onConnectCb func()

	// Reconnection state.
	reconnectAttempts          int
	reconnectStartTime         time.Time
	lastReconnectAttemptTime   time.Time
	reconnectStartTimeSet      bool
	lastReconnectAttemptTimeSet bool
	reconnectTimer             *time.Timer

	// Activity tracking.
	lastActivityTime time.Time
	lastSentID       string

	// Ping liveness.
	pingTicker   *time.Ticker
	pingDone     chan struct{}
	pongReceived bool
	lastTickTime time.Time

	// Keepalive.
	keepaliveTicker *time.Ticker
	keepaliveDone   chan struct{}

	// Message replay buffer.
	messageBuffer *CircularBuffer[StdoutMessage]

	// Connection context — cancelled on disconnect/close.
	connCancel context.CancelFunc
	connCtx    context.Context

	// Timing helpers.
	now       func() time.Time
	randFloat func() float64
	log       func(string)
	dial      func(ctx context.Context, url string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error)
}

// NewWebSocketTransport creates a WebSocketTransport. Call Connect() to start.
func NewWebSocketTransport(opts WebSocketTransportOpts) *WebSocketTransport {
	if opts.Logger == nil {
		opts.Logger = func(string) {}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.RandFloat == nil {
		opts.RandFloat = func() float64 { return rand.Float64() }
	}
	dial := opts.DialWebSocket
	if dial == nil {
		dial = websocket.Dial
	}

	return &WebSocketTransport{
		opts:          opts,
		state:         WSStateIdle,
		messageBuffer: NewCircularBuffer[StdoutMessage](DefaultMaxBufferSize),
		now:           opts.Now,
		randFloat:     opts.RandFloat,
		log:           opts.Logger,
		dial:          dial,
	}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Connect opens the WebSocket connection.
func (t *WebSocketTransport) Connect() {
	t.mu.Lock()
	if t.state != WSStateIdle && t.state != WSStateReconnecting {
		t.log(fmt.Sprintf("WebSocketTransport: Cannot connect, current state is %s", t.state))
		t.mu.Unlock()
		return
	}
	if t.state == WSStateIdle {
		t.state = WSStateConnecting
	}
	t.mu.Unlock()

	t.doConnect()
}

// Write sends a message through the WebSocket. Messages with a "uuid" field
// are buffered for replay on reconnection.
func (t *WebSocketTransport) Write(message StdoutMessage) error {
	// Buffer messages with uuid for replay.
	uuid := extractUUID(message)
	if uuid != "" {
		t.mu.Lock()
		t.messageBuffer.Add(message)
		t.lastSentID = uuid
		t.mu.Unlock()
	}

	line := string(message) + "\n"

	t.mu.Lock()
	if t.state != WSStateConnected {
		t.mu.Unlock()
		return nil // buffered for replay
	}
	t.mu.Unlock()

	t.log(fmt.Sprintf("WebSocketTransport: Sending message"))
	t.sendLine(line)
	return nil
}

// WriteBatch sends multiple messages through the WebSocket sequentially.
func (t *WebSocketTransport) WriteBatch(msgs []StdoutMessage) error {
	for _, msg := range msgs {
		if err := t.Write(msg); err != nil {
			return err
		}
	}
	return nil
}

// IsConnectedStatus reports whether the transport is connected.
// This is an alias for IsConnected that satisfies the V1TransportDelegate interface.
func (t *WebSocketTransport) IsConnectedStatus() bool {
	return t.IsConnected()
}

// DroppedBatchCount returns the number of dropped batches. WebSocketTransport
// does not drop batches (messages are buffered for replay), so this always
// returns 0.
func (t *WebSocketTransport) DroppedBatchCount() int64 {
	return 0
}

// Close tears down the transport.
func (t *WebSocketTransport) Close() {
	t.mu.Lock()
	if t.reconnectTimer != nil {
		t.reconnectTimer.Stop()
		t.reconnectTimer = nil
	}
	t.state = WSStateClosed
	t.mu.Unlock()

	t.stopPingInterval()
	t.stopKeepaliveInterval()
	t.doDisconnect()
}

// IsConnected reports whether the transport is connected.
func (t *WebSocketTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state == WSStateConnected
}

// IsClosed reports whether the transport is closed.
func (t *WebSocketTransport) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state == WSStateClosed
}

// State returns the current transport state.
func (t *WebSocketTransport) State() WSTransportState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// GetStateLabel returns a human-readable state string.
func (t *WebSocketTransport) GetStateLabel() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state.String()
}

// SetOnData registers the inbound data callback.
func (t *WebSocketTransport) SetOnData(cb func(data string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDataCb = cb
}

// SetOnClose registers the close callback.
func (t *WebSocketTransport) SetOnClose(cb func(closeCode int)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onCloseCb = cb
}

// SetOnConnect registers the connect callback.
func (t *WebSocketTransport) SetOnConnect(cb func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onConnectCb = cb
}

// ---------------------------------------------------------------------------
// Connection lifecycle
// ---------------------------------------------------------------------------

func (t *WebSocketTransport) doConnect() {
	ctx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.connCtx = ctx
	t.connCancel = cancel
	t.mu.Unlock()

	// Build headers.
	headers := make(http.Header)
	for k, v := range t.opts.Headers {
		headers.Set(k, v)
	}
	t.mu.Lock()
	if t.lastSentID != "" {
		headers.Set("X-Last-Request-Id", t.lastSentID)
		t.log(fmt.Sprintf("WebSocketTransport: Adding X-Last-Request-Id header: %s", t.lastSentID))
	}
	t.mu.Unlock()

	dialOpts := &websocket.DialOptions{
		HTTPHeader: headers,
	}

	t.log(fmt.Sprintf("WebSocketTransport: Opening %s", t.opts.URL.String()))

	conn, resp, err := t.dial(ctx, t.opts.URL.String(), dialOpts)
	if err != nil {
		t.log(fmt.Sprintf("WebSocketTransport: Dial failed: %s", err))
		t.handleConnectionError(0)
		return
	}

	// Check for last-id in upgrade response headers.
	var serverLastID string
	if resp != nil {
		serverLastID = resp.Header.Get("X-Last-Request-Id")
	}

	t.mu.Lock()
	t.conn = conn
	t.mu.Unlock()

	t.handleOpenEvent()

	// Replay buffered messages if we have a lastSentID.
	t.mu.Lock()
	hasLastSentID := t.lastSentID != ""
	t.mu.Unlock()
	if hasLastSentID {
		t.replayBufferedMessages(serverLastID)
	}

	// Start read loop.
	go t.readLoop(ctx, conn)
}

func (t *WebSocketTransport) handleOpenEvent() {
	now := t.now()
	t.log("WebSocketTransport: Connected")

	t.mu.Lock()
	// Reconnect success analytics.
	if t.opts.IsBridge && t.reconnectStartTimeSet {
		downtimeMs := now.Sub(t.reconnectStartTime).Milliseconds()
		analytics.LogEvent("tengu_ws_transport_reconnected", analytics.EventMetadata{
			"attempts":   t.reconnectAttempts,
			"downtimeMs": downtimeMs,
		})
	}

	t.reconnectAttempts = 0
	t.reconnectStartTimeSet = false
	t.lastReconnectAttemptTimeSet = false
	t.lastActivityTime = now
	t.state = WSStateConnected
	cb := t.onConnectCb
	t.mu.Unlock()

	if cb != nil {
		cb()
	}

	t.startPingInterval()
	t.startKeepaliveInterval()
}

func (t *WebSocketTransport) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // normal shutdown
			}
			t.log(fmt.Sprintf("WebSocketTransport: Read error: %s", err))
			closeCode := extractCloseCode(err)
			t.handleConnectionError(closeCode)
			return
		}

		t.mu.Lock()
		t.lastActivityTime = t.now()
		cb := t.onDataCb
		t.mu.Unlock()

		if cb != nil {
			cb(string(data))
		}
	}
}

func (t *WebSocketTransport) sendLine(line string) bool {
	t.mu.Lock()
	conn := t.conn
	state := t.state
	t.mu.Unlock()

	if conn == nil || state != WSStateConnected {
		t.log("WebSocketTransport: Not connected")
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := conn.Write(ctx, websocket.MessageText, []byte(line))
	if err != nil {
		t.log(fmt.Sprintf("WebSocketTransport: Failed to send: %s", err))
		t.handleConnectionError(0)
		return false
	}

	t.mu.Lock()
	t.lastActivityTime = t.now()
	t.mu.Unlock()
	return true
}

func (t *WebSocketTransport) doDisconnect() {
	t.stopPingInterval()
	t.stopKeepaliveInterval()

	t.mu.Lock()
	conn := t.conn
	t.conn = nil
	cancel := t.connCancel
	t.connCancel = nil
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "")
	}
}

// ---------------------------------------------------------------------------
// Connection error handling and reconnection
// ---------------------------------------------------------------------------

func (t *WebSocketTransport) handleConnectionError(closeCode int) {
	t.log(fmt.Sprintf("WebSocketTransport: Disconnected (code %d)", closeCode))

	t.mu.Lock()
	if t.opts.IsBridge {
		msSinceActivity := int64(-1)
		if !t.lastActivityTime.IsZero() {
			msSinceActivity = t.now().Sub(t.lastActivityTime).Milliseconds()
		}
		wasConnected := t.state == WSStateConnected
		attempts := t.reconnectAttempts
		t.mu.Unlock()

		analytics.LogEvent("tengu_ws_transport_closed", analytics.EventMetadata{
			"closeCode":           closeCode,
			"msSinceLastActivity": msSinceActivity,
			"wasConnected":        wasConnected,
			"reconnectAttempts":   attempts,
		})

		t.mu.Lock()
	}
	t.mu.Unlock()

	t.doDisconnect()

	t.mu.Lock()
	if t.state == WSStateClosed {
		t.mu.Unlock()
		return
	}

	// Permanent close codes: don't retry.
	headersRefreshed := false
	if closeCode == 4003 && t.opts.RefreshHeaders != nil {
		freshHeaders := t.opts.RefreshHeaders()
		if freshHeaders["Authorization"] != t.opts.Headers["Authorization"] {
			for k, v := range freshHeaders {
				t.opts.Headers[k] = v
			}
			headersRefreshed = true
			t.log("WebSocketTransport: 4003 received but headers refreshed, scheduling reconnect")
		}
	}

	if closeCode != 0 && PermanentCloseCodes[closeCode] && !headersRefreshed {
		t.log(fmt.Sprintf("WebSocketTransport: Permanent close code %d, not reconnecting", closeCode))
		t.state = WSStateClosed
		cb := t.onCloseCb
		t.mu.Unlock()
		if cb != nil {
			cb(closeCode)
		}
		return
	}

	// When autoReconnect is disabled, go straight to closed.
	if !t.opts.autoReconnect() {
		t.state = WSStateClosed
		cb := t.onCloseCb
		t.mu.Unlock()
		if cb != nil {
			cb(closeCode)
		}
		return
	}

	// Schedule reconnection with exponential backoff.
	now := t.now()
	if !t.reconnectStartTimeSet {
		t.reconnectStartTime = now
		t.reconnectStartTimeSet = true
	}

	// Sleep detection: reset budget if gap since last attempt > threshold.
	if t.lastReconnectAttemptTimeSet &&
		now.Sub(t.lastReconnectAttemptTime) > SleepDetectionThresholdMS {
		gap := now.Sub(t.lastReconnectAttemptTime)
		t.log(fmt.Sprintf("WebSocketTransport: Detected system sleep (%ds gap), resetting reconnection budget",
			int(gap.Seconds())))
		t.reconnectStartTime = now
		t.reconnectStartTimeSet = true
		t.reconnectAttempts = 0
	}
	t.lastReconnectAttemptTime = now
	t.lastReconnectAttemptTimeSet = true

	elapsed := now.Sub(t.reconnectStartTime)
	if elapsed < DefaultReconnectGiveUpMS {
		// Clear existing reconnection timer.
		if t.reconnectTimer != nil {
			t.reconnectTimer.Stop()
			t.reconnectTimer = nil
		}

		// Refresh headers before reconnecting.
		if !headersRefreshed && t.opts.RefreshHeaders != nil {
			freshHeaders := t.opts.RefreshHeaders()
			for k, v := range freshHeaders {
				t.opts.Headers[k] = v
			}
			t.log("WebSocketTransport: Refreshed headers for reconnect")
		}

		t.state = WSStateReconnecting
		t.reconnectAttempts++

		baseDelay := float64(DefaultBaseReconnectDelay) * math.Pow(2, float64(t.reconnectAttempts-1))
		if baseDelay > float64(DefaultMaxReconnectDelay) {
			baseDelay = float64(DefaultMaxReconnectDelay)
		}
		// Add +/-25% jitter.
		delay := time.Duration(math.Max(0, baseDelay+baseDelay*0.25*(2*t.randFloat()-1)))

		attempt := t.reconnectAttempts
		elapsedSec := int(elapsed.Seconds())

		t.log(fmt.Sprintf("WebSocketTransport: Reconnecting in %dms (attempt %d, %ds elapsed)",
			delay.Milliseconds(), attempt, elapsedSec))

		if t.opts.IsBridge {
			t.mu.Unlock()
			analytics.LogEvent("tengu_ws_transport_reconnecting", analytics.EventMetadata{
				"attempt":   attempt,
				"elapsedMs": elapsed.Milliseconds(),
				"delayMs":   delay.Milliseconds(),
			})
			t.mu.Lock()
		}

		t.reconnectTimer = time.AfterFunc(delay, func() {
			t.mu.Lock()
			t.reconnectTimer = nil
			t.mu.Unlock()
			t.Connect()
		})
		t.mu.Unlock()
	} else {
		t.log(fmt.Sprintf("WebSocketTransport: Reconnection time budget exhausted after %ds",
			int(elapsed.Seconds())))
		t.state = WSStateClosed
		cb := t.onCloseCb
		t.mu.Unlock()
		if cb != nil {
			cb(closeCode)
		}
	}
}

// ---------------------------------------------------------------------------
// Message replay
// ---------------------------------------------------------------------------

func (t *WebSocketTransport) replayBufferedMessages(serverLastID string) {
	t.mu.Lock()
	messages := t.messageBuffer.ToArray()
	t.mu.Unlock()

	if len(messages) == 0 {
		return
	}

	startIndex := 0
	if serverLastID != "" {
		for i, msg := range messages {
			uuid := extractUUID(msg)
			if uuid == serverLastID {
				startIndex = i + 1
				break
			}
		}
		if startIndex > 0 {
			remaining := messages[startIndex:]
			t.mu.Lock()
			t.messageBuffer.Clear()
			t.messageBuffer.AddAll(remaining)
			if len(remaining) == 0 {
				t.lastSentID = ""
			}
			t.mu.Unlock()
			t.log(fmt.Sprintf("WebSocketTransport: Evicted %d confirmed messages, %d remaining",
				startIndex, len(remaining)))
		}
	}

	toReplay := messages[startIndex:]
	if len(toReplay) == 0 {
		t.log("WebSocketTransport: No new messages to replay")
		return
	}

	t.log(fmt.Sprintf("WebSocketTransport: Replaying %d buffered messages", len(toReplay)))
	for _, msg := range toReplay {
		line := string(msg) + "\n"
		if !t.sendLine(line) {
			t.handleConnectionError(0)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Ping interval — detects dead connections
// ---------------------------------------------------------------------------

func (t *WebSocketTransport) startPingInterval() {
	t.stopPingInterval()

	t.mu.Lock()
	t.pongReceived = true
	t.lastTickTime = t.now()
	t.mu.Unlock()

	ticker := time.NewTicker(DefaultPingInterval)
	done := make(chan struct{})

	t.mu.Lock()
	t.pingTicker = ticker
	t.pingDone = done
	t.mu.Unlock()

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				t.mu.Lock()
				state := t.state
				conn := t.conn
				now := t.now()
				gap := now.Sub(t.lastTickTime)
				t.lastTickTime = now
				t.mu.Unlock()

				if state != WSStateConnected || conn == nil {
					continue
				}

				// Sleep detection via tick gap.
				if gap > SleepDetectionThresholdMS {
					t.log(fmt.Sprintf("WebSocketTransport: %ds tick gap detected — process was suspended, forcing reconnect",
						int(gap.Seconds())))
					t.handleConnectionError(0)
					return
				}

				t.mu.Lock()
				pongOK := t.pongReceived
				t.pongReceived = false
				t.mu.Unlock()

				if !pongOK {
					t.log("WebSocketTransport: No pong received, connection appears dead")
					t.handleConnectionError(0)
					return
				}

				// Send ping.
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := conn.Ping(ctx)
				cancel()
				if err != nil {
					t.log(fmt.Sprintf("WebSocketTransport: Ping failed: %s", err))
				} else {
					t.mu.Lock()
					t.pongReceived = true
					t.mu.Unlock()
				}
			}
		}
	}()
}

func (t *WebSocketTransport) stopPingInterval() {
	t.mu.Lock()
	ticker := t.pingTicker
	done := t.pingDone
	t.pingTicker = nil
	t.pingDone = nil
	t.mu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}
	if done != nil {
		close(done)
	}
}

// ---------------------------------------------------------------------------
// Keepalive interval — resets proxy idle timers
// ---------------------------------------------------------------------------

func (t *WebSocketTransport) startKeepaliveInterval() {
	t.stopKeepaliveInterval()

	ticker := time.NewTicker(DefaultKeepaliveInterval)
	done := make(chan struct{})

	t.mu.Lock()
	t.keepaliveTicker = ticker
	t.keepaliveDone = done
	t.mu.Unlock()

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				t.mu.Lock()
				state := t.state
				conn := t.conn
				t.mu.Unlock()

				if state != WSStateConnected || conn == nil {
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := conn.Write(ctx, websocket.MessageText, []byte(KeepAliveFrame))
				cancel()
				if err != nil {
					t.log(fmt.Sprintf("WebSocketTransport: Periodic keep_alive failed: %s", err))
				} else {
					t.mu.Lock()
					t.lastActivityTime = t.now()
					t.mu.Unlock()
					t.log("WebSocketTransport: Sent periodic keep_alive data frame")
				}
			}
		}
	}()
}

func (t *WebSocketTransport) stopKeepaliveInterval() {
	t.mu.Lock()
	ticker := t.keepaliveTicker
	done := t.keepaliveDone
	t.keepaliveTicker = nil
	t.keepaliveDone = nil
	t.mu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}
	if done != nil {
		close(done)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractUUID pulls the "uuid" field from a JSON message.
func extractUUID(msg StdoutMessage) string {
	var envelope struct {
		UUID string `json:"uuid"`
	}
	if json.Unmarshal(msg, &envelope) == nil {
		return envelope.UUID
	}
	return ""
}

// extractCloseCode attempts to extract a WebSocket close code from an error.
func extractCloseCode(err error) int {
	status := websocket.CloseStatus(err)
	if status == -1 {
		return 0
	}
	return int(status)
}
