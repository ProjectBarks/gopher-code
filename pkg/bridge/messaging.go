// Package bridge — BridgeMessaging: outbound event buffering, flush, and ack tracking.
// Source: src/bridge/bridgeMessaging.ts + src/cli/transports/SerialBatchEventUploader.ts
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
// Constants — user-visible strings (verbatim match with TS source)
// ---------------------------------------------------------------------------

// OutboundOnlyError is the error sent when a mutable control request arrives
// on an outbound-only bridge session.
const OutboundOnlyError = "This session is outbound-only. Enable Remote Control locally to allow inbound control."

// PermissionModeNotSupportedError is returned when no onSetPermissionMode
// callback is registered.
const PermissionModeNotSupportedError = "set_permission_mode is not supported in this context (onSetPermissionMode callback not registered)"

// ---------------------------------------------------------------------------
// BridgeEvent — the unit of work enqueued into BridgeMessaging
// ---------------------------------------------------------------------------

// BridgeEvent is any JSON-serializable event destined for the bridge API.
type BridgeEvent struct {
	// Type is the SDK message type discriminant (e.g. "user", "assistant",
	// "result", "control_response").
	Type string `json:"type"`

	// SessionID is stamped on every outbound event.
	SessionID string `json:"session_id,omitempty"`

	// SeqNum is the monotonic sequence number assigned by BridgeMessaging.
	// The bridge API uses this for ordering and dedup.
	SeqNum uint64 `json:"seq_num"`

	// Payload carries the full event body. It is merged into the top-level
	// JSON object on serialization (the Type/SessionID/SeqNum fields above
	// are convenience accessors — Payload is the authority).
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ---------------------------------------------------------------------------
// SendFunc — caller-supplied HTTP POST callback
// ---------------------------------------------------------------------------

// SendFunc posts a batch of serialized events to the bridge API.
// It must return nil on success. Any non-nil error triggers retry with
// backoff. The batch is guaranteed non-empty.
type SendFunc func(ctx context.Context, batch []BridgeEvent) error

// ---------------------------------------------------------------------------
// Ack tracking
// ---------------------------------------------------------------------------

// pendingAck tracks a single unacknowledged event.
type pendingAck struct {
	seqNum  uint64
	sentAt  time.Time
	timer   *time.Timer
	expired atomic.Bool
}

// ---------------------------------------------------------------------------
// BridgeMessagingConfig tunes buffer sizes, batching, and retry.
// ---------------------------------------------------------------------------

// BridgeMessagingConfig holds tunables for BridgeMessaging.
type BridgeMessagingConfig struct {
	// MaxBatchSize is the max events per flush POST (default 100).
	MaxBatchSize int

	// MaxQueueSize is the max pending events before Enqueue blocks (default 1000).
	MaxQueueSize int

	// AckTimeout is how long to wait for an ack before considering the
	// event lost and eligible for re-send (default 30s).
	AckTimeout time.Duration

	// BaseRetryDelay is the initial backoff delay on send failure (default 500ms).
	BaseRetryDelay time.Duration

	// MaxRetryDelay caps exponential backoff (default 30s).
	MaxRetryDelay time.Duration

	// Send is the caller-supplied function that posts a batch to the API.
	Send SendFunc
}

func (c *BridgeMessagingConfig) defaults() {
	if c.MaxBatchSize <= 0 {
		c.MaxBatchSize = 100
	}
	if c.MaxQueueSize <= 0 {
		c.MaxQueueSize = 1000
	}
	if c.AckTimeout <= 0 {
		c.AckTimeout = 30 * time.Second
	}
	if c.BaseRetryDelay <= 0 {
		c.BaseRetryDelay = 500 * time.Millisecond
	}
	if c.MaxRetryDelay <= 0 {
		c.MaxRetryDelay = 30 * time.Second
	}
}

// ---------------------------------------------------------------------------
// BridgeMessaging — outbound event queue with flush, ordering, ack tracking
// ---------------------------------------------------------------------------

// BridgeMessaging manages a serial outbound event queue. At most one flush
// is in-flight at a time. New events accumulate while a flush is running.
// Sequence numbers are monotonically assigned to guarantee ordering and
// enable at-least-once delivery (via ack tracking and re-send on timeout).
type BridgeMessaging struct {
	mu   sync.Mutex
	cond *sync.Cond // signalled when pending grows or close is called

	cfg BridgeMessagingConfig

	pending []BridgeEvent // buffered events awaiting flush
	nextSeq uint64        // next sequence number to assign

	// Ack tracking: seqNum -> pendingAck. Entries are added on send,
	// removed on Acknowledge(). If the ack timer fires, the event is
	// re-queued at the front of pending for re-delivery.
	acks map[uint64]*pendingAck

	draining atomic.Bool // true while drain goroutine is active
	closed   atomic.Bool

	// flushWaiters are notified when pending is fully drained.
	flushWaiters []chan struct{}
}

// NewBridgeMessaging creates a new BridgeMessaging with the given config.
func NewBridgeMessaging(cfg BridgeMessagingConfig) *BridgeMessaging {
	cfg.defaults()
	bm := &BridgeMessaging{
		cfg:     cfg,
		acks:    make(map[uint64]*pendingAck),
		nextSeq: 1,
	}
	bm.cond = sync.NewCond(&bm.mu)
	return bm
}

// Enqueue adds an event to the outbound buffer. If the buffer is at capacity,
// Enqueue blocks until space is available or the context is cancelled.
// A monotonic sequence number is assigned to the event.
// If the buffer reaches MaxBatchSize, a drain is triggered automatically.
func (bm *BridgeMessaging) Enqueue(ctx context.Context, evt BridgeEvent) error {
	if bm.closed.Load() {
		return fmt.Errorf("bridge messaging closed")
	}

	bm.mu.Lock()
	// Backpressure: wait for space.
	for len(bm.pending) >= bm.cfg.MaxQueueSize && !bm.closed.Load() {
		// Release lock, wait for signal, re-acquire.
		// Check context between waits.
		bm.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		bm.mu.Lock()
		if len(bm.pending) >= bm.cfg.MaxQueueSize && !bm.closed.Load() {
			bm.cond.Wait()
		}
	}

	if bm.closed.Load() {
		bm.mu.Unlock()
		return fmt.Errorf("bridge messaging closed")
	}

	// Assign sequence number.
	evt.SeqNum = bm.nextSeq
	bm.nextSeq++

	bm.pending = append(bm.pending, evt)
	needsDrain := len(bm.pending) >= bm.cfg.MaxBatchSize
	bm.mu.Unlock()

	if needsDrain {
		bm.triggerDrain()
	}

	return nil
}

// Flush blocks until all currently-pending events have been sent. It kicks
// a drain if one is not already running.
func (bm *BridgeMessaging) Flush(ctx context.Context) error {
	bm.mu.Lock()
	if len(bm.pending) == 0 && !bm.draining.Load() {
		bm.mu.Unlock()
		return nil
	}

	ch := make(chan struct{}, 1)
	bm.flushWaiters = append(bm.flushWaiters, ch)
	bm.mu.Unlock()

	bm.triggerDrain()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Acknowledge marks a sequence number as successfully received by the server.
// Returns true if the seqNum was pending, false if it was already acked or unknown.
func (bm *BridgeMessaging) Acknowledge(seqNum uint64) bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	ack, ok := bm.acks[seqNum]
	if !ok {
		return false
	}
	ack.timer.Stop()
	delete(bm.acks, seqNum)
	return true
}

// PendingCount returns the number of events waiting to be sent.
func (bm *BridgeMessaging) PendingCount() int {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return len(bm.pending)
}

// PendingAckCount returns the number of events sent but not yet acknowledged.
func (bm *BridgeMessaging) PendingAckCount() int {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return len(bm.acks)
}

// NextSeqNum returns the next sequence number that will be assigned.
func (bm *BridgeMessaging) NextSeqNum() uint64 {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.nextSeq
}

// Close stops the messaging system. Pending events are dropped. Blocked
// Enqueue and Flush callers are unblocked.
func (bm *BridgeMessaging) Close() {
	if bm.closed.Swap(true) {
		return // already closed
	}

	bm.mu.Lock()
	bm.pending = nil

	// Stop all ack timers.
	for seq, ack := range bm.acks {
		ack.timer.Stop()
		delete(bm.acks, seq)
	}

	// Unblock flush waiters.
	for _, ch := range bm.flushWaiters {
		close(ch)
	}
	bm.flushWaiters = nil

	bm.cond.Broadcast()
	bm.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Internal drain loop
// ---------------------------------------------------------------------------

// triggerDrain starts the drain goroutine if one is not already running.
func (bm *BridgeMessaging) triggerDrain() {
	if bm.draining.CompareAndSwap(false, true) {
		go bm.drain()
	}
}

// drain sends batches serially until pending is empty.
func (bm *BridgeMessaging) drain() {
	defer bm.draining.Store(false)

	for !bm.closed.Load() {
		batch := bm.takeBatch()
		if len(batch) == 0 {
			break
		}

		// Send with retry + exponential backoff.
		failures := 0
		for !bm.closed.Load() {
			err := bm.cfg.Send(context.Background(), batch)
			if err == nil {
				// Track acks for the batch.
				bm.trackAcks(batch)
				break
			}
			failures++
			delay := bm.retryDelay(failures)
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			}
			timer.Stop()
		}

		// Release backpressure.
		bm.cond.Broadcast()
	}

	// Notify flush waiters.
	bm.mu.Lock()
	if len(bm.pending) == 0 {
		for _, ch := range bm.flushWaiters {
			close(ch)
		}
		bm.flushWaiters = nil
	}
	bm.mu.Unlock()
}

// takeBatch removes up to MaxBatchSize events from the front of pending.
func (bm *BridgeMessaging) takeBatch() []BridgeEvent {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	n := len(bm.pending)
	if n == 0 {
		return nil
	}
	if n > bm.cfg.MaxBatchSize {
		n = bm.cfg.MaxBatchSize
	}
	batch := make([]BridgeEvent, n)
	copy(batch, bm.pending[:n])
	bm.pending = bm.pending[n:]
	return batch
}

// trackAcks registers sent events for acknowledgement tracking.
func (bm *BridgeMessaging) trackAcks(batch []BridgeEvent) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	for _, evt := range batch {
		pa := &pendingAck{
			seqNum: evt.SeqNum,
			sentAt: now,
		}
		pa.timer = time.AfterFunc(bm.cfg.AckTimeout, func() {
			bm.handleAckTimeout(pa)
		})
		bm.acks[evt.SeqNum] = pa
	}
}

// handleAckTimeout re-queues an unacknowledged event at the front of pending
// for at-least-once delivery.
func (bm *BridgeMessaging) handleAckTimeout(pa *pendingAck) {
	pa.expired.Store(true)

	bm.mu.Lock()
	// Only re-queue if still in the ack map (not already acked).
	if _, ok := bm.acks[pa.seqNum]; !ok {
		bm.mu.Unlock()
		return
	}
	delete(bm.acks, pa.seqNum)

	// Re-queue at front for re-delivery with same sequence number.
	requeue := BridgeEvent{SeqNum: pa.seqNum}
	bm.pending = append([]BridgeEvent{requeue}, bm.pending...)
	bm.mu.Unlock()

	bm.triggerDrain()
}

// retryDelay computes exponential backoff clamped to MaxRetryDelay.
func (bm *BridgeMessaging) retryDelay(failures int) time.Duration {
	delay := bm.cfg.BaseRetryDelay
	for i := 1; i < failures; i++ {
		delay *= 2
		if delay > bm.cfg.MaxRetryDelay {
			delay = bm.cfg.MaxRetryDelay
			break
		}
	}
	return delay
}

// ---------------------------------------------------------------------------
// BoundedUUIDSet — FIFO ring buffer for echo-dedup
// Source: BoundedUUIDSet in src/bridge/bridgeMessaging.ts
// ---------------------------------------------------------------------------

// BoundedUUIDSet is a capacity-bounded set backed by a circular buffer.
// It evicts the oldest entry when capacity is reached, keeping memory at
// O(capacity). Used for echo dedup (recentPostedUUIDs, recentInboundUUIDs).
//
// Safe for concurrent use.
type BoundedUUIDSet struct {
	mu       sync.RWMutex
	capacity int
	ring     []string
	set      map[string]struct{}
	writeIdx int
}

// NewBoundedUUIDSet creates a set with the given capacity.
func NewBoundedUUIDSet(capacity int) *BoundedUUIDSet {
	if capacity <= 0 {
		capacity = 1
	}
	return &BoundedUUIDSet{
		capacity: capacity,
		ring:     make([]string, capacity),
		set:      make(map[string]struct{}, capacity),
	}
}

// Add inserts a UUID. If the set is at capacity, the oldest entry is evicted.
// If the UUID is already present, this is a no-op.
func (s *BoundedUUIDSet) Add(uuid string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.set[uuid]; ok {
		return
	}

	// Evict the entry at writeIdx if occupied.
	if evicted := s.ring[s.writeIdx]; evicted != "" {
		delete(s.set, evicted)
	}

	s.ring[s.writeIdx] = uuid
	s.set[uuid] = struct{}{}
	s.writeIdx = (s.writeIdx + 1) % s.capacity
}

// Has returns true if the UUID is in the set.
func (s *BoundedUUIDSet) Has(uuid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.set[uuid]
	return ok
}

// Clear removes all entries and resets the write cursor.
func (s *BoundedUUIDSet) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.set = make(map[string]struct{}, s.capacity)
	for i := range s.ring {
		s.ring[i] = ""
	}
	s.writeIdx = 0
}

// Len returns the current number of entries in the set.
func (s *BoundedUUIDSet) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.set)
}
