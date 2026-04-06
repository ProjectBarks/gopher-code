// WorkerStateUploader — coalescing uploader for PUT /worker (session state +
// metadata). At most 1 in-flight + 1 pending slot. Coalesces patches via
// RFC 7396 JSON Merge Patch semantics.
// Source: src/cli/transports/WorkerStateUploader.ts
package bridge

import (
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

// WorkerStateUploaderConfig configures a WorkerStateUploader.
type WorkerStateUploaderConfig struct {
	// Send uploads the coalesced patch. Returns true on success, false to retry.
	Send func(body map[string]any) bool
	// BaseDelay is the base delay for exponential backoff.
	BaseDelay time.Duration
	// MaxDelay is the ceiling for backoff.
	MaxDelay time.Duration
	// Jitter is the random jitter range added to retry delay.
	Jitter time.Duration
}

// WorkerStateUploader coalesces state patches and uploads them serially.
// At most one send is in-flight at a time; new patches coalesce into a
// single pending slot. On failure it retries with exponential backoff,
// absorbing any new patches before each retry attempt.
type WorkerStateUploader struct {
	cfg WorkerStateUploaderConfig

	mu       sync.Mutex
	pending  map[string]any // nil when nothing pending
	inflight bool
	closed   bool
	closeCh  chan struct{}
}

// NewWorkerStateUploader creates a new uploader. Call Close when done.
func NewWorkerStateUploader(cfg WorkerStateUploaderConfig) *WorkerStateUploader {
	return &WorkerStateUploader{
		cfg:     cfg,
		closeCh: make(chan struct{}),
	}
}

// Enqueue adds a patch to be uploaded. Fire-and-forget — coalesces with any
// existing pending patch using RFC 7396 JSON Merge Patch semantics.
func (u *WorkerStateUploader) Enqueue(patch map[string]any) {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return
	}
	if u.pending != nil {
		u.pending = CoalescePatches(u.pending, patch)
	} else {
		u.pending = patch
	}
	if !u.inflight {
		u.inflight = true
		go u.drain()
	}
	u.mu.Unlock()
}

// Close halts the uploader and drops any pending patch.
func (u *WorkerStateUploader) Close() {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return
	}
	u.closed = true
	u.pending = nil
	u.mu.Unlock()
	close(u.closeCh)
}

// drain runs in a goroutine, sending the pending payload and retrying on failure.
func (u *WorkerStateUploader) drain() {
	for {
		u.mu.Lock()
		if u.closed || u.pending == nil {
			u.inflight = false
			u.mu.Unlock()
			return
		}
		payload := u.pending
		u.pending = nil
		u.mu.Unlock()

		u.sendWithRetry(payload)
	}
}

// sendWithRetry retries indefinitely with exponential backoff until success
// or close. Before each retry, absorbs any pending patches.
func (u *WorkerStateUploader) sendWithRetry(payload map[string]any) {
	current := payload
	failures := 0

	for {
		u.mu.Lock()
		if u.closed {
			u.inflight = false
			u.mu.Unlock()
			return
		}
		u.mu.Unlock()

		ok := u.cfg.Send(current)
		if ok {
			return
		}

		failures++
		delay := u.retryDelay(failures)
		u.interruptibleSleep(delay)

		// Absorb any patches that arrived during sleep.
		u.mu.Lock()
		if u.pending != nil && !u.closed {
			current = CoalescePatches(current, u.pending)
			u.pending = nil
		}
		u.mu.Unlock()
	}
}

func (u *WorkerStateUploader) retryDelay(failures int) time.Duration {
	exp := float64(u.cfg.BaseDelay) * math.Pow(2, float64(failures-1))
	if exp > float64(u.cfg.MaxDelay) {
		exp = float64(u.cfg.MaxDelay)
	}
	jitter := time.Duration(rand.Float64() * float64(u.cfg.Jitter))
	return time.Duration(exp) + jitter
}

func (u *WorkerStateUploader) interruptibleSleep(d time.Duration) {
	select {
	case <-u.closeCh:
	case <-time.After(d):
	}
}

// CoalescePatches merges two patches for PUT /worker.
//
// Top-level keys: overlay replaces base (last value wins).
// Metadata keys (external_metadata, internal_metadata): RFC 7396 merge
// one level deep — overlay keys are added/overwritten, null values
// preserved for server-side delete.
func CoalescePatches(base, overlay map[string]any) map[string]any {
	merged := make(map[string]any, len(base))
	for k, v := range base {
		merged[k] = v
	}

	for key, value := range overlay {
		if (key == "external_metadata" || key == "internal_metadata") &&
			merged[key] != nil && value != nil {
			if baseMap, ok := merged[key].(map[string]any); ok {
				if overlayMap, ok := value.(map[string]any); ok {
					m := make(map[string]any, len(baseMap)+len(overlayMap))
					for k, v := range baseMap {
						m[k] = v
					}
					for k, v := range overlayMap {
						m[k] = v // null (nil) values preserved
					}
					merged[key] = m
					continue
				}
			}
		}
		merged[key] = value
	}
	return merged
}
