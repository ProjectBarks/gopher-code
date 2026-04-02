package mcp

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Source: services/mcp/useManageMCPConnections.ts, services/mcp/client.ts

func TestRecoveryConstants(t *testing.T) {
	// Source: useManageMCPConnections.ts:88-90
	if MaxReconnectAttempts != 5 {
		t.Errorf("MaxReconnectAttempts = %d, want 5", MaxReconnectAttempts)
	}
	if InitialBackoffMs != 1000 {
		t.Errorf("InitialBackoffMs = %d, want 1000", InitialBackoffMs)
	}
	if MaxBackoffMs != 30000 {
		t.Errorf("MaxBackoffMs = %d, want 30000", MaxBackoffMs)
	}
}

func TestErrorConstants(t *testing.T) {
	// Source: client.ts:1228, 457, 463
	if MaxErrorsBeforeReconnect != 3 {
		t.Errorf("MaxErrorsBeforeReconnect = %d, want 3", MaxErrorsBeforeReconnect)
	}
	if MCPConnectionTimeoutMs != 30000 {
		t.Errorf("MCPConnectionTimeoutMs = %d, want 30000", MCPConnectionTimeoutMs)
	}
	if MCPRequestTimeoutMs != 60000 {
		t.Errorf("MCPRequestTimeoutMs = %d, want 60000", MCPRequestTimeoutMs)
	}
}

func TestJSONRPCErrorCodes(t *testing.T) {
	// Source: client.ts:200-204, 1236
	if JSONRPCSessionNotFound != -32001 {
		t.Errorf("JSONRPCSessionNotFound = %d, want -32001", JSONRPCSessionNotFound)
	}
	if JSONRPCConnectionClosed != -32000 {
		t.Errorf("JSONRPCConnectionClosed = %d, want -32000", JSONRPCConnectionClosed)
	}
}

func TestIsMcpSessionExpiredError(t *testing.T) {
	// Source: client.ts:193-206
	t.Run("nil_error", func(t *testing.T) {
		if IsMcpSessionExpiredError(nil) {
			t.Error("nil should not be session expired")
		}
	})

	t.Run("404_with_-32001_detected", func(t *testing.T) {
		// Source: client.ts:203 — checks for "code":-32001
		err := errors.New(`HTTP 404: {"error":{"code":-32001,"message":"Session not found"}}`)
		if !IsMcpSessionExpiredError(err) {
			t.Error("should detect 404 + -32001")
		}
	})

	t.Run("404_with_-32001_spaced", func(t *testing.T) {
		// Source: client.ts:204 — also checks for "code": -32001
		err := errors.New(`HTTP 404: {"error":{"code": -32001,"message":"Session not found"}}`)
		if !IsMcpSessionExpiredError(err) {
			t.Error("should detect 404 + spaced -32001")
		}
	})

	t.Run("404_without_-32001_not_detected", func(t *testing.T) {
		// Source: client.ts:198 — generic 404 is not session expired
		err := errors.New("HTTP 404: Not Found")
		if IsMcpSessionExpiredError(err) {
			t.Error("generic 404 should not be session expired")
		}
	})

	t.Run("other_error_not_detected", func(t *testing.T) {
		err := errors.New("connection refused")
		if IsMcpSessionExpiredError(err) {
			t.Error("non-404 should not be session expired")
		}
	})
}

func TestIsConnectionClosedError(t *testing.T) {
	// Source: client.ts:3220
	t.Run("connection_closed_detected", func(t *testing.T) {
		err := errors.New(`JSON-RPC error -32000: Connection closed`)
		if !IsConnectionClosedError(err) {
			t.Error("should detect -32000 Connection closed")
		}
	})

	t.Run("other_error_not_detected", func(t *testing.T) {
		err := errors.New("timeout")
		if IsConnectionClosedError(err) {
			t.Error("non-32000 should not match")
		}
	})
}

func TestCalculateBackoff(t *testing.T) {
	// Source: useManageMCPConnections.ts:448-449
	// backoff = INITIAL_BACKOFF_MS * Math.pow(2, attempt - 1), capped at MAX_BACKOFF_MS

	t.Run("attempt_1", func(t *testing.T) {
		// 1000 * pow(2, 0) = 1000ms
		d := CalculateBackoff(1)
		if d != 1*time.Second {
			t.Errorf("got %v, want 1s", d)
		}
	})

	t.Run("attempt_2", func(t *testing.T) {
		// 1000 * pow(2, 1) = 2000ms
		d := CalculateBackoff(2)
		if d != 2*time.Second {
			t.Errorf("got %v, want 2s", d)
		}
	})

	t.Run("attempt_3", func(t *testing.T) {
		// 1000 * pow(2, 2) = 4000ms
		d := CalculateBackoff(3)
		if d != 4*time.Second {
			t.Errorf("got %v, want 4s", d)
		}
	})

	t.Run("attempt_4", func(t *testing.T) {
		// 1000 * pow(2, 3) = 8000ms
		d := CalculateBackoff(4)
		if d != 8*time.Second {
			t.Errorf("got %v, want 8s", d)
		}
	})

	t.Run("attempt_5", func(t *testing.T) {
		// 1000 * pow(2, 4) = 16000ms
		d := CalculateBackoff(5)
		if d != 16*time.Second {
			t.Errorf("got %v, want 16s", d)
		}
	})

	t.Run("capped_at_max", func(t *testing.T) {
		// 1000 * pow(2, 9) = 512000ms > 30000ms cap
		d := CalculateBackoff(10)
		if d != 30*time.Second {
			t.Errorf("got %v, want 30s (capped)", d)
		}
	})
}

func TestConnectionState(t *testing.T) {
	// Source: services/mcp/types.ts
	if StateConnected != "connected" {
		t.Error("wrong")
	}
	if StatePending != "pending" {
		t.Error("wrong")
	}
	if StateFailed != "failed" {
		t.Error("wrong")
	}
	if StateDisabled != "disabled" {
		t.Error("wrong")
	}
	if StateNeedsAuth != "needs_auth" {
		t.Error("wrong")
	}
}

func TestMcpSessionExpiredError(t *testing.T) {
	// Source: client.ts:165-170
	err := &McpSessionExpiredError{ServerName: "test-server"}
	if !containsStr(err.Error(), "test-server") {
		t.Errorf("error should mention server name: %q", err.Error())
	}
	if !containsStr(err.Error(), "expired") {
		t.Errorf("error should mention expired: %q", err.Error())
	}
}

func TestManagerDisconnect(t *testing.T) {
	m := NewManager()

	// Disconnect non-existent server should be no-op
	err := m.Disconnect("nonexistent")
	if err != nil {
		t.Errorf("disconnect nonexistent should not error: %v", err)
	}
}

func TestManagerIsConnected(t *testing.T) {
	m := NewManager()

	if m.IsConnected("test") {
		t.Error("should not be connected before Connect")
	}
}

func TestManagerServerNames(t *testing.T) {
	m := NewManager()

	names := m.ServerNames()
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

func TestReconnectCancelledByContext(t *testing.T) {
	m := NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := m.Reconnect(ctx, "test", ServerConfig{Command: "nonexistent-command"})
	if result.Success {
		t.Error("should not succeed with cancelled context")
	}
	if result.State != StateFailed {
		t.Errorf("expected failed state, got %s", result.State)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
