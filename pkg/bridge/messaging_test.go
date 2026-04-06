package bridge

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// BridgeMessaging — Enqueue + Flush
// ---------------------------------------------------------------------------

func TestBridgeMessaging_EnqueueAndFlush(t *testing.T) {
	var sent [][]BridgeEvent
	var mu sync.Mutex

	bm := NewBridgeMessaging(BridgeMessagingConfig{
		MaxBatchSize: 10,
		MaxQueueSize: 100,
		AckTimeout:   5 * time.Second,
		Send: func(_ context.Context, batch []BridgeEvent) error {
			mu.Lock()
			cp := make([]BridgeEvent, len(batch))
			copy(cp, batch)
			sent = append(sent, cp)
			mu.Unlock()
			return nil
		},
	})
	defer bm.Close()

	ctx := context.Background()

	// Enqueue 3 events.
	for i := 0; i < 3; i++ {
		if err := bm.Enqueue(ctx, BridgeEvent{Type: "user"}); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	// Flush should send all pending.
	if err := bm.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	mu.Lock()
	total := 0
	for _, batch := range sent {
		total += len(batch)
	}
	mu.Unlock()

	if total != 3 {
		t.Errorf("expected 3 events sent, got %d", total)
	}

	// Sequence numbers should be 1, 2, 3.
	mu.Lock()
	var seqs []uint64
	for _, batch := range sent {
		for _, e := range batch {
			seqs = append(seqs, e.SeqNum)
		}
	}
	mu.Unlock()

	for i, want := range []uint64{1, 2, 3} {
		if i >= len(seqs) {
			t.Fatalf("missing seq at index %d", i)
		}
		if seqs[i] != want {
			t.Errorf("seq[%d] = %d, want %d", i, seqs[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Buffer overflow triggers flush
// ---------------------------------------------------------------------------

func TestBridgeMessaging_BufferOverflowTriggersFlush(t *testing.T) {
	var sendCount atomic.Int32

	bm := NewBridgeMessaging(BridgeMessagingConfig{
		MaxBatchSize: 3,
		MaxQueueSize: 100,
		AckTimeout:   5 * time.Second,
		Send: func(_ context.Context, batch []BridgeEvent) error {
			sendCount.Add(1)
			return nil
		},
	})
	defer bm.Close()

	ctx := context.Background()

	// Enqueue exactly MaxBatchSize events — should auto-trigger flush.
	for i := 0; i < 3; i++ {
		if err := bm.Enqueue(ctx, BridgeEvent{Type: "assistant"}); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	// Give the drain goroutine time to run.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sendCount.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if sendCount.Load() == 0 {
		t.Error("expected auto-flush when buffer reaches MaxBatchSize, but Send was never called")
	}
}

// ---------------------------------------------------------------------------
// Ack tracking — acknowledge before timeout
// ---------------------------------------------------------------------------

func TestBridgeMessaging_AckTracking(t *testing.T) {
	bm := NewBridgeMessaging(BridgeMessagingConfig{
		MaxBatchSize: 10,
		MaxQueueSize: 100,
		AckTimeout:   2 * time.Second,
		Send: func(_ context.Context, batch []BridgeEvent) error {
			return nil
		},
	})
	defer bm.Close()

	ctx := context.Background()

	if err := bm.Enqueue(ctx, BridgeEvent{Type: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := bm.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	// SeqNum 1 should be pending ack.
	if bm.PendingAckCount() != 1 {
		t.Fatalf("expected 1 pending ack, got %d", bm.PendingAckCount())
	}

	// Acknowledge it.
	if !bm.Acknowledge(1) {
		t.Error("Acknowledge(1) returned false, expected true")
	}

	// Should be gone now.
	if bm.PendingAckCount() != 0 {
		t.Errorf("expected 0 pending acks after acknowledge, got %d", bm.PendingAckCount())
	}

	// Double-ack returns false.
	if bm.Acknowledge(1) {
		t.Error("Acknowledge(1) second call returned true, expected false")
	}
}

// ---------------------------------------------------------------------------
// Ack timeout re-queues event
// ---------------------------------------------------------------------------

func TestBridgeMessaging_AckTimeout_RequeuesEvent(t *testing.T) {
	var sendCount atomic.Int32

	bm := NewBridgeMessaging(BridgeMessagingConfig{
		MaxBatchSize: 10,
		MaxQueueSize: 100,
		AckTimeout:   100 * time.Millisecond, // short timeout for test
		Send: func(_ context.Context, batch []BridgeEvent) error {
			sendCount.Add(1)
			return nil
		},
	})
	defer bm.Close()

	ctx := context.Background()

	if err := bm.Enqueue(ctx, BridgeEvent{Type: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := bm.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	initial := sendCount.Load()
	if initial == 0 {
		t.Fatal("expected at least one send")
	}

	// Wait for ack timeout to fire and re-queue + re-send.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sendCount.Load() > initial {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if sendCount.Load() <= initial {
		t.Error("expected ack timeout to trigger re-send, but send count did not increase")
	}
}

// ---------------------------------------------------------------------------
// Sequence numbers are monotonic
// ---------------------------------------------------------------------------

func TestBridgeMessaging_SequenceNumbersMonotonic(t *testing.T) {
	var mu sync.Mutex
	var allSeqs []uint64

	bm := NewBridgeMessaging(BridgeMessagingConfig{
		MaxBatchSize: 100,
		MaxQueueSize: 200,
		AckTimeout:   5 * time.Second,
		Send: func(_ context.Context, batch []BridgeEvent) error {
			mu.Lock()
			for _, e := range batch {
				allSeqs = append(allSeqs, e.SeqNum)
			}
			mu.Unlock()
			return nil
		},
	})
	defer bm.Close()

	ctx := context.Background()
	n := 50
	for i := 0; i < n; i++ {
		if err := bm.Enqueue(ctx, BridgeEvent{Type: "user"}); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}
	if err := bm.Flush(ctx); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(allSeqs) != n {
		t.Fatalf("expected %d seqs, got %d", n, len(allSeqs))
	}
	for i := 1; i < len(allSeqs); i++ {
		if allSeqs[i] <= allSeqs[i-1] {
			t.Errorf("seq[%d]=%d not > seq[%d]=%d", i, allSeqs[i], i-1, allSeqs[i-1])
		}
	}
}

// ---------------------------------------------------------------------------
// Close unblocks Flush
// ---------------------------------------------------------------------------

func TestBridgeMessaging_CloseUnblocksFlush(t *testing.T) {
	bm := NewBridgeMessaging(BridgeMessagingConfig{
		MaxBatchSize: 100,
		MaxQueueSize: 100,
		AckTimeout:   5 * time.Second,
		Send: func(_ context.Context, batch []BridgeEvent) error {
			// Block forever — simulates a hung network call.
			select {}
		},
	})

	ctx := context.Background()

	// Enqueue one event.
	_ = bm.Enqueue(ctx, BridgeEvent{Type: "user"})

	done := make(chan struct{})
	go func() {
		_ = bm.Flush(ctx)
		close(done)
	}()

	// Give flush a moment to start.
	time.Sleep(50 * time.Millisecond)

	// Close should unblock the flush.
	bm.Close()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Flush did not unblock after Close")
	}
}

// ---------------------------------------------------------------------------
// Enqueue after Close returns error
// ---------------------------------------------------------------------------

func TestBridgeMessaging_EnqueueAfterClose(t *testing.T) {
	bm := NewBridgeMessaging(BridgeMessagingConfig{
		MaxBatchSize: 10,
		MaxQueueSize: 100,
		AckTimeout:   5 * time.Second,
		Send: func(_ context.Context, batch []BridgeEvent) error {
			return nil
		},
	})

	bm.Close()

	err := bm.Enqueue(context.Background(), BridgeEvent{Type: "user"})
	if err == nil {
		t.Error("expected error from Enqueue after Close, got nil")
	}
}

// ---------------------------------------------------------------------------
// BoundedUUIDSet — basic operations
// ---------------------------------------------------------------------------

func TestBoundedUUIDSet_AddHas(t *testing.T) {
	s := NewBoundedUUIDSet(5)

	s.Add("aaa")
	s.Add("bbb")
	s.Add("ccc")

	if !s.Has("aaa") {
		t.Error("expected Has(aaa) = true")
	}
	if !s.Has("bbb") {
		t.Error("expected Has(bbb) = true")
	}
	if s.Has("zzz") {
		t.Error("expected Has(zzz) = false")
	}
	if s.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", s.Len())
	}
}

func TestBoundedUUIDSet_Eviction(t *testing.T) {
	s := NewBoundedUUIDSet(3)

	s.Add("a")
	s.Add("b")
	s.Add("c")
	// At capacity. Adding "d" should evict "a".
	s.Add("d")

	if s.Has("a") {
		t.Error("expected 'a' to be evicted")
	}
	if !s.Has("b") || !s.Has("c") || !s.Has("d") {
		t.Error("expected b, c, d to be present")
	}
	if s.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", s.Len())
	}
}

func TestBoundedUUIDSet_DuplicateIsNoop(t *testing.T) {
	s := NewBoundedUUIDSet(3)

	s.Add("a")
	s.Add("b")
	s.Add("a") // duplicate — should not advance writeIdx
	s.Add("c")

	// All three should be present (a was not evicted by itself).
	if !s.Has("a") || !s.Has("b") || !s.Has("c") {
		t.Error("expected a, b, c to all be present after duplicate add")
	}
	if s.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", s.Len())
	}
}

func TestBoundedUUIDSet_Clear(t *testing.T) {
	s := NewBoundedUUIDSet(5)
	s.Add("x")
	s.Add("y")
	s.Clear()

	if s.Has("x") || s.Has("y") {
		t.Error("expected empty set after Clear")
	}
	if s.Len() != 0 {
		t.Errorf("expected Len()=0 after Clear, got %d", s.Len())
	}
}

func TestBoundedUUIDSet_WrapAround(t *testing.T) {
	s := NewBoundedUUIDSet(3)

	// Fill and overflow twice to exercise wrap-around.
	for i := 0; i < 9; i++ {
		s.Add(string(rune('a' + i)))
	}

	// Only the last 3 should survive: g, h, i.
	if s.Len() != 3 {
		t.Errorf("expected Len()=3 after wrap-around, got %d", s.Len())
	}
	if !s.Has("g") || !s.Has("h") || !s.Has("i") {
		t.Error("expected g, h, i to be present")
	}
	if s.Has("a") || s.Has("f") {
		t.Error("expected earlier entries to be evicted")
	}
}
