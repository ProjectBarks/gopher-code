package bridge

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestBridge(t *testing.T, sessionID string) *ReplBridge {
	t.Helper()
	return NewReplBridge(ReplBridgeConfig{
		SessionID: sessionID,
	})
}

// ---------------------------------------------------------------------------
// Test: dispatch user input via handle
// ---------------------------------------------------------------------------

func TestReplBridgeHandle_DispatchUserInput(t *testing.T) {
	rb := newTestBridge(t, "cse_user1")
	h := NewReplBridgeHandle(rb)

	payload, _ := json.Marshal(map[string]string{"content": "hello"})
	msg := SDKMessage{
		Type:    "user",
		UUID:    "uuid-1",
		Payload: payload,
	}

	h.WriteMessages([]SDKMessage{msg})

	// The message should appear on the outbound channel with session ID stamped.
	select {
	case out := <-rb.OutboundMessages():
		if out.Type != "user" {
			t.Fatalf("expected type 'user', got %q", out.Type)
		}
		if out.SessionID != "cse_user1" {
			t.Fatalf("expected session ID 'cse_user1', got %q", out.SessionID)
		}
		if out.UUID != "uuid-1" {
			t.Fatalf("expected UUID 'uuid-1', got %q", out.UUID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for outbound message")
	}
}

// ---------------------------------------------------------------------------
// Test: dispatch permission response via handle
// ---------------------------------------------------------------------------

func TestReplBridgeHandle_DispatchPermissionResponse(t *testing.T) {
	rb := newTestBridge(t, "cse_perm1")
	h := NewReplBridgeHandle(rb)

	payload, _ := json.Marshal(map[string]any{
		"response": map[string]any{
			"subtype":    "permission",
			"request_id": "req-42",
			"response":   map[string]any{"approved": true},
		},
	})
	resp := SDKMessage{
		UUID:    "uuid-perm",
		Payload: payload,
	}

	h.SendControlResponse(resp)

	select {
	case out := <-rb.OutboundMessages():
		if out.Type != "control_response" {
			t.Fatalf("expected type 'control_response', got %q", out.Type)
		}
		if out.SessionID != "cse_perm1" {
			t.Fatalf("expected session ID 'cse_perm1', got %q", out.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for permission response")
	}
}

// ---------------------------------------------------------------------------
// Test: handle close tears down bridge and clears singleton
// ---------------------------------------------------------------------------

func TestReplBridgeHandle_Close(t *testing.T) {
	rb := newTestBridge(t, "cse_close1")
	h := NewReplBridgeHandle(rb)

	// Set global handle.
	SetReplBridgeHandle(h)
	if got := GetReplBridgeHandle(); got != h {
		t.Fatalf("expected handle to be set, got %v", got)
	}

	// Close should teardown bridge and clear singleton.
	h.Close()

	// The bridge's Done channel should be closed.
	select {
	case <-rb.Done():
		// ok
	case <-time.After(time.Second):
		t.Fatal("bridge Done channel not closed after Close()")
	}

	// Global handle should be nil.
	if got := GetReplBridgeHandle(); got != nil {
		t.Fatal("expected global handle to be nil after Close()")
	}
}

// ---------------------------------------------------------------------------
// Test: set/get singleton and compat ID
// ---------------------------------------------------------------------------

func TestSetGetReplBridgeHandle(t *testing.T) {
	// Ensure clean state.
	SetReplBridgeHandle(nil)

	if got := GetReplBridgeHandle(); got != nil {
		t.Fatal("expected nil handle initially")
	}
	if got := GetSelfBridgeCompatID(); got != "" {
		t.Fatalf("expected empty compat ID, got %q", got)
	}

	rb := newTestBridge(t, "cse_abc123")
	h := NewReplBridgeHandle(rb)
	SetReplBridgeHandle(h)

	if got := GetReplBridgeHandle(); got != h {
		t.Fatal("expected handle to match")
	}
	if got := GetSelfBridgeCompatID(); got != "session_abc123" {
		t.Fatalf("expected 'session_abc123', got %q", got)
	}

	// Clear.
	SetReplBridgeHandle(nil)
	if got := GetReplBridgeHandle(); got != nil {
		t.Fatal("expected nil after clear")
	}
	if got := GetSelfBridgeCompatID(); got != "" {
		t.Fatalf("expected empty compat ID after clear, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Test: fire-and-forget session bridge ID update
// ---------------------------------------------------------------------------

func TestSetReplBridgeHandle_FiresSessionBridgeIDUpdate(t *testing.T) {
	// Reset updater after test.
	defer SetSessionBridgeIDUpdater(nil)

	var calls atomic.Int32
	var lastID atomic.Value
	lastID.Store("")

	SetSessionBridgeIDUpdater(func(compatID string) {
		calls.Add(1)
		lastID.Store(compatID)
	})

	rb := newTestBridge(t, "cse_fire1")
	h := NewReplBridgeHandle(rb)
	SetReplBridgeHandle(h)

	// Wait for the goroutine to fire.
	deadline := time.After(time.Second)
	for {
		if calls.Load() >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for session bridge ID update")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	if got := lastID.Load().(string); got != "session_fire1" {
		t.Fatalf("expected compat ID 'session_fire1', got %q", got)
	}

	// Clear — should fire again with "".
	SetReplBridgeHandle(nil)

	deadline = time.After(time.Second)
	for {
		if calls.Load() >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for clear update")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	if got := lastID.Load().(string); got != "" {
		t.Fatalf("expected empty compat ID on clear, got %q", got)
	}
}
