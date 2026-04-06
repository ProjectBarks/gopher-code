package bridge

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- helpers ---

// collectSender returns a send func that records every batch it receives.
func collectSender[T any]() (func(context.Context, []T) error, *[][]T) {
	var mu sync.Mutex
	var batches [][]T
	send := func(_ context.Context, batch []T) error {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]T, len(batch))
		copy(cp, batch)
		batches = append(batches, cp)
		return nil
	}
	return send, &batches
}

func defaultCfg[T any](send func(context.Context, []T) error) SerialBatchUploaderConfig[T] {
	return SerialBatchUploaderConfig[T]{
		MaxBatchSize:  10,
		MaxQueueSize:  100,
		Send:          send,
		BaseDelay:     time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		Jitter:        time.Millisecond,
	}
}

// --- tests ---

func TestBatchSizeCap(t *testing.T) {
	send, batches := collectSender[int]()
	cfg := defaultCfg(send)
	cfg.MaxBatchSize = 3
	cfg.MaxQueueSize = 20

	u := NewSerialBatchEventUploader(cfg)
	for i := 0; i < 7; i++ {
		u.Enqueue(i)
	}
	u.Flush()
	u.Close()

	// We sent 7 items with batch size 3 => batches of [3, 3, 1].
	total := 0
	for _, b := range *batches {
		if len(b) > 3 {
			t.Errorf("batch size %d exceeds cap 3", len(b))
		}
		total += len(b)
	}
	if total != 7 {
		t.Errorf("expected 7 total items, got %d", total)
	}
}

func TestByteBudget(t *testing.T) {
	// Each string "aa" marshals to `"aa"` = 4 bytes; "bbbb" -> `"bbbb"` = 6 bytes.
	send, batches := collectSender[string]()
	cfg := defaultCfg(send)
	cfg.MaxBatchSize = 10
	cfg.MaxBatchBytes = 10 // budget: 10 bytes
	cfg.MaxQueueSize = 20

	u := NewSerialBatchEventUploader(cfg)
	// "aa" = 4 bytes, "bb" = 4 bytes, "cccc" = 6 bytes
	u.Enqueue("aa", "bb", "cccc")
	u.Flush()
	u.Close()

	// First batch: "aa"(4) + "bb"(4) = 8 <= 10, then "cccc"(6) => 14 > 10 => stop.
	// So batch 1 = ["aa","bb"], batch 2 = ["cccc"].
	if len(*batches) < 2 {
		t.Fatalf("expected at least 2 batches, got %d: %v", len(*batches), *batches)
	}
	if len((*batches)[0]) != 2 {
		t.Errorf("first batch should have 2 items, got %d", len((*batches)[0]))
	}
}

func TestFirstItemAlwaysIncluded(t *testing.T) {
	send, batches := collectSender[string]()
	cfg := defaultCfg(send)
	cfg.MaxBatchSize = 10
	cfg.MaxBatchBytes = 5 // very small budget
	cfg.MaxQueueSize = 20

	u := NewSerialBatchEventUploader(cfg)
	// "this_is_huge" marshals to `"this_is_huge"` = 14 bytes, way over 5.
	u.Enqueue("this_is_huge")
	u.Flush()
	u.Close()

	if len(*batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(*batches))
	}
	if len((*batches)[0]) != 1 || (*batches)[0][0] != "this_is_huge" {
		t.Errorf("first item should always be included, got %v", (*batches)[0])
	}
}

func TestBackoffOnFailure(t *testing.T) {
	var attempts atomic.Int32
	send := func(_ context.Context, _ []string) error {
		n := attempts.Add(1)
		if n <= 2 {
			return errors.New("transient failure")
		}
		return nil
	}

	cfg := defaultCfg(send)
	cfg.Send = send
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = 5 * time.Millisecond
	cfg.Jitter = 0 // deterministic for test
	cfg.MaxQueueSize = 10

	u := NewSerialBatchEventUploader(cfg)
	u.Enqueue("event1")
	u.Flush()
	u.Close()

	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", attempts.Load())
	}
}

func TestRetryableErrorDelay(t *testing.T) {
	var attempts atomic.Int32
	var retryTimes []time.Time
	var mu sync.Mutex

	send := func(_ context.Context, _ []string) error {
		mu.Lock()
		retryTimes = append(retryTimes, time.Now())
		mu.Unlock()
		n := attempts.Add(1)
		if n == 1 {
			return &RetryableError{
				Err:        errors.New("rate limited"),
				RetryAfter: 20 * time.Millisecond,
			}
		}
		return nil
	}

	cfg := defaultCfg(send)
	cfg.Send = send
	cfg.BaseDelay = 5 * time.Millisecond
	cfg.MaxDelay = 50 * time.Millisecond
	cfg.Jitter = 0
	cfg.MaxQueueSize = 10

	u := NewSerialBatchEventUploader(cfg)
	u.Enqueue("event")
	u.Flush()
	u.Close()

	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	gap := retryTimes[1].Sub(retryTimes[0])
	// RetryAfter=20ms, clamped to [5ms, 50ms] = 20ms, jitter=0.
	if gap < 15*time.Millisecond {
		t.Errorf("retry gap %v is too short, expected ~20ms", gap)
	}
}

func TestRetryAfterClamp(t *testing.T) {
	// retryAfterMs below baseDelay should be clamped up.
	var attempts atomic.Int32
	var retryTimes []time.Time
	var mu sync.Mutex

	send := func(_ context.Context, _ []string) error {
		mu.Lock()
		retryTimes = append(retryTimes, time.Now())
		mu.Unlock()
		n := attempts.Add(1)
		if n == 1 {
			return &RetryableError{
				Err:        errors.New("rate limited"),
				RetryAfter: time.Microsecond, // way below baseDelay
			}
		}
		return nil
	}

	cfg := defaultCfg(send)
	cfg.Send = send
	cfg.BaseDelay = 10 * time.Millisecond
	cfg.MaxDelay = 100 * time.Millisecond
	cfg.Jitter = 0
	cfg.MaxQueueSize = 10

	u := NewSerialBatchEventUploader(cfg)
	u.Enqueue("event")
	u.Flush()
	u.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(retryTimes) >= 2 {
		gap := retryTimes[1].Sub(retryTimes[0])
		if gap < 8*time.Millisecond {
			t.Errorf("retry gap %v should be at least ~10ms (clamped to baseDelay)", gap)
		}
	}
}

func TestDropAfterMaxFailures(t *testing.T) {
	var droppedSize int
	var droppedFailures int

	send := func(_ context.Context, _ []string) error {
		return errors.New("permanent failure")
	}

	cfg := defaultCfg(send)
	cfg.Send = send
	cfg.MaxConsecutiveFailures = 3
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = time.Millisecond
	cfg.Jitter = 0
	cfg.MaxQueueSize = 10
	cfg.OnBatchDropped = func(size, failures int) {
		droppedSize = size
		droppedFailures = failures
	}

	u := NewSerialBatchEventUploader(cfg)
	u.Enqueue("doomed")
	u.Flush()
	u.Close()

	if u.DroppedBatchCount() != 1 {
		t.Errorf("expected 1 dropped batch, got %d", u.DroppedBatchCount())
	}
	if droppedSize != 1 {
		t.Errorf("expected dropped batch size 1, got %d", droppedSize)
	}
	if droppedFailures != 3 {
		t.Errorf("expected 3 failures before drop, got %d", droppedFailures)
	}
}

func TestCloseInterruptsSleep(t *testing.T) {
	// send always fails so drain will sleep; close should interrupt immediately.
	send := func(_ context.Context, _ []string) error {
		return errors.New("fail")
	}

	cfg := defaultCfg(send)
	cfg.Send = send
	cfg.BaseDelay = 10 * time.Second // very long, would hang without interrupt
	cfg.MaxDelay = 10 * time.Second
	cfg.Jitter = 0
	cfg.MaxQueueSize = 10

	u := NewSerialBatchEventUploader(cfg)
	u.Enqueue("event")

	// Give the drain loop time to start sleeping.
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		u.Close()
		close(done)
	}()

	select {
	case <-done:
		// OK — close returned promptly.
	case <-time.After(2 * time.Second):
		t.Fatal("Close() did not return within 2s; interruptible sleep may be broken")
	}
}

func TestFlushDrainWait(t *testing.T) {
	var count atomic.Int32
	send := func(_ context.Context, batch []int) error {
		count.Add(int32(len(batch)))
		return nil
	}

	cfg := defaultCfg(send)
	cfg.Send = send
	cfg.MaxBatchSize = 5
	cfg.MaxQueueSize = 50

	u := NewSerialBatchEventUploader(cfg)
	for i := 0; i < 20; i++ {
		u.Enqueue(i)
	}
	u.Flush()

	if count.Load() != 20 {
		t.Errorf("after Flush(), expected all 20 items sent, got %d", count.Load())
	}
	u.Close()
}

func TestBackpressureBlocks(t *testing.T) {
	// send blocks on the first call until we release the gate.
	// This keeps the drain goroutine stuck in send() while we fill the queue.
	gate := make(chan struct{})
	sendStarted := make(chan struct{}, 1)
	send := func(_ context.Context, _ []int) error {
		select {
		case sendStarted <- struct{}{}:
		default:
		}
		<-gate
		return nil
	}

	cfg := defaultCfg(send)
	cfg.Send = send
	cfg.MaxBatchSize = 1 // take 1 item per batch
	cfg.MaxQueueSize = 2

	u := NewSerialBatchEventUploader(cfg)
	// Enqueue 1 item — drain starts and send blocks on gate.
	u.Enqueue(1)
	<-sendStarted // wait for drain to enter send()

	// Now enqueue 2 more — queue capacity is 2, so this fills it.
	u.Enqueue(2, 3)

	blocked := make(chan struct{})
	go func() {
		u.Enqueue(4) // should block — queue full (2 pending + 1 in-flight)
		close(blocked)
	}()

	select {
	case <-blocked:
		t.Fatal("Enqueue should have blocked but returned immediately")
	case <-time.After(50 * time.Millisecond):
		// OK — it's blocking as expected.
	}

	// Unblock the send so drain frees space.
	close(gate)

	select {
	case <-blocked:
		// OK — unblocked after drain freed space.
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue still blocked after drain freed space")
	}

	u.Close()
}
