package bridge

import (
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// Integration tests — exercise ReplBridge construction and basic operations
// as wired by cmd/gopher-code/main.go's remote-control path (T195).
// ---------------------------------------------------------------------------

// TestReplBridge_Construction_BinaryWiring verifies that NewReplBridge can be
// constructed with the same callback pattern used in the remote-control handler,
// and that the resulting bridge is in the expected initial state.
func TestReplBridge_Construction_BinaryWiring(t *testing.T) {
	var stateChanges []BridgeState
	var debugMsgs []string

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID:     "sess-001",
		EnvironmentID: "env-001",
		OnStateChange: func(state BridgeState, detail string) {
			stateChanges = append(stateChanges, state)
		},
		OnInboundMessage: func(msg SDKMessage) {},
		OnPermissionResponse: func(response SDKMessage) {},
		OnInterrupt:          func() {},
		OnSetModel:           func(model string) {},
		OnDebug: func(msg string) {
			debugMsgs = append(debugMsgs, msg)
		},
	})

	if rb == nil {
		t.Fatal("NewReplBridge returned nil")
	}

	// Verify initial state.
	if got := rb.State(); got != BridgeStateReady {
		t.Errorf("initial state = %q, want %q", got, BridgeStateReady)
	}

	// Verify accessors.
	if got := rb.SessionID(); got != "sess-001" {
		t.Errorf("SessionID() = %q, want %q", got, "sess-001")
	}
	if got := rb.EnvironmentID(); got != "env-001" {
		t.Errorf("EnvironmentID() = %q, want %q", got, "env-001")
	}

	// Verify SSE sequence number defaults to 0.
	if got := rb.GetSSESequenceNum(); got != 0 {
		t.Errorf("GetSSESequenceNum() = %d, want 0", got)
	}
}

// TestReplBridge_WriteAndRead_InboundOutbound verifies that messages can be
// written to the outbound channel and inbound messages can be received.
func TestReplBridge_WriteAndRead_InboundOutbound(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{
		SessionID:     "sess-002",
		EnvironmentID: "env-002",
		OnDebug:       func(msg string) {},
	})

	// Write an outbound message.
	rb.WriteMessage(SDKMessage{
		Type: "assistant",
		UUID: "uuid-out-1",
	})

	// Read it from the outbound channel.
	select {
	case msg := <-rb.OutboundMessages():
		if msg.Type != "assistant" {
			t.Errorf("outbound msg type = %q, want %q", msg.Type, "assistant")
		}
		if msg.SessionID != "sess-002" {
			t.Errorf("outbound msg session_id = %q, want %q", msg.SessionID, "sess-002")
		}
	default:
		t.Fatal("expected outbound message, got none")
	}

	// Deliver an inbound message through HandleInboundMessage.
	rb.HandleInboundMessage(SDKMessage{
		Type: "user",
		UUID: "uuid-in-1",
	})

	// Read it from the inbound channel.
	select {
	case msg := <-rb.InboundMessages():
		if msg.Type != "user" {
			t.Errorf("inbound msg type = %q, want %q", msg.Type, "user")
		}
	default:
		t.Fatal("expected inbound message, got none")
	}
}

// TestReplBridge_ControlRequest_Routing verifies that control requests are
// routed to the correct callbacks (interrupt, set_model).
func TestReplBridge_ControlRequest_Routing(t *testing.T) {
	var interrupted atomic.Bool
	var modelSet atomic.Value

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-003",
		OnInterrupt: func() {
			interrupted.Store(true)
		},
		OnSetModel: func(model string) {
			modelSet.Store(model)
		},
		OnDebug: func(msg string) {},
	})

	// Send an interrupt control request.
	rb.HandleInboundMessage(SDKMessage{
		Type:    "control_request",
		Payload: []byte(`{"request":{"subtype":"interrupt"}}`),
	})
	if !interrupted.Load() {
		t.Error("expected OnInterrupt to be called")
	}

	// Send a set_model control request.
	rb.HandleInboundMessage(SDKMessage{
		Type:    "control_request",
		Payload: []byte(`{"request":{"subtype":"set_model","model":"claude-opus-4-6"}}`),
	})
	if got, ok := modelSet.Load().(string); !ok || got != "claude-opus-4-6" {
		t.Errorf("OnSetModel model = %q, want %q", got, "claude-opus-4-6")
	}
}

// TestReplBridge_SSESequenceNum verifies get/set of the SSE sequence number.
func TestReplBridge_SSESequenceNum_Integration(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{
		SessionID:             "sess-004",
		InitialSSESequenceNum: 42,
		OnDebug:               func(msg string) {},
	})

	if got := rb.GetSSESequenceNum(); got != 42 {
		t.Errorf("initial SSE seq = %d, want 42", got)
	}

	rb.SetSSESequenceNum(99)
	if got := rb.GetSSESequenceNum(); got != 99 {
		t.Errorf("updated SSE seq = %d, want 99", got)
	}
}

// TestReplBridge_Teardown verifies graceful shutdown and Done channel closure.
func TestReplBridge_Teardown(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-005",
		OnStateChange: func(state BridgeState, detail string) {},
		OnDebug:       func(msg string) {},
	})

	// Teardown should close the Done channel.
	rb.Teardown()

	select {
	case <-rb.Done():
		// OK — channel closed as expected.
	default:
		t.Fatal("Done() channel not closed after Teardown")
	}

	// State should be failed after teardown.
	if got := rb.State(); got != BridgeStateFailed {
		t.Errorf("state after teardown = %q, want %q", got, BridgeStateFailed)
	}

	// Double teardown should be safe (idempotent).
	rb.Teardown()
}

// TestReplBridge_Deduplication verifies that duplicate UUIDs are dropped.
func TestReplBridge_Deduplication(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-006",
		OnDebug:   func(msg string) {},
	})

	// Write the same UUID twice — second should be dropped.
	rb.WriteMessage(SDKMessage{Type: "assistant", UUID: "dup-uuid-1"})
	rb.WriteMessage(SDKMessage{Type: "assistant", UUID: "dup-uuid-1"})

	// Should only get one message.
	<-rb.OutboundMessages()
	select {
	case msg := <-rb.OutboundMessages():
		t.Errorf("expected duplicate to be dropped, got message type=%s uuid=%s", msg.Type, msg.UUID)
	default:
		// OK — duplicate was dropped.
	}

	// Same for inbound.
	rb.HandleInboundMessage(SDKMessage{Type: "user", UUID: "dup-uuid-2"})
	rb.HandleInboundMessage(SDKMessage{Type: "user", UUID: "dup-uuid-2"})

	<-rb.InboundMessages()
	select {
	case msg := <-rb.InboundMessages():
		t.Errorf("expected inbound duplicate to be dropped, got message type=%s uuid=%s", msg.Type, msg.UUID)
	default:
		// OK.
	}
}
