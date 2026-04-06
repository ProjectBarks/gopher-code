// Package server — DirectConnectSessionManager: WebSocket session manager for
// direct-connect (CCR) mode. Sends/receives JSONL messages over a single
// WebSocket connection with optional Bearer auth and reconnect support.
//
// Source: src/server/directConnectManager.ts
// Tasks: T96 (manager), T97 (JSONL parse), T98 (Bearer auth), T99 (reconnect)
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// ---------------------------------------------------------------------------
// Config + Callbacks
// ---------------------------------------------------------------------------

// DirectConnectConfig holds the parameters needed to establish a direct-connect
// WebSocket session. Returned by CreateDirectConnectSession.
type DirectConnectConfig struct {
	ServerURL string `json:"serverUrl"`
	SessionID string `json:"sessionId"`
	WSURL     string `json:"wsUrl"`
	AuthToken string `json:"authToken,omitempty"`
}

// DirectConnectCallbacks are the event handlers invoked by the session manager.
// OnMessage and OnPermissionRequest are required; the rest are optional.
type DirectConnectCallbacks struct {
	// OnMessage is called for every SDK message that is not a control/keep-alive.
	OnMessage func(msg json.RawMessage)
	// OnPermissionRequest is called for control_request messages with subtype can_use_tool.
	OnPermissionRequest func(request json.RawMessage, requestID string)
	// OnConnected fires when the WebSocket connection is established.
	OnConnected func()
	// OnDisconnected fires when the WebSocket connection closes.
	OnDisconnected func()
	// OnError fires on WebSocket errors.
	OnError func(err error)
}

// ---------------------------------------------------------------------------
// StdoutMessage type guard
// ---------------------------------------------------------------------------

// stdoutEnvelope is the minimal shape we need to route incoming JSONL messages.
type stdoutEnvelope struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Request json.RawMessage `json:"request,omitempty"`
	// For control_request routing:
	RequestID string `json:"request_id,omitempty"`
}

// controlRequest is the nested request inside a control_request envelope.
type controlRequest struct {
	Subtype string `json:"subtype"`
}

// isStdoutMessage mirrors the TS isStdoutMessage type guard:
// typeof == 'object' && 'type' in value && typeof value.type == 'string'.
func isStdoutMessage(env *stdoutEnvelope) bool {
	return env.Type != ""
}

// ---------------------------------------------------------------------------
// DirectConnectSessionManager — T96
// ---------------------------------------------------------------------------

// DirectConnectSessionManager manages a WebSocket connection to a direct-connect
// server. It reads JSONL frames (T97), routes messages to callbacks, and
// supports Bearer auth (T98) and reconnect-on-disconnect (T99).
type DirectConnectSessionManager struct {
	config    DirectConnectConfig
	callbacks DirectConnectCallbacks

	mu   sync.Mutex
	conn *websocket.Conn

	// reconnect settings (T99)
	reconnect        bool
	reconnectBackoff time.Duration
	maxBackoff       time.Duration

	// cancel is set when connect() starts the read loop.
	cancel context.CancelFunc
}

// NewDirectConnectSessionManager creates a manager that will connect to the
// configured WebSocket URL when Connect is called.
func NewDirectConnectSessionManager(cfg DirectConnectConfig, cb DirectConnectCallbacks) *DirectConnectSessionManager {
	return &DirectConnectSessionManager{
		config:           cfg,
		callbacks:        cb,
		reconnect:        false,
		reconnectBackoff: 1 * time.Second,
		maxBackoff:       30 * time.Second,
	}
}

// SetReconnect enables automatic reconnect on disconnect with exponential backoff.
func (m *DirectConnectSessionManager) SetReconnect(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconnect = enabled
}

// Connect opens the WebSocket connection and starts the message read loop.
// The provided context controls the lifetime of the connection.
func (m *DirectConnectSessionManager) Connect(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancel = cancel
	m.mu.Unlock()

	return m.dial(ctx)
}

func (m *DirectConnectSessionManager) dial(ctx context.Context) error {
	// T98: Bearer auth header
	opts := &websocket.DialOptions{}
	if m.config.AuthToken != "" {
		opts.HTTPHeader = http.Header{
			"Authorization": []string{"Bearer " + m.config.AuthToken},
		}
	}

	conn, _, err := websocket.Dial(ctx, m.config.WSURL, opts)
	if err != nil {
		if m.callbacks.OnError != nil {
			m.callbacks.OnError(err)
		}
		return err
	}

	m.mu.Lock()
	m.conn = conn
	m.mu.Unlock()

	if m.callbacks.OnConnected != nil {
		m.callbacks.OnConnected()
	}

	// Start read loop in background
	go m.readLoop(ctx)
	return nil
}

// readLoop reads WebSocket text frames, splits on newlines, and routes JSONL
// messages to the appropriate callbacks (T97).
func (m *DirectConnectSessionManager) readLoop(ctx context.Context) {
	backoff := m.reconnectBackoff
	for {
		_, data, err := m.conn.Read(ctx)
		if err != nil {
			// Connection closed or error
			if m.callbacks.OnDisconnected != nil {
				m.callbacks.OnDisconnected()
			}
			if errors.Is(err, context.Canceled) {
				return
			}

			// T99: reconnect logic
			m.mu.Lock()
			shouldReconnect := m.reconnect
			m.mu.Unlock()

			if shouldReconnect {
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				// Exponential backoff, capped
				backoff = backoff * 2
				if backoff > m.maxBackoff {
					backoff = m.maxBackoff
				}
				if dialErr := m.dial(ctx); dialErr != nil {
					continue
				}
				return // new readLoop started by dial
			}
			return
		}

		// Reset backoff on successful read
		backoff = m.reconnectBackoff

		// T97: JSONL parsing — split on newlines, parse each non-empty line
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var env stdoutEnvelope
			if err := json.Unmarshal([]byte(line), &env); err != nil {
				continue // skip unparsable lines, matching TS behavior
			}

			if !isStdoutMessage(&env) {
				continue
			}

			raw := json.RawMessage(line)

			// Route control_request messages
			if env.Type == "control_request" {
				var cr controlRequest
				if env.Request != nil {
					_ = json.Unmarshal(env.Request, &cr)
				}
				if cr.Subtype == "can_use_tool" {
					if m.callbacks.OnPermissionRequest != nil {
						m.callbacks.OnPermissionRequest(env.Request, env.RequestID)
					}
				} else {
					// Send error response for unrecognized subtypes
					m.sendErrorResponse(env.RequestID, "Unsupported control request subtype: "+cr.Subtype)
				}
				continue
			}

			// Filter out non-SDK message types (matching TS logic)
			if env.Type == "control_response" ||
				env.Type == "keep_alive" ||
				env.Type == "control_cancel_request" ||
				env.Type == "streamlined_text" ||
				env.Type == "streamlined_tool_use_summary" ||
				(env.Type == "system" && env.Subtype == "post_turn_summary") {
				continue
			}

			if m.callbacks.OnMessage != nil {
				m.callbacks.OnMessage(raw)
			}
		}
	}
}

// SendMessage sends a user message over the WebSocket.
// Returns false if the connection is not open.
func (m *DirectConnectSessionManager) SendMessage(ctx context.Context, content json.RawMessage) bool {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return false
	}

	msg, _ := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
		"parent_tool_use_id": nil,
		"session_id":         "",
	})

	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		return false
	}
	return true
}

// RespondToPermissionRequest sends a permission response over the WebSocket.
func (m *DirectConnectSessionManager) RespondToPermissionRequest(ctx context.Context, requestID string, behavior string, updatedInput json.RawMessage, message string) {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return
	}

	resp := map[string]any{
		"subtype":    "success",
		"request_id": requestID,
	}
	inner := map[string]any{
		"behavior": behavior,
	}
	if behavior == "allow" && updatedInput != nil {
		inner["updatedInput"] = updatedInput
	} else if behavior != "allow" {
		inner["message"] = message
	}
	resp["response"] = inner

	msg, _ := json.Marshal(map[string]any{
		"type":     "control_response",
		"response": resp,
	})

	_ = conn.Write(ctx, websocket.MessageText, msg)
}

// SendInterrupt sends an interrupt control request to cancel the current operation.
func (m *DirectConnectSessionManager) SendInterrupt(ctx context.Context) {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return
	}

	msg, _ := json.Marshal(map[string]any{
		"type":       "control_request",
		"request_id": time.Now().UnixNano(), // unique ID
		"request": map[string]string{
			"subtype": "interrupt",
		},
	})

	_ = conn.Write(ctx, websocket.MessageText, msg)
}

func (m *DirectConnectSessionManager) sendErrorResponse(requestID string, errMsg string) {
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	if conn == nil {
		return
	}

	msg, _ := json.Marshal(map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "error",
			"request_id": requestID,
			"error":      errMsg,
		},
	})

	_ = conn.Write(context.Background(), websocket.MessageText, msg)
}

// Disconnect closes the WebSocket connection.
func (m *DirectConnectSessionManager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.conn != nil {
		m.conn.Close(websocket.StatusNormalClosure, "client disconnect")
		m.conn = nil
	}
}

// IsConnected returns true if the WebSocket connection is currently open.
func (m *DirectConnectSessionManager) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.conn != nil
}
