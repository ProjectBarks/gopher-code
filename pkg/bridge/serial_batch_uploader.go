// SerialBatchEventUploader — generic serial-ordered event uploader with
// batching, retry, backpressure, and exponential backoff.
// Source: src/cli/transports/SerialBatchEventUploader.ts
package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
)

// RetryableError can be returned from SendFunc to request a server-supplied
// retry delay (e.g. 429 Retry-After). When RetryAfter is set it overrides
// exponential backoff for that attempt, clamped to [BaseDelay, MaxDelay] and
// jittered.
type RetryableError struct {
	Err        error
	RetryAfter time.Duration // zero means use normal backoff
}

func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

// SerialBatchUploaderConfig configures a SerialBatchEventUploader.
type SerialBatchUploaderConfig[T any] struct {
	MaxBatchSize  int // max items per POST (1 = no batching)
	MaxBatchBytes int // byte cap per POST; 0 = no byte limit
	MaxQueueSize  int // backpressure threshold
	// Send is the actual upload call. Return nil on success or an error
	// (optionally *RetryableError) on failure.
	Send func(ctx context.Context, batch []T) error
	BaseDelay     time.Duration // base delay for exponential backoff
	MaxDelay      time.Duration // ceiling for backoff
	Jitter        time.Duration // random jitter added to retry delay
	// MaxConsecutiveFailures: after this many consecutive failures, drop the
	// batch and move on. Zero means retry forever.
	MaxConsecutiveFailures int
	// OnBatchDropped is called when a batch is dropped after hitting
	// MaxConsecutiveFailures.
	OnBatchDropped func(batchSize int, failures int)
	// SizeFunc returns the serialized byte size of an item. If nil, items are
	// marshaled with encoding/json.
	SizeFunc func(T) int
}

// SerialBatchEventUploader batches events for serial HTTP POST upload.
// At most one send is in-flight at a time. It supports dual-cap batching
// (by count and by bytes), backpressure, exponential backoff with jitter,
// and a max-consecutive-failure drop policy.
type SerialBatchEventUploader[T any] struct {
	cfg SerialBatchUploaderConfig[T]

	mu       sync.Mutex
	pending  []T
	closed   bool
	draining bool

	// Backpressure: enqueue blocks when queue is full by waiting on this cond.
	cond *sync.Cond

	// flushWaiters are notified when drain completes with an empty queue.
	flushWaiters []chan struct{}

	// closeCh is closed on Close() to interrupt sleeps.
	closeCh chan struct{}

	// drainDone is closed when the drain goroutine exits.
	drainDone chan struct{}

	droppedBatches atomic.Int64
	pendingAtClose atomic.Int64
}

// NewSerialBatchEventUploader creates a new uploader. Call Close when done.
func NewSerialBatchEventUploader[T any](cfg SerialBatchUploaderConfig[T]) *SerialBatchEventUploader[T] {
	u := &SerialBatchEventUploader[T]{
		cfg:       cfg,
		closeCh:   make(chan struct{}),
		drainDone: make(chan struct{}),
	}
	u.cond = sync.NewCond(&u.mu)
	return u
}

// DroppedBatchCount returns the monotonic count of batches dropped via
// MaxConsecutiveFailures.
func (u *SerialBatchEventUploader[T]) DroppedBatchCount() int64 {
	return u.droppedBatches.Load()
}

// PendingCount returns the number of pending items. After Close, returns the
// count at close time.
func (u *SerialBatchEventUploader[T]) PendingCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return int(u.pendingAtClose.Load())
	}
	return len(u.pending)
}

// Enqueue adds items to the pending buffer. Blocks if the queue is full
// (backpressure). Returns immediately if closed.
func (u *SerialBatchEventUploader[T]) Enqueue(items ...T) {
	if len(items) == 0 {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return
	}

	// Backpressure: wait until there's space.
	for len(u.pending)+len(items) > u.cfg.MaxQueueSize && !u.closed {
		u.cond.Wait()
	}
	if u.closed {
		return
	}

	u.pending = append(u.pending, items...)
	u.startDrainLocked()
}

// Flush blocks until all pending events have been sent (or close is called).
func (u *SerialBatchEventUploader[T]) Flush() {
	u.mu.Lock()
	if len(u.pending) == 0 && !u.draining {
		u.mu.Unlock()
		return
	}
	ch := make(chan struct{})
	u.flushWaiters = append(u.flushWaiters, ch)
	u.startDrainLocked()
	u.mu.Unlock()
	<-ch
}

// Close drops pending events and stops processing. Unblocks any blocked
// Enqueue and Flush callers.
func (u *SerialBatchEventUploader[T]) Close() {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return
	}
	u.closed = true
	u.pendingAtClose.Store(int64(len(u.pending)))
	u.pending = nil
	// Wake up backpressure waiters.
	u.cond.Broadcast()
	// Wake up flush waiters.
	for _, ch := range u.flushWaiters {
		close(ch)
	}
	u.flushWaiters = nil
	u.mu.Unlock()
	// Interrupt any sleep in the drain loop.
	close(u.closeCh)
}

// startDrainLocked kicks off the drain goroutine if not already running.
// Must be called with u.mu held.
func (u *SerialBatchEventUploader[T]) startDrainLocked() {
	if u.draining || u.closed {
		return
	}
	u.draining = true
	u.drainDone = make(chan struct{})
	go u.drain()
}

func (u *SerialBatchEventUploader[T]) drain() {
	failures := 0
	defer func() {
		u.mu.Lock()
		u.draining = false
		if len(u.pending) == 0 {
			for _, ch := range u.flushWaiters {
				close(ch)
			}
			u.flushWaiters = nil
		}
		done := u.drainDone
		u.mu.Unlock()
		close(done)
	}()

	for {
		u.mu.Lock()
		if len(u.pending) == 0 || u.closed {
			u.mu.Unlock()
			return
		}
		batch := u.takeBatchLocked()
		u.mu.Unlock()

		if len(batch) == 0 {
			continue
		}

		ctx := context.Background()
		err := u.cfg.Send(ctx, batch)
		if err == nil {
			failures = 0
			u.mu.Lock()
			u.cond.Broadcast()
			u.mu.Unlock()
			continue
		}

		// Failure path.
		failures++
		if u.cfg.MaxConsecutiveFailures > 0 && failures >= u.cfg.MaxConsecutiveFailures {
			u.droppedBatches.Add(1)
			if u.cfg.OnBatchDropped != nil {
				u.cfg.OnBatchDropped(len(batch), failures)
			}
			failures = 0
			u.mu.Lock()
			u.cond.Broadcast()
			u.mu.Unlock()
			continue
		}

		// Re-queue failed batch at the front.
		u.mu.Lock()
		u.pending = append(batch, u.pending...)
		u.mu.Unlock()

		delay := u.retryDelay(failures, err)
		u.interruptibleSleep(delay)

		u.mu.Lock()
		if u.closed {
			u.mu.Unlock()
			return
		}
		u.mu.Unlock()
	}
}

// takeBatchLocked pulls the next batch from pending respecting both
// MaxBatchSize and MaxBatchBytes. The first item is always included
// regardless of byte size. Must be called with u.mu held.
func (u *SerialBatchEventUploader[T]) takeBatchLocked() []T {
	if u.cfg.MaxBatchBytes <= 0 {
		// Count-only batching.
		n := u.cfg.MaxBatchSize
		if n > len(u.pending) {
			n = len(u.pending)
		}
		batch := make([]T, n)
		copy(batch, u.pending[:n])
		u.pending = u.pending[n:]
		return batch
	}

	// Dual-cap batching.
	var bytes int
	var count int
	for count < len(u.pending) && count < u.cfg.MaxBatchSize {
		itemBytes := u.itemSize(u.pending[count])
		if itemBytes < 0 {
			// Un-serializable — drop in place.
			u.pending = append(u.pending[:count], u.pending[count+1:]...)
			continue
		}
		if count > 0 && bytes+itemBytes > u.cfg.MaxBatchBytes {
			break
		}
		bytes += itemBytes
		count++
	}
	batch := make([]T, count)
	copy(batch, u.pending[:count])
	u.pending = u.pending[count:]
	return batch
}

func (u *SerialBatchEventUploader[T]) itemSize(item T) int {
	if u.cfg.SizeFunc != nil {
		return u.cfg.SizeFunc(item)
	}
	b, err := json.Marshal(item)
	if err != nil {
		return -1
	}
	return len(b)
}

func (u *SerialBatchEventUploader[T]) retryDelay(failures int, err error) time.Duration {
	jitter := time.Duration(rand.Float64() * float64(u.cfg.Jitter))

	var retryErr *RetryableError
	if errors.As(err, &retryErr) && retryErr.RetryAfter > 0 {
		clamped := retryErr.RetryAfter
		if clamped < u.cfg.BaseDelay {
			clamped = u.cfg.BaseDelay
		}
		if clamped > u.cfg.MaxDelay {
			clamped = u.cfg.MaxDelay
		}
		return clamped + jitter
	}

	exp := float64(u.cfg.BaseDelay) * math.Pow(2, float64(failures-1))
	if exp > float64(u.cfg.MaxDelay) {
		exp = float64(u.cfg.MaxDelay)
	}
	return time.Duration(exp) + jitter
}

func (u *SerialBatchEventUploader[T]) interruptibleSleep(d time.Duration) {
	select {
	case <-u.closeCh:
	case <-time.After(d):
	}
}
