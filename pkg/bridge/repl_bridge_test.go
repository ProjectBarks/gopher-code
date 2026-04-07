package bridge

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Message routing: inbound (bridge → REPL)
// ---------------------------------------------------------------------------

func TestReplBridge_InboundRouting(t *testing.T) {
	var callbackMsgs []SDKMessage
	var mu sync.Mutex

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID:     "sess-1",
		EnvironmentID: "env-1",
		OnInboundMessage: func(msg SDKMessage) {
			mu.Lock()
			callbackMsgs = append(callbackMsgs, msg)
			mu.Unlock()
		},
	})

	// Send 3 inbound messages.
	for i := 0; i < 3; i++ {
		rb.HandleInboundMessage(SDKMessage{
			Type: "user",
			UUID: "uuid-" + string(rune('a'+i)),
		})
	}

	// All 3 should be available on the inbound channel.
	for i := 0; i < 3; i++ {
		select {
		case msg := <-rb.InboundMessages():
			if msg.Type != "user" {
				t.Errorf("inbound[%d]: expected type=user, got %s", i, msg.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("inbound[%d]: timed out waiting for message", i)
		}
	}

	// Callback should have been called 3 times.
	mu.Lock()
	got := len(callbackMsgs)
	mu.Unlock()
	if got != 3 {
		t.Errorf("expected OnInboundMessage called 3 times, got %d", got)
	}

	rb.Teardown()
}

func TestReplBridge_InboundDedup(t *testing.T) {
	var count atomic.Int32

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-1",
		OnInboundMessage: func(msg SDKMessage) {
			count.Add(1)
		},
	})

	// Send the same UUID twice.
	rb.HandleInboundMessage(SDKMessage{Type: "user", UUID: "dup-uuid"})
	rb.HandleInboundMessage(SDKMessage{Type: "user", UUID: "dup-uuid"})

	// Only one should have been delivered.
	if got := count.Load(); got != 1 {
		t.Errorf("expected 1 delivery after dedup, got %d", got)
	}

	rb.Teardown()
}

// ---------------------------------------------------------------------------
// Message routing: outbound (REPL → bridge)
// ---------------------------------------------------------------------------

func TestReplBridge_OutboundRouting(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{
		SessionID:     "sess-out",
		EnvironmentID: "env-out",
	})

	// Write 3 messages.
	for i := 0; i < 3; i++ {
		rb.WriteMessage(SDKMessage{
			Type: "assistant",
			UUID: "out-" + string(rune('a'+i)),
		})
	}

	// All 3 should appear on the outbound channel with session ID stamped.
	for i := 0; i < 3; i++ {
		select {
		case msg := <-rb.OutboundMessages():
			if msg.Type != "assistant" {
				t.Errorf("outbound[%d]: expected type=assistant, got %s", i, msg.Type)
			}
			if msg.SessionID != "sess-out" {
				t.Errorf("outbound[%d]: expected session_id=sess-out, got %s", i, msg.SessionID)
			}
		case <-time.After(time.Second):
			t.Fatalf("outbound[%d]: timed out waiting for message", i)
		}
	}

	rb.Teardown()
}

func TestReplBridge_OutboundDedup(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-dedup",
	})

	rb.WriteMessage(SDKMessage{Type: "assistant", UUID: "dup-out"})
	rb.WriteMessage(SDKMessage{Type: "assistant", UUID: "dup-out"})

	// Only one should be enqueued.
	select {
	case <-rb.OutboundMessages():
	case <-time.After(time.Second):
		t.Fatal("expected at least one outbound message")
	}

	// Second should not arrive.
	select {
	case msg := <-rb.OutboundMessages():
		t.Errorf("expected no second message, got type=%s uuid=%s", msg.Type, msg.UUID)
	case <-time.After(50 * time.Millisecond):
		// Good — no duplicate.
	}

	rb.Teardown()
}

func TestReplBridge_WriteMessages_Batch(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-batch"})

	rb.WriteMessages([]SDKMessage{
		{Type: "user", UUID: "b1"},
		{Type: "assistant", UUID: "b2"},
		{Type: "result", UUID: "b3"},
	})

	for i := 0; i < 3; i++ {
		select {
		case <-rb.OutboundMessages():
		case <-time.After(time.Second):
			t.Fatalf("outbound batch[%d]: timed out", i)
		}
	}

	rb.Teardown()
}

// ---------------------------------------------------------------------------
// Session lifecycle callbacks
// ---------------------------------------------------------------------------

func TestReplBridge_SessionLifecycle(t *testing.T) {
	var states []BridgeState
	var mu sync.Mutex

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-lc",
		OnStateChange: func(state BridgeState, detail string) {
			mu.Lock()
			states = append(states, state)
			mu.Unlock()
		},
	})

	// Initial state should be ready.
	if got := rb.State(); got != BridgeStateReady {
		t.Errorf("initial state: expected %s, got %s", BridgeStateReady, got)
	}

	// OnSessionStart → connected.
	rb.OnSessionStart()
	if got := rb.State(); got != BridgeStateConnected {
		t.Errorf("after OnSessionStart: expected %s, got %s", BridgeStateConnected, got)
	}

	// OnSessionEnd → triggers teardown (state goes to failed/teardown).
	rb.OnSessionEnd()
	<-rb.Done()

	mu.Lock()
	defer mu.Unlock()
	if len(states) < 2 {
		t.Fatalf("expected at least 2 state changes, got %d: %v", len(states), states)
	}
	if states[0] != BridgeStateConnected {
		t.Errorf("first state change: expected %s, got %s", BridgeStateConnected, states[0])
	}
}

func TestReplBridge_OnPermissionRequest(t *testing.T) {
	var permResp SDKMessage
	var got bool

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-perm",
		OnPermissionResponse: func(response SDKMessage) {
			permResp = response
			got = true
		},
	})

	msg := SDKMessage{Type: "control_response", UUID: "perm-1"}
	rb.OnPermissionRequest(msg)

	if !got {
		t.Fatal("OnPermissionResponse not called")
	}
	if permResp.UUID != "perm-1" {
		t.Errorf("expected uuid=perm-1, got %s", permResp.UUID)
	}

	// Should also be on the inbound channel.
	select {
	case inbound := <-rb.InboundMessages():
		if inbound.UUID != "perm-1" {
			t.Errorf("inbound uuid: expected perm-1, got %s", inbound.UUID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for inbound permission message")
	}

	rb.Teardown()
}

// ---------------------------------------------------------------------------
// Control request routing
// ---------------------------------------------------------------------------

func TestReplBridge_ControlRequest_Interrupt(t *testing.T) {
	var interrupted atomic.Bool

	payload, _ := json.Marshal(map[string]any{
		"request": map[string]any{
			"subtype": "interrupt",
		},
	})

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-int",
		OnInterrupt: func() {
			interrupted.Store(true)
		},
	})

	rb.HandleInboundMessage(SDKMessage{
		Type:    "control_request",
		UUID:    "ctrl-1",
		Payload: payload,
	})

	if !interrupted.Load() {
		t.Error("expected OnInterrupt to be called")
	}

	rb.Teardown()
}

func TestReplBridge_ControlRequest_SetModel(t *testing.T) {
	var gotModel string
	var mu sync.Mutex

	payload, _ := json.Marshal(map[string]any{
		"request": map[string]any{
			"subtype": "set_model",
			"model":   "claude-3-opus",
		},
	})

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-model",
		OnSetModel: func(model string) {
			mu.Lock()
			gotModel = model
			mu.Unlock()
		},
	})

	rb.HandleInboundMessage(SDKMessage{
		Type:    "control_request",
		UUID:    "ctrl-model",
		Payload: payload,
	})

	mu.Lock()
	if gotModel != "claude-3-opus" {
		t.Errorf("expected model=claude-3-opus, got %s", gotModel)
	}
	mu.Unlock()

	rb.Teardown()
}

// ---------------------------------------------------------------------------
// Event forwarding: outbound control + result
// ---------------------------------------------------------------------------

func TestReplBridge_SendControlRequest(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-ctrl"})

	rb.SendControlRequest(SDKMessage{UUID: "ctrl-req-1"})

	select {
	case msg := <-rb.OutboundMessages():
		if msg.Type != "control_request" {
			t.Errorf("expected type=control_request, got %s", msg.Type)
		}
		if msg.SessionID != "sess-ctrl" {
			t.Errorf("expected session_id=sess-ctrl, got %s", msg.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for outbound control_request")
	}

	rb.Teardown()
}

func TestReplBridge_SendControlResponse(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-cr"})

	rb.SendControlResponse(SDKMessage{UUID: "resp-1"})

	select {
	case msg := <-rb.OutboundMessages():
		if msg.Type != "control_response" {
			t.Errorf("expected type=control_response, got %s", msg.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for outbound control_response")
	}

	rb.Teardown()
}

func TestReplBridge_SendResult(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-res"})

	rb.SendResult()

	select {
	case msg := <-rb.OutboundMessages():
		if msg.Type != "result" {
			t.Errorf("expected type=result, got %s", msg.Type)
		}
		if msg.SessionID != "sess-res" {
			t.Errorf("expected session_id=sess-res, got %s", msg.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for outbound result")
	}

	rb.Teardown()
}

// ---------------------------------------------------------------------------
// SSE sequence number
// ---------------------------------------------------------------------------

func TestReplBridge_SSESequenceNum(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{
		SessionID:             "sess-sse",
		InitialSSESequenceNum: 42,
	})

	if got := rb.GetSSESequenceNum(); got != 42 {
		t.Errorf("expected initial SSE seq=42, got %d", got)
	}

	rb.SetSSESequenceNum(100)
	if got := rb.GetSSESequenceNum(); got != 100 {
		t.Errorf("expected SSE seq=100 after set, got %d", got)
	}

	rb.Teardown()
}

// ---------------------------------------------------------------------------
// Graceful teardown
// ---------------------------------------------------------------------------

func TestReplBridge_TeardownIdempotent(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-td"})

	// Teardown should be safe to call multiple times.
	rb.Teardown()
	rb.Teardown()
	rb.Teardown()

	select {
	case <-rb.Done():
		// Good — done is closed.
	case <-time.After(time.Second):
		t.Fatal("done channel not closed after teardown")
	}
}

func TestReplBridge_TeardownClosesDone(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-done"})

	// Done should not be closed yet.
	select {
	case <-rb.Done():
		t.Fatal("done closed before teardown")
	default:
	}

	rb.Teardown()

	select {
	case <-rb.Done():
		// Good.
	case <-time.After(time.Second):
		t.Fatal("done not closed after teardown")
	}
}

// ---------------------------------------------------------------------------
// UserMessage title derivation
// ---------------------------------------------------------------------------

func TestReplBridge_UserMessageCallback(t *testing.T) {
	var calls int
	var mu sync.Mutex

	rb := NewReplBridge(ReplBridgeConfig{
		SessionID: "sess-um",
		OnUserMessage: func(text string, sessionID string) bool {
			mu.Lock()
			calls++
			done := calls >= 2
			mu.Unlock()
			return done
		},
	})

	payload, _ := json.Marshal(map[string]any{"content": "hello"})

	// First user message — callback returns false (not done).
	rb.WriteMessage(SDKMessage{Type: "user", UUID: "u1", Payload: payload})

	mu.Lock()
	if calls != 1 {
		t.Errorf("expected 1 call after first message, got %d", calls)
	}
	mu.Unlock()

	// Second user message — callback returns true (done).
	rb.WriteMessage(SDKMessage{Type: "user", UUID: "u2", Payload: payload})

	mu.Lock()
	if calls != 2 {
		t.Errorf("expected 2 calls after second message, got %d", calls)
	}
	mu.Unlock()

	// Third user message — callback should NOT be called (done latch).
	rb.WriteMessage(SDKMessage{Type: "user", UUID: "u3", Payload: payload})

	mu.Lock()
	if calls != 2 {
		t.Errorf("expected still 2 calls after third message (done), got %d", calls)
	}
	mu.Unlock()

	rb.Teardown()
}

func TestReplBridge_NoUserMessageCallback_DaemonPath(t *testing.T) {
	// When OnUserMessage is nil (daemon path), userMessageDone should be true
	// from the start.
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-daemon"})

	if !rb.userMessageDone.Load() {
		t.Error("expected userMessageDone=true when OnUserMessage is nil")
	}

	rb.Teardown()
}

// ---------------------------------------------------------------------------
// Accessor methods
// ---------------------------------------------------------------------------

func TestReplBridge_Accessors(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{
		SessionID:         "s-123",
		EnvironmentID:     "e-456",
		SessionIngressURL: "https://ingress.example.com",
	})

	if got := rb.SessionID(); got != "s-123" {
		t.Errorf("SessionID: expected s-123, got %s", got)
	}
	if got := rb.EnvironmentID(); got != "e-456" {
		t.Errorf("EnvironmentID: expected e-456, got %s", got)
	}
	if got := rb.SessionIngressURL(); got != "https://ingress.example.com" {
		t.Errorf("SessionIngressURL: expected https://ingress.example.com, got %s", got)
	}

	rb.Teardown()
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestReplBridge_PollConstants(t *testing.T) {
	if PollErrorInitialDelayMS != 2000 {
		t.Errorf("PollErrorInitialDelayMS: expected 2000, got %d", PollErrorInitialDelayMS)
	}
	if PollErrorMaxDelayMS != 60000 {
		t.Errorf("PollErrorMaxDelayMS: expected 60000, got %d", PollErrorMaxDelayMS)
	}
	if PollErrorGiveUpMS != 900000 {
		t.Errorf("PollErrorGiveUpMS: expected 900000, got %d", PollErrorGiveUpMS)
	}
}

// ---------------------------------------------------------------------------
// Flush gate integration
// ---------------------------------------------------------------------------

func TestReplBridge_FlushGate_QueuesDuringFlush(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-fg"})

	// Initially not active.
	if rb.IsFlushActive() {
		t.Fatal("flush gate should be inactive on new bridge")
	}

	// Start flush — messages should be queued, not sent to outbound channel.
	rb.StartFlush()
	if !rb.IsFlushActive() {
		t.Fatal("flush gate should be active after StartFlush")
	}

	// Write 3 messages while flush is active.
	for i := 0; i < 3; i++ {
		rb.WriteMessage(SDKMessage{
			Type: "assistant",
			UUID: "fg-" + string(rune('a'+i)),
		})
	}

	// Outbound channel should be empty — messages are gated.
	select {
	case <-rb.OutboundMessages():
		t.Fatal("expected no outbound messages while flush gate is active")
	default:
		// Good.
	}

	// End flush — queued messages should drain to outbound channel.
	drained := rb.EndFlush()
	if drained != 3 {
		t.Errorf("expected 3 drained messages, got %d", drained)
	}
	if rb.IsFlushActive() {
		t.Fatal("flush gate should be inactive after EndFlush")
	}

	// All 3 messages should now be on the outbound channel.
	for i := 0; i < 3; i++ {
		select {
		case msg := <-rb.OutboundMessages():
			if msg.Type != "assistant" {
				t.Errorf("drained[%d]: expected type=assistant, got %s", i, msg.Type)
			}
			if msg.SessionID != "sess-fg" {
				t.Errorf("drained[%d]: expected session_id=sess-fg, got %s", i, msg.SessionID)
			}
		case <-time.After(time.Second):
			t.Fatalf("drained[%d]: timed out waiting for message", i)
		}
	}

	rb.Teardown()
}

func TestReplBridge_FlushGate_PassthroughWhenInactive(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-fg2"})

	// Write without starting flush — should go directly to outbound.
	rb.WriteMessage(SDKMessage{Type: "user", UUID: "direct-1"})

	select {
	case msg := <-rb.OutboundMessages():
		if msg.UUID != "direct-1" {
			t.Errorf("expected uuid=direct-1, got %s", msg.UUID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for direct outbound message")
	}

	rb.Teardown()
}

func TestReplBridge_FlushGate_DropOnTeardown(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-fg3"})

	rb.StartFlush()
	rb.WriteMessage(SDKMessage{Type: "user", UUID: "drop-1"})
	rb.WriteMessage(SDKMessage{Type: "user", UUID: "drop-2"})

	// Teardown should drop flush-gated messages.
	rb.Teardown()

	if rb.IsFlushActive() {
		t.Error("flush gate should be inactive after teardown")
	}
}

func TestReplBridge_FlushGate_DropFlush(t *testing.T) {
	rb := NewReplBridge(ReplBridgeConfig{SessionID: "sess-fg4"})

	rb.StartFlush()
	rb.WriteMessage(SDKMessage{Type: "user", UUID: "d-1"})
	rb.WriteMessage(SDKMessage{Type: "user", UUID: "d-2"})

	dropped := rb.DropFlush()
	if dropped != 2 {
		t.Errorf("expected 2 dropped, got %d", dropped)
	}
	if rb.IsFlushActive() {
		t.Error("flush gate should be inactive after DropFlush")
	}

	// Outbound should be empty.
	select {
	case <-rb.OutboundMessages():
		t.Fatal("expected no outbound messages after DropFlush")
	default:
	}

	rb.Teardown()
}
