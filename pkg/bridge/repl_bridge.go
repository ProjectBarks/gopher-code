// Package bridge — ReplBridge transport layer for bidirectional REPL↔bridge messaging.
// Source: src/bridge/replBridge.ts (initBridgeCore, ReplBridgeHandle, BridgeCoreHandle)
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// BridgeState — lifecycle state for the REPL bridge connection
// Source: BridgeState in src/bridge/replBridge.ts
// ---------------------------------------------------------------------------

// BridgeState represents the REPL bridge connection lifecycle state.
type BridgeState string

const (
	BridgeStateReady        BridgeState = "ready"
	BridgeStateConnected    BridgeState = "connected"
	BridgeStateReconnecting BridgeState = "reconnecting"
	BridgeStateFailed       BridgeState = "failed"
)

// ---------------------------------------------------------------------------
// Poll error recovery constants
// Source: replBridge.ts lines 244–246
// ---------------------------------------------------------------------------

const (
	// PollErrorInitialDelayMS is the initial backoff delay when poll starts failing.
	PollErrorInitialDelayMS = 2_000

	// PollErrorMaxDelayMS caps exponential backoff for poll errors.
	PollErrorMaxDelayMS = 60_000

	// PollErrorGiveUpMS is the total timeout before giving up on poll recovery (15 min).
	PollErrorGiveUpMS = 15 * 60 * 1000
)

// ---------------------------------------------------------------------------
// SDKMessage — minimal wire type for bridge SDK messages
// ---------------------------------------------------------------------------

// SDKMessage is a bridge SDK message (user prompt, assistant response, tool use,
// control request/response, result). The Type field discriminates the payload.
type SDKMessage struct {
	Type      string          `json:"type"`
	UUID      string          `json:"uuid,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// ---------------------------------------------------------------------------
// Callback types for lifecycle hooks
// ---------------------------------------------------------------------------

// StateChangeFunc is called when the bridge connection state changes.
type StateChangeFunc func(state BridgeState, detail string)

// InboundMessageFunc is called for each inbound SDK message from the bridge.
type InboundMessageFunc func(msg SDKMessage)

// PermissionResponseFunc is called when a permission control_response arrives.
type PermissionResponseFunc func(response SDKMessage)

// InterruptFunc is called when an interrupt control_request arrives.
type InterruptFunc func()

// SetModelFunc is called when a set_model control_request arrives.
type SetModelFunc func(model string)

// UserMessageFunc is called on each outbound user message for title derivation.
// Returns true when the callback is done (no further calls needed).
type UserMessageFunc func(text string, sessionID string) bool

// ---------------------------------------------------------------------------
// ReplBridgeConfig — all injected dependencies for the REPL bridge core
// Source: BridgeCoreParams in src/bridge/replBridge.ts
// ---------------------------------------------------------------------------

// ReplBridgeConfig holds all configuration and callbacks for a ReplBridge.
type ReplBridgeConfig struct {
	// SessionID is the bridge session ID.
	SessionID string

	// EnvironmentID is the registered environment ID.
	EnvironmentID string

	// SessionIngressURL is the ingress URL for the session.
	SessionIngressURL string

	// InitialSSESequenceNum seeds the SSE sequence-number high-water mark
	// for transport swap carryover. REPL callers pass 0; daemon callers
	// pass the value persisted at prior shutdown.
	InitialSSESequenceNum int

	// OnStateChange is called on bridge state transitions.
	OnStateChange StateChangeFunc

	// OnInboundMessage is called for each inbound SDK message.
	OnInboundMessage InboundMessageFunc

	// OnPermissionResponse is called for permission control_responses.
	OnPermissionResponse PermissionResponseFunc

	// OnInterrupt is called when an interrupt arrives from the bridge.
	OnInterrupt InterruptFunc

	// OnSetModel is called when the bridge sends a set_model request.
	OnSetModel SetModelFunc

	// OnUserMessage is called on each outbound user message for title derivation.
	OnUserMessage UserMessageFunc

	// OnDebug receives debug log messages.
	OnDebug func(msg string)
}

func (c *ReplBridgeConfig) debug(msg string) {
	if c.OnDebug != nil {
		c.OnDebug(msg)
	}
}

// ---------------------------------------------------------------------------
// ReplBridge — bidirectional transport between REPL and bridge
// ---------------------------------------------------------------------------

// ReplBridge manages the bidirectional message channel between a REPL session
// and the CCR bridge. It routes inbound messages from the bridge to the REPL
// input queue, forwards outbound events from the REPL to the bridge, manages
// session lifecycle hooks, and handles graceful teardown with flush.
//
// Thread-safe: all methods may be called from any goroutine.
type ReplBridge struct {
	cfg ReplBridgeConfig

	mu    sync.Mutex
	state BridgeState

	// sseSeqNum is the SSE sequence-number high-water mark, carried across
	// transport swaps. Atomic for lock-free reads from GetSSESequenceNum.
	sseSeqNum atomic.Int64

	// inbound is the queue of messages arriving from the bridge, destined
	// for the REPL. Buffered to decouple bridge receive rate from REPL
	// processing rate.
	inbound chan SDKMessage

	// outbound is the queue of events from the REPL destined for the bridge.
	// A drain goroutine reads from this channel and writes to the transport.
	outbound chan SDKMessage

	// recentPostedUUIDs deduplicates echoed messages.
	recentPostedUUIDs *BoundedUUIDSet

	// recentInboundUUIDs deduplicates re-delivered inbound prompts.
	recentInboundUUIDs *BoundedUUIDSet

	// flushGate gates outbound writes during initial history flush.
	// When active, new outbound messages are queued and drained once
	// the flush completes. Protected by mu.
	flushGate FlushGate[SDKMessage]

	// userMessageDone latches true when OnUserMessage returns true.
	userMessageDone atomic.Bool

	// teardownOnce ensures teardown runs exactly once.
	teardownOnce sync.Once

	// done is closed when teardown completes.
	done chan struct{}

	// cancel cancels the bridge context, stopping all background goroutines.
	cancel context.CancelFunc

	// ctx is the bridge-scoped context.
	ctx context.Context
}

const (
	// inboundBufferSize is the capacity of the inbound message channel.
	inboundBufferSize = 256

	// outboundBufferSize is the capacity of the outbound event channel.
	outboundBufferSize = 256

	// echoDeduplicateCapacity is the ring buffer size for echo dedup.
	echoDeduplicateCapacity = 2000
)

// NewReplBridge creates a new ReplBridge. Call Start to begin processing.
func NewReplBridge(cfg ReplBridgeConfig) *ReplBridge {
	ctx, cancel := context.WithCancel(context.Background())

	rb := &ReplBridge{
		cfg:                cfg,
		state:              BridgeStateReady,
		inbound:            make(chan SDKMessage, inboundBufferSize),
		outbound:           make(chan SDKMessage, outboundBufferSize),
		recentPostedUUIDs:  NewBoundedUUIDSet(echoDeduplicateCapacity),
		recentInboundUUIDs: NewBoundedUUIDSet(echoDeduplicateCapacity),
		done:               make(chan struct{}),
		cancel:             cancel,
		ctx:                ctx,
	}

	rb.sseSeqNum.Store(int64(cfg.InitialSSESequenceNum))

	// If no OnUserMessage, mark done immediately (daemon path).
	if cfg.OnUserMessage == nil {
		rb.userMessageDone.Store(true)
	}

	return rb
}

// ---------------------------------------------------------------------------
// State management
// ---------------------------------------------------------------------------

// State returns the current bridge state (thread-safe).
func (rb *ReplBridge) State() BridgeState {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.state
}

// transitionState moves to a new state and fires the OnStateChange callback.
func (rb *ReplBridge) transitionState(next BridgeState, detail string) {
	rb.mu.Lock()
	prev := rb.state
	rb.state = next
	rb.mu.Unlock()

	rb.cfg.debug(fmt.Sprintf("[bridge:repl] State %s → %s (detail=%q)", prev, next, detail))

	if rb.cfg.OnStateChange != nil {
		rb.cfg.OnStateChange(next, detail)
	}
}

// ---------------------------------------------------------------------------
// Session lifecycle hooks
// ---------------------------------------------------------------------------

// OnSessionStart signals that a bridge session has started. Transitions state
// to connected and notifies the state change callback.
func (rb *ReplBridge) OnSessionStart() {
	rb.transitionState(BridgeStateConnected, "")
	rb.cfg.debug("[bridge:repl] Session started")
}

// OnSessionEnd signals that the bridge session has ended normally. Initiates
// graceful teardown with flush.
func (rb *ReplBridge) OnSessionEnd() {
	rb.cfg.debug("[bridge:repl] Session ended")
	rb.Teardown()
}

// OnPermissionRequest handles an inbound permission request from the bridge.
// It invokes the OnPermissionResponse callback if registered, and enqueues
// the message for REPL consumption.
func (rb *ReplBridge) OnPermissionRequest(msg SDKMessage) {
	rb.cfg.debug(fmt.Sprintf("[bridge:repl] Permission request type=%s", msg.Type))

	if rb.cfg.OnPermissionResponse != nil {
		rb.cfg.OnPermissionResponse(msg)
	}

	// Also enqueue for REPL processing.
	rb.enqueueInbound(msg)
}

// ---------------------------------------------------------------------------
// Inbound: bridge → REPL
// ---------------------------------------------------------------------------

// enqueueInbound delivers a message from the bridge to the REPL input queue.
// Deduplicates against recently-seen inbound UUIDs.
func (rb *ReplBridge) enqueueInbound(msg SDKMessage) {
	// Dedup: skip if we've already forwarded this UUID.
	if msg.UUID != "" && rb.recentInboundUUIDs.Has(msg.UUID) {
		rb.cfg.debug(fmt.Sprintf("[bridge:repl] Skipping duplicate inbound uuid=%s", msg.UUID))
		return
	}
	if msg.UUID != "" {
		rb.recentInboundUUIDs.Add(msg.UUID)
	}

	// Dispatch to registered callback.
	if rb.cfg.OnInboundMessage != nil {
		rb.cfg.OnInboundMessage(msg)
	}

	// Non-blocking enqueue — drop if the REPL isn't keeping up.
	select {
	case rb.inbound <- msg:
	default:
		rb.cfg.debug("[bridge:repl] Inbound queue full, dropping message")
	}
}

// HandleInboundMessage processes an inbound SDK message from the bridge transport.
// Routes control messages to the appropriate lifecycle hook; data messages to
// the REPL input queue.
func (rb *ReplBridge) HandleInboundMessage(msg SDKMessage) {
	switch msg.Type {
	case "control_request":
		rb.handleControlRequest(msg)
	case "control_response":
		rb.OnPermissionRequest(msg)
	default:
		rb.enqueueInbound(msg)
	}
}

// handleControlRequest dispatches control requests to the appropriate callback.
func (rb *ReplBridge) handleControlRequest(msg SDKMessage) {
	// Parse subtype from payload to route to the right callback.
	var parsed struct {
		Request struct {
			Subtype string `json:"subtype"`
		} `json:"request"`
	}
	if msg.Payload != nil {
		_ = json.Unmarshal(msg.Payload, &parsed)
	}

	switch parsed.Request.Subtype {
	case "interrupt":
		rb.cfg.debug("[bridge:repl] Interrupt control request received")
		if rb.cfg.OnInterrupt != nil {
			rb.cfg.OnInterrupt()
		}
	case "set_model":
		var modelReq struct {
			Request struct {
				Model string `json:"model"`
			} `json:"request"`
		}
		if msg.Payload != nil {
			_ = json.Unmarshal(msg.Payload, &modelReq)
		}
		rb.cfg.debug(fmt.Sprintf("[bridge:repl] set_model control request model=%s", modelReq.Request.Model))
		if rb.cfg.OnSetModel != nil {
			rb.cfg.OnSetModel(modelReq.Request.Model)
		}
	default:
		// Forward unrecognized control requests to the REPL.
		rb.enqueueInbound(msg)
	}
}

// InboundMessages returns the channel from which the REPL reads inbound messages.
func (rb *ReplBridge) InboundMessages() <-chan SDKMessage {
	return rb.inbound
}

// ---------------------------------------------------------------------------
// Outbound: REPL → bridge
// ---------------------------------------------------------------------------

// WriteMessage enqueues an outbound SDK message for the bridge. The message is
// deduplicated against recently-posted UUIDs and, if OnUserMessage is registered
// and not yet done, the callback is invoked for title derivation.
func (rb *ReplBridge) WriteMessage(msg SDKMessage) {
	// Echo dedup: skip if recently posted.
	if msg.UUID != "" && rb.recentPostedUUIDs.Has(msg.UUID) {
		rb.cfg.debug(fmt.Sprintf("[bridge:repl] Skipping duplicate outbound uuid=%s", msg.UUID))
		return
	}
	if msg.UUID != "" {
		rb.recentPostedUUIDs.Add(msg.UUID)
	}

	// Title derivation: fire OnUserMessage for user-type messages until done.
	if msg.Type == "user" && !rb.userMessageDone.Load() {
		// Extract text from payload for title derivation.
		var textMsg struct {
			Content string `json:"content"`
		}
		if msg.Payload != nil {
			_ = json.Unmarshal(msg.Payload, &textMsg)
		}
		if textMsg.Content != "" && rb.cfg.OnUserMessage != nil {
			done := rb.cfg.OnUserMessage(textMsg.Content, rb.cfg.SessionID)
			if done {
				rb.userMessageDone.Store(true)
			}
		}
	}

	// Stamp session ID.
	msg.SessionID = rb.cfg.SessionID

	// If the flush gate is active, queue the message for later drain.
	rb.mu.Lock()
	if rb.flushGate.Enqueue(msg) {
		rb.mu.Unlock()
		rb.cfg.debug("[bridge:repl] Flush gate active, queued outbound message")
		return
	}
	rb.mu.Unlock()

	// Non-blocking enqueue.
	select {
	case rb.outbound <- msg:
	default:
		rb.cfg.debug("[bridge:repl] Outbound queue full, dropping message")
	}
}

// WriteMessages enqueues multiple outbound SDK messages.
func (rb *ReplBridge) WriteMessages(msgs []SDKMessage) {
	for i := range msgs {
		rb.WriteMessage(msgs[i])
	}
}

// SendControlRequest enqueues an outbound control_request for the bridge.
func (rb *ReplBridge) SendControlRequest(request SDKMessage) {
	request.Type = "control_request"
	request.SessionID = rb.cfg.SessionID
	rb.cfg.debug(fmt.Sprintf("[bridge:repl] Sending control_request"))

	select {
	case rb.outbound <- request:
	default:
		rb.cfg.debug("[bridge:repl] Outbound queue full, dropping control_request")
	}
}

// SendControlResponse enqueues an outbound control_response for the bridge.
func (rb *ReplBridge) SendControlResponse(response SDKMessage) {
	response.Type = "control_response"
	response.SessionID = rb.cfg.SessionID
	rb.cfg.debug(fmt.Sprintf("[bridge:repl] Sending control_response"))

	select {
	case rb.outbound <- response:
	default:
		rb.cfg.debug("[bridge:repl] Outbound queue full, dropping control_response")
	}
}

// SendResult enqueues a result message signalling the session turn is complete.
func (rb *ReplBridge) SendResult() {
	rb.cfg.debug(fmt.Sprintf("[bridge:repl] Sending result for session=%s", rb.cfg.SessionID))

	msg := SDKMessage{
		Type:      "result",
		SessionID: rb.cfg.SessionID,
	}
	select {
	case rb.outbound <- msg:
	default:
		rb.cfg.debug("[bridge:repl] Outbound queue full, dropping result")
	}
}

// OutboundMessages returns the channel from which the transport reads outbound events.
func (rb *ReplBridge) OutboundMessages() <-chan SDKMessage {
	return rb.outbound
}

// ---------------------------------------------------------------------------
// SSE sequence number
// ---------------------------------------------------------------------------

// GetSSESequenceNum returns the current SSE high-water mark.
func (rb *ReplBridge) GetSSESequenceNum() int {
	return int(rb.sseSeqNum.Load())
}

// SetSSESequenceNum updates the SSE high-water mark (called on transport swap).
func (rb *ReplBridge) SetSSESequenceNum(n int) {
	rb.sseSeqNum.Store(int64(n))
}

// ---------------------------------------------------------------------------
// Flush gate — gates outbound writes during initial history flush
// Source: src/bridge/replBridge.ts — flushGate lifecycle
// ---------------------------------------------------------------------------

// StartFlush activates the flush gate. While active, outbound messages are
// queued instead of being written to the outbound channel. Call EndFlush to
// drain the queued messages after the history flush completes.
func (rb *ReplBridge) StartFlush() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.flushGate.Start()
	rb.cfg.debug("[bridge:repl] Flush gate started")
}

// EndFlush ends the flush gate and drains all queued messages into the
// outbound channel. Returns the number of messages drained.
func (rb *ReplBridge) EndFlush() int {
	rb.mu.Lock()
	queued := rb.flushGate.End()
	rb.mu.Unlock()

	for _, msg := range queued {
		select {
		case rb.outbound <- msg:
		default:
			rb.cfg.debug("[bridge:repl] Outbound queue full during flush drain, dropping message")
		}
	}

	rb.cfg.debug(fmt.Sprintf("[bridge:repl] Flush gate ended, drained %d messages", len(queued)))
	return len(queued)
}

// DropFlush discards all queued flush messages (e.g. on transport close).
// Returns the number of messages dropped.
func (rb *ReplBridge) DropFlush() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	count := rb.flushGate.Drop()
	if count > 0 {
		rb.cfg.debug(fmt.Sprintf("[bridge:repl] Flush gate dropped %d messages", count))
	}
	return count
}

// IsFlushActive reports whether the flush gate is currently active.
func (rb *ReplBridge) IsFlushActive() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.flushGate.Active()
}

// ---------------------------------------------------------------------------
// Teardown
// ---------------------------------------------------------------------------

// Teardown initiates graceful shutdown: drains the outbound queue, closes
// channels, and cancels the bridge context. Safe to call multiple times;
// only the first call has effect. Blocks until teardown is complete.
func (rb *ReplBridge) Teardown() {
	rb.teardownOnce.Do(func() {
		rb.cfg.debug("[bridge:repl] Teardown starting")

		// Drop any flush-gated messages — the transport is going away.
		rb.mu.Lock()
		dropped := rb.flushGate.Drop()
		rb.mu.Unlock()
		if dropped > 0 {
			rb.cfg.debug(fmt.Sprintf("[bridge:repl] Teardown dropped %d flush-gated messages", dropped))
		}

		// Cancel the bridge context to stop background goroutines.
		rb.cancel()

		// Drain remaining outbound messages with a deadline.
		rb.drainOutbound(2 * time.Second)

		// Transition state.
		rb.transitionState(BridgeStateFailed, "teardown")

		// Close channels.
		close(rb.outbound)
		close(rb.inbound)

		rb.cfg.debug("[bridge:repl] Teardown complete")
		close(rb.done)
	})
}

// drainOutbound reads and discards remaining outbound messages within the timeout.
func (rb *ReplBridge) drainOutbound(timeout time.Duration) {
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-rb.outbound:
			if !ok {
				return
			}
		case <-deadline:
			rb.cfg.debug("[bridge:repl] Drain timeout reached, proceeding with teardown")
			return
		default:
			// Queue empty.
			return
		}
	}
}

// Done returns a channel that is closed when teardown completes.
func (rb *ReplBridge) Done() <-chan struct{} {
	return rb.done
}

// SessionID returns the bridge session ID.
func (rb *ReplBridge) SessionID() string {
	return rb.cfg.SessionID
}

// EnvironmentID returns the registered environment ID.
func (rb *ReplBridge) EnvironmentID() string {
	return rb.cfg.EnvironmentID
}

// SessionIngressURL returns the session ingress URL.
func (rb *ReplBridge) SessionIngressURL() string {
	return rb.cfg.SessionIngressURL
}
