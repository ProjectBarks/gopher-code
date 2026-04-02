package mcp

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// Connection recovery constants.
// Source: services/mcp/useManageMCPConnections.ts:88-90
const (
	MaxReconnectAttempts = 5
	InitialBackoffMs    = 1000
	MaxBackoffMs        = 30000
)

// Error handling constants.
// Source: services/mcp/client.ts:1228, 457, 463
const (
	MaxErrorsBeforeReconnect = 3   // Source: client.ts:1228
	MCPConnectionTimeoutMs   = 30000 // Source: client.ts:457
	MCPRequestTimeoutMs      = 60000 // Source: client.ts:463
)

// JSON-RPC error codes from MCP spec.
const (
	JSONRPCSessionNotFound   = -32001 // Source: client.ts:200-204
	JSONRPCConnectionClosed  = -32000 // Source: client.ts:1236
)

// McpSessionExpiredError is returned when an MCP session is no longer valid.
// Source: services/mcp/client.ts:165-170
type McpSessionExpiredError struct {
	ServerName string
}

func (e *McpSessionExpiredError) Error() string {
	return fmt.Sprintf("MCP server %q session expired", e.ServerName)
}

// IsMcpSessionExpiredError detects whether an error is an MCP "Session not found"
// error (HTTP 404 + JSON-RPC code -32001).
// Source: services/mcp/client.ts:193-206
func IsMcpSessionExpiredError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Check for HTTP 404 combined with JSON-RPC -32001
	if !strings.Contains(msg, "404") {
		return false
	}
	return strings.Contains(msg, `"code":-32001`) ||
		strings.Contains(msg, `"code": -32001`)
}

// IsConnectionClosedError detects JSON-RPC -32000 "Connection closed" errors.
// Source: services/mcp/client.ts:3220
func IsConnectionClosedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "-32000") && strings.Contains(msg, "onnection closed")
}

// CalculateBackoff computes exponential backoff duration for a reconnection attempt.
// Source: services/mcp/useManageMCPConnections.ts:448-449
func CalculateBackoff(attempt int) time.Duration {
	backoffMs := float64(InitialBackoffMs) * math.Pow(2, float64(attempt-1))
	if backoffMs > float64(MaxBackoffMs) {
		backoffMs = float64(MaxBackoffMs)
	}
	return time.Duration(backoffMs) * time.Millisecond
}

// ConnectionState describes the lifecycle state of an MCP server connection.
// Source: services/mcp/types.ts
type ConnectionState string

const (
	StateConnected ConnectionState = "connected"
	StatePending   ConnectionState = "pending"
	StateFailed    ConnectionState = "failed"
	StateDisabled  ConnectionState = "disabled"
	StateNeedsAuth ConnectionState = "needs_auth"
)

// ReconnectResult captures the outcome of a reconnection attempt.
type ReconnectResult struct {
	Success  bool
	Attempt  int
	State    ConnectionState
	Error    error
}

// Reconnect attempts to reconnect an MCP server with exponential backoff.
// Source: services/mcp/useManageMCPConnections.ts:374-449
func (m *Manager) Reconnect(ctx context.Context, name string, cfg ServerConfig) *ReconnectResult {
	for attempt := 1; attempt <= MaxReconnectAttempts; attempt++ {
		// Try to connect
		err := m.Connect(ctx, name, cfg)
		if err == nil {
			return &ReconnectResult{
				Success: true,
				Attempt: attempt,
				State:   StateConnected,
			}
		}

		// On final attempt, give up
		if attempt == MaxReconnectAttempts {
			return &ReconnectResult{
				Success: false,
				Attempt: attempt,
				State:   StateFailed,
				Error:   fmt.Errorf("max reconnection attempts (%d) reached: %w", MaxReconnectAttempts, err),
			}
		}

		// Wait with exponential backoff before retrying
		backoff := CalculateBackoff(attempt)
		select {
		case <-time.After(backoff):
			// Continue to next attempt
		case <-ctx.Done():
			return &ReconnectResult{
				Success: false,
				Attempt: attempt,
				State:   StateFailed,
				Error:   ctx.Err(),
			}
		}
	}

	// Should not reach here
	return &ReconnectResult{Success: false, State: StateFailed}
}

// Disconnect closes a specific server connection and removes it from the manager.
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	client, ok := m.clients[name]
	if ok {
		delete(m.clients, name)
	}
	m.mu.Unlock()

	if ok && client != nil {
		return client.Close()
	}
	return nil
}

// IsConnected checks if a server is currently connected.
func (m *Manager) IsConnected(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.clients[name]
	return ok
}

// ServerNames returns the names of all connected servers.
func (m *Manager) ServerNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}
