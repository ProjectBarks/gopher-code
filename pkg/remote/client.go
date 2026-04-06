// Package remote — WebSocket client for CCR remote sessions.
// Source: src/remote/SessionsWebSocket.ts
package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// T82: WebSocket/SSE remote client
// Source: SessionsWebSocket.ts
// ---------------------------------------------------------------------------

const (
	// reconnectDelay is the base delay between reconnection attempts.
	// Source: SessionsWebSocket.ts:17
	reconnectDelay = 2 * time.Second

	// maxReconnectAttempts is the maximum number of reconnection attempts.
	// Source: SessionsWebSocket.ts:18
	maxReconnectAttempts = 5

	// pingInterval is the keepalive ping cadence.
	// Source: SessionsWebSocket.ts:19
	pingInterval = 30 * time.Second

	// maxSessionNotFoundRetries is the retry limit for 4001 (session not found).
	// During compaction the server may briefly consider the session stale.
	// Source: SessionsWebSocket.ts:26
	maxSessionNotFoundRetries = 3
)

// permanentCloseCodes are WebSocket close codes that stop reconnection.
// Source: SessionsWebSocket.ts:34-36
var permanentCloseCodes = map[int]bool{
	4003: true, // unauthorized
}

// wsState represents the WebSocket connection lifecycle.
type wsState int

const (
	wsStateClosed     wsState = iota
	wsStateConnecting
	wsStateConnected
)

// SessionsWSCallbacks defines event handlers for the WebSocket client.
// Source: SessionsWebSocket.ts:57-65
type SessionsWSCallbacks struct {
	// OnMessage is called for each decoded message from the WebSocket.
	OnMessage func(raw json.RawMessage)
	// OnClose is called on permanent disconnect (server ended or retries exhausted).
	OnClose func()
	// OnError is called on connection or protocol errors.
	OnError func(err error)
	// OnConnected is called when the connection is established.
	OnConnected func()
	// OnReconnecting is called when a transient close triggers a reconnect.
	OnReconnecting func()
}

// SessionsWebSocket is a WebSocket client for CCR remote session streaming.
// It implements the WebSocketClient interface used by RemoteSessionManager.
//
// Protocol:
//  1. Connect to wss://<host>/v1/sessions/ws/{sessionId}/subscribe?organization_uuid=...
//  2. Auth via Authorization header (Bearer token)
//  3. Receive JSON messages from the session
//
// Source: SessionsWebSocket.ts:82-404
type SessionsWebSocket struct {
	sessionID      string
	orgUUID        string
	getAccessToken func() string
	callbacks      SessionsWSCallbacks
	baseAPIURL     string

	mu                     sync.Mutex
	state                  wsState
	conn                   *websocket.Conn
	reconnectAttempts      int
	sessionNotFoundRetries int
	reconnectTimer         *time.Timer
	pingDone               chan struct{}
	connCancel             context.CancelFunc

	// Overridable for testing.
	dialFunc func(ctx context.Context, url string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error)
	now      func() time.Time
}

// SessionsWebSocketOpts configures a SessionsWebSocket.
type SessionsWebSocketOpts struct {
	SessionID      string
	OrgUUID        string
	GetAccessToken func() string
	Callbacks      SessionsWSCallbacks
	// BaseAPIURL is the HTTPS base URL (e.g. "https://api.anthropic.com").
	// Replaced with wss:// for WebSocket connections.
	BaseAPIURL string
	// DialFunc overrides websocket.Dial for testing.
	DialFunc func(ctx context.Context, url string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error)
	// Now overrides time.Now for testing.
	Now func() time.Time
}

// NewSessionsWebSocket creates a new WebSocket client.
func NewSessionsWebSocket(opts SessionsWebSocketOpts) *SessionsWebSocket {
	dial := opts.DialFunc
	if dial == nil {
		dial = websocket.Dial
	}
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	baseURL := opts.BaseAPIURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &SessionsWebSocket{
		sessionID:      opts.SessionID,
		orgUUID:        opts.OrgUUID,
		getAccessToken: opts.GetAccessToken,
		callbacks:      opts.Callbacks,
		baseAPIURL:     baseURL,
		state:          wsStateClosed,
		dialFunc:       dial,
		now:            nowFn,
	}
}

// ---------------------------------------------------------------------------
// WebSocketClient interface implementation
// ---------------------------------------------------------------------------

// Connect opens the WebSocket connection.
// Source: SessionsWebSocket.ts:100-205
func (s *SessionsWebSocket) Connect() error {
	s.mu.Lock()
	if s.state == wsStateConnecting {
		s.mu.Unlock()
		return nil
	}
	s.state = wsStateConnecting
	s.mu.Unlock()

	return s.doConnect()
}

// Close shuts down the connection and stops reconnection.
// Source: SessionsWebSocket.ts:369-387
func (s *SessionsWebSocket) Close() {
	slog.Debug("sessions websocket: closing")

	s.mu.Lock()
	s.state = wsStateClosed
	if s.reconnectTimer != nil {
		s.reconnectTimer.Stop()
		s.reconnectTimer = nil
	}
	conn := s.conn
	s.conn = nil
	cancel := s.connCancel
	s.connCancel = nil
	s.mu.Unlock()

	s.stopPing()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "")
	}
}

// Reconnect forces a reconnect. Resets retry counters.
// Source: SessionsWebSocket.ts:393-403
func (s *SessionsWebSocket) Reconnect() {
	slog.Debug("sessions websocket: force reconnect")

	s.mu.Lock()
	s.reconnectAttempts = 0
	s.sessionNotFoundRetries = 0
	s.mu.Unlock()

	s.Close()

	// Small delay before reconnecting.
	s.mu.Lock()
	s.reconnectTimer = time.AfterFunc(500*time.Millisecond, func() {
		s.mu.Lock()
		s.reconnectTimer = nil
		s.mu.Unlock()
		_ = s.Connect()
	})
	s.mu.Unlock()
}

// IsConnected returns true if the WebSocket is healthy.
// Source: SessionsWebSocket.ts:362-364
func (s *SessionsWebSocket) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == wsStateConnected
}

// SendControlResponse sends a control response to CCR.
// Source: SessionsWebSocket.ts:328-336
func (s *SessionsWebSocket) SendControlResponse(resp SDKControlResponse) error {
	s.mu.Lock()
	conn := s.conn
	state := s.state
	s.mu.Unlock()

	if conn == nil || state != wsStateConnected {
		return fmt.Errorf("sessions websocket: not connected")
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("sessions websocket: marshal response: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

// SendControlRequest sends a control request to CCR (e.g. interrupt).
// Source: SessionsWebSocket.ts:341-357
func (s *SessionsWebSocket) SendControlRequest(inner map[string]any) error {
	s.mu.Lock()
	conn := s.conn
	state := s.state
	s.mu.Unlock()

	if conn == nil || state != wsStateConnected {
		return fmt.Errorf("sessions websocket: not connected")
	}

	req := SDKControlRequest{
		Type:      "control_request",
		RequestID: uuid.New().String(),
		Request: SDKControlRequestInner{
			Subtype: inner["subtype"].(string),
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("sessions websocket: marshal request: %w", err)
	}

	slog.Debug("sessions websocket: sending control request", "subtype", req.Request.Subtype)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

// ---------------------------------------------------------------------------
// Internal connection lifecycle
// ---------------------------------------------------------------------------

func (s *SessionsWebSocket) doConnect() error {
	// Build WebSocket URL.
	wsURL := replaceScheme(s.baseAPIURL) +
		"/v1/sessions/ws/" + s.sessionID +
		"/subscribe?organization_uuid=" + s.orgUUID

	slog.Debug("sessions websocket: connecting", "url", wsURL)

	token := s.getAccessToken()
	headers := http.Header{
		"Authorization":     []string{"Bearer " + token},
		"Anthropic-Version": []string{"2023-06-01"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.connCancel = cancel
	s.mu.Unlock()

	conn, _, err := s.dialFunc(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		cancel()
		slog.Error("sessions websocket: dial failed", "err", err)
		if s.callbacks.OnError != nil {
			s.callbacks.OnError(fmt.Errorf("sessions websocket: dial: %w", err))
		}
		s.handleClose(0)
		return fmt.Errorf("sessions websocket: dial: %w", err)
	}

	// Disable the default read limit (32KB) to accept large messages.
	conn.SetReadLimit(-1)

	s.mu.Lock()
	s.conn = conn
	s.state = wsStateConnected
	s.reconnectAttempts = 0
	s.sessionNotFoundRetries = 0
	s.mu.Unlock()

	slog.Debug("sessions websocket: connected")
	if s.callbacks.OnConnected != nil {
		s.callbacks.OnConnected()
	}

	s.startPing()

	// Start read loop in background.
	go s.readLoop(ctx, conn)

	return nil
}

func (s *SessionsWebSocket) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // normal shutdown
			}
			slog.Debug("sessions websocket: read error", "err", err)
			closeCode := extractWSCloseCode(err)
			s.handleClose(closeCode)
			return
		}

		// Validate it's a JSON object with a "type" field.
		var envelope struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &envelope) != nil || envelope.Type == "" {
			slog.Debug("sessions websocket: ignoring non-typed message")
			continue
		}

		if s.callbacks.OnMessage != nil {
			s.callbacks.OnMessage(json.RawMessage(data))
		}
	}
}

// handleClose processes a WebSocket close event with reconnection logic.
// Source: SessionsWebSocket.ts:234-298
func (s *SessionsWebSocket) handleClose(closeCode int) {
	s.stopPing()

	s.mu.Lock()
	if s.state == wsStateClosed {
		s.mu.Unlock()
		return
	}

	previousState := s.state
	s.conn = nil
	s.state = wsStateClosed

	// Cancel read loop context.
	cancel := s.connCancel
	s.connCancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	// Permanent close codes: stop reconnecting.
	if permanentCloseCodes[closeCode] {
		slog.Debug("sessions websocket: permanent close code", "code", closeCode)
		if s.callbacks.OnClose != nil {
			s.callbacks.OnClose()
		}
		return
	}

	// 4001 (session not found) — limited retries for transient compaction state.
	if closeCode == 4001 {
		s.mu.Lock()
		s.sessionNotFoundRetries++
		retries := s.sessionNotFoundRetries
		s.mu.Unlock()

		if retries > maxSessionNotFoundRetries {
			slog.Debug("sessions websocket: 4001 retry budget exhausted")
			if s.callbacks.OnClose != nil {
				s.callbacks.OnClose()
			}
			return
		}
		s.scheduleReconnect(reconnectDelay * time.Duration(retries))
		return
	}

	// Attempt reconnection if we were connected.
	s.mu.Lock()
	attempts := s.reconnectAttempts
	s.mu.Unlock()

	if previousState == wsStateConnected && attempts < maxReconnectAttempts {
		s.mu.Lock()
		s.reconnectAttempts++
		s.mu.Unlock()
		s.scheduleReconnect(reconnectDelay)
	} else {
		slog.Debug("sessions websocket: not reconnecting")
		if s.callbacks.OnClose != nil {
			s.callbacks.OnClose()
		}
	}
}

func (s *SessionsWebSocket) scheduleReconnect(delay time.Duration) {
	if s.callbacks.OnReconnecting != nil {
		s.callbacks.OnReconnecting()
	}

	slog.Debug("sessions websocket: scheduling reconnect", "delay", delay)

	s.mu.Lock()
	s.reconnectTimer = time.AfterFunc(delay, func() {
		s.mu.Lock()
		s.reconnectTimer = nil
		s.mu.Unlock()
		_ = s.Connect()
	})
	s.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Ping/pong keepalive
// Source: SessionsWebSocket.ts:301-323
// ---------------------------------------------------------------------------

func (s *SessionsWebSocket) startPing() {
	s.stopPing()

	done := make(chan struct{})
	s.mu.Lock()
	s.pingDone = done
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.mu.Lock()
				conn := s.conn
				state := s.state
				s.mu.Unlock()

				if conn == nil || state != wsStateConnected {
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := conn.Ping(ctx)
				cancel()
				if err != nil {
					slog.Debug("sessions websocket: ping failed", "err", err)
					// Don't treat ping failure as fatal; the read loop will
					// detect the dead connection.
				}
			}
		}
	}()
}

func (s *SessionsWebSocket) stopPing() {
	s.mu.Lock()
	done := s.pingDone
	s.pingDone = nil
	s.mu.Unlock()

	if done != nil {
		close(done)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// replaceScheme converts https:// to wss:// and http:// to ws://.
func replaceScheme(u string) string {
	if len(u) > 8 && u[:8] == "https://" {
		return "wss://" + u[8:]
	}
	if len(u) > 7 && u[:7] == "http://" {
		return "ws://" + u[7:]
	}
	return u
}

// extractWSCloseCode extracts a WebSocket close code from an error.
func extractWSCloseCode(err error) int {
	status := websocket.CloseStatus(err)
	if status == -1 {
		return 0
	}
	return int(status)
}
