package bridge

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------- CoalescePatches tests ----------

func TestCoalescePatches_TopLevelLastWins(t *testing.T) {
	base := map[string]any{"worker_status": "idle", "foo": 1}
	overlay := map[string]any{"worker_status": "busy", "bar": 2}
	got := CoalescePatches(base, overlay)

	if got["worker_status"] != "busy" {
		t.Errorf("worker_status = %v, want busy", got["worker_status"])
	}
	if got["foo"] != 1 {
		t.Errorf("foo = %v, want 1", got["foo"])
	}
	if got["bar"] != 2 {
		t.Errorf("bar = %v, want 2", got["bar"])
	}
}

func TestCoalescePatches_MetadataDeepMerge(t *testing.T) {
	base := map[string]any{
		"external_metadata": map[string]any{"a": 1, "b": 2},
	}
	overlay := map[string]any{
		"external_metadata": map[string]any{"b": 99, "c": 3},
	}
	got := CoalescePatches(base, overlay)
	meta := got["external_metadata"].(map[string]any)

	if meta["a"] != 1 {
		t.Errorf("a = %v, want 1", meta["a"])
	}
	if meta["b"] != 99 {
		t.Errorf("b = %v, want 99", meta["b"])
	}
	if meta["c"] != 3 {
		t.Errorf("c = %v, want 3", meta["c"])
	}
}

func TestCoalescePatches_NullAsDeletePreserved(t *testing.T) {
	base := map[string]any{
		"external_metadata": map[string]any{"a": 1, "b": 2},
	}
	overlay := map[string]any{
		"external_metadata": map[string]any{"b": nil},
	}
	got := CoalescePatches(base, overlay)
	meta := got["external_metadata"].(map[string]any)

	if meta["a"] != 1 {
		t.Errorf("a = %v, want 1", meta["a"])
	}
	// nil must be preserved (not deleted from map) — server interprets as delete.
	v, ok := meta["b"]
	if !ok {
		t.Fatal("b key missing; null-as-delete must preserve the key")
	}
	if v != nil {
		t.Errorf("b = %v, want nil", v)
	}
}

func TestCoalescePatches_InternalMetadataMerge(t *testing.T) {
	base := map[string]any{
		"internal_metadata": map[string]any{"x": 10},
	}
	overlay := map[string]any{
		"internal_metadata": map[string]any{"y": 20},
	}
	got := CoalescePatches(base, overlay)
	meta := got["internal_metadata"].(map[string]any)

	if meta["x"] != 10 {
		t.Errorf("x = %v, want 10", meta["x"])
	}
	if meta["y"] != 20 {
		t.Errorf("y = %v, want 20", meta["y"])
	}
}

func TestCoalescePatches_MetadataOverlayNotMap(t *testing.T) {
	// If metadata value is not a map, top-level last-wins applies.
	base := map[string]any{
		"external_metadata": map[string]any{"a": 1},
	}
	overlay := map[string]any{
		"external_metadata": "reset",
	}
	got := CoalescePatches(base, overlay)
	if got["external_metadata"] != "reset" {
		t.Errorf("external_metadata = %v, want reset", got["external_metadata"])
	}
}

func TestCoalescePatches_MetadataOverlayNull(t *testing.T) {
	// If metadata overlay is nil, top-level last-wins (set to nil).
	base := map[string]any{
		"external_metadata": map[string]any{"a": 1},
	}
	overlay := map[string]any{
		"external_metadata": nil,
	}
	got := CoalescePatches(base, overlay)
	if got["external_metadata"] != nil {
		t.Errorf("external_metadata = %v, want nil", got["external_metadata"])
	}
}

func TestCoalescePatches_BaseDoesNotMutate(t *testing.T) {
	base := map[string]any{"k": "v"}
	overlay := map[string]any{"k": "w"}
	_ = CoalescePatches(base, overlay)
	if base["k"] != "v" {
		t.Errorf("base mutated: k = %v, want v", base["k"])
	}
}

// ---------- WorkerStateUploader integration tests ----------

func TestWorkerStateUploader_SinglePatch(t *testing.T) {
	var mu sync.Mutex
	var sent []map[string]any

	u := NewWorkerStateUploader(WorkerStateUploaderConfig{
		Send: func(body map[string]any) bool {
			mu.Lock()
			sent = append(sent, body)
			mu.Unlock()
			return true
		},
		BaseDelay: time.Millisecond,
		MaxDelay:  10 * time.Millisecond,
		Jitter:    0,
	})
	defer u.Close()

	u.Enqueue(map[string]any{"worker_status": "busy"})

	// Wait for delivery.
	deadline := time.After(time.Second)
	for {
		mu.Lock()
		n := len(sent)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for send")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if sent[0]["worker_status"] != "busy" {
		t.Errorf("sent[0] = %v, want worker_status=busy", sent[0])
	}
}

func TestWorkerStateUploader_CoalescesEnqueuedPatches(t *testing.T) {
	// Block the first send so subsequent enqueues coalesce.
	sendCh := make(chan struct{})
	var mu sync.Mutex
	var sent []map[string]any

	u := NewWorkerStateUploader(WorkerStateUploaderConfig{
		Send: func(body map[string]any) bool {
			mu.Lock()
			n := len(sent)
			mu.Unlock()
			if n == 0 {
				// First send: block until released, then succeed.
				<-sendCh
			}
			mu.Lock()
			sent = append(sent, body)
			mu.Unlock()
			return true
		},
		BaseDelay: time.Millisecond,
		MaxDelay:  10 * time.Millisecond,
		Jitter:    0,
	})
	defer u.Close()

	// First enqueue triggers in-flight send (which blocks).
	u.Enqueue(map[string]any{"worker_status": "idle"})
	// Give goroutine time to start send.
	time.Sleep(5 * time.Millisecond)

	// These should coalesce into a single pending patch.
	u.Enqueue(map[string]any{"worker_status": "busy"})
	u.Enqueue(map[string]any{"external_metadata": map[string]any{"a": 1}})
	u.Enqueue(map[string]any{"external_metadata": map[string]any{"b": 2}})

	// Release first send.
	close(sendCh)

	// Wait for second (coalesced) send.
	deadline := time.After(time.Second)
	for {
		mu.Lock()
		n := len(sent)
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for coalesced send")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	// Second send should be the coalesced patch.
	coalesced := sent[1]
	if coalesced["worker_status"] != "busy" {
		t.Errorf("worker_status = %v, want busy", coalesced["worker_status"])
	}
	meta := coalesced["external_metadata"].(map[string]any)
	if meta["a"] != 1 || meta["b"] != 2 {
		t.Errorf("metadata = %v, want a=1,b=2", meta)
	}
}

func TestWorkerStateUploader_AbsorbOnRetry(t *testing.T) {
	// First call fails, second succeeds. Between them a new patch arrives.
	var callCount atomic.Int32
	absorbReady := make(chan struct{})
	var mu sync.Mutex
	var sent []map[string]any

	u := NewWorkerStateUploader(WorkerStateUploaderConfig{
		Send: func(body map[string]any) bool {
			n := callCount.Add(1)
			if n == 1 {
				// First call fails. Signal test to enqueue more.
				close(absorbReady)
				return false
			}
			mu.Lock()
			sent = append(sent, body)
			mu.Unlock()
			return true
		},
		BaseDelay: 5 * time.Millisecond,
		MaxDelay:  10 * time.Millisecond,
		Jitter:    0,
	})
	defer u.Close()

	u.Enqueue(map[string]any{"worker_status": "idle"})

	// Wait for the first (failing) send.
	select {
	case <-absorbReady:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first send")
	}

	// Enqueue a new patch during the retry backoff window.
	u.Enqueue(map[string]any{"worker_status": "active", "external_metadata": map[string]any{"k": "v"}})

	// Wait for the successful retry send.
	deadline := time.After(time.Second)
	for {
		mu.Lock()
		n := len(sent)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for retry send")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	// The retry should have absorbed the new patch.
	absorbed := sent[0]
	if absorbed["worker_status"] != "active" {
		t.Errorf("worker_status = %v, want active (absorbed)", absorbed["worker_status"])
	}
	meta := absorbed["external_metadata"].(map[string]any)
	if meta["k"] != "v" {
		t.Errorf("metadata = %v, want k=v", meta)
	}
}

func TestWorkerStateUploader_ExponentialBackoff(t *testing.T) {
	// Track delays between consecutive send calls.
	var mu sync.Mutex
	var times []time.Time
	done := make(chan struct{})

	u := NewWorkerStateUploader(WorkerStateUploaderConfig{
		Send: func(body map[string]any) bool {
			mu.Lock()
			times = append(times, time.Now())
			n := len(times)
			mu.Unlock()
			if n >= 4 {
				close(done)
				return true
			}
			return false
		},
		BaseDelay: 10 * time.Millisecond,
		MaxDelay:  100 * time.Millisecond,
		Jitter:    0,
	})
	defer u.Close()

	u.Enqueue(map[string]any{"x": 1})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for backoff retries")
	}

	mu.Lock()
	defer mu.Unlock()

	// We expect delays: ~10ms, ~20ms, ~40ms (exponential).
	// Just verify they're increasing and non-zero.
	for i := 1; i < len(times); i++ {
		gap := times[i].Sub(times[i-1])
		if gap < 5*time.Millisecond {
			t.Errorf("gap[%d] = %v, expected >=5ms", i, gap)
		}
	}
	// Verify second gap >= first gap (exponential growth).
	if len(times) >= 3 {
		gap1 := times[1].Sub(times[0])
		gap2 := times[2].Sub(times[1])
		if gap2 < gap1 {
			t.Errorf("backoff not increasing: gap1=%v, gap2=%v", gap1, gap2)
		}
	}
}

func TestWorkerStateUploader_CloseDropsPending(t *testing.T) {
	// Send blocks forever.
	u := NewWorkerStateUploader(WorkerStateUploaderConfig{
		Send: func(body map[string]any) bool {
			time.Sleep(time.Hour)
			return true
		},
		BaseDelay: time.Millisecond,
		MaxDelay:  time.Millisecond,
		Jitter:    0,
	})

	u.Enqueue(map[string]any{"x": 1})
	time.Sleep(5 * time.Millisecond)

	// Enqueue something that should be pending.
	u.Enqueue(map[string]any{"y": 2})

	u.Close()

	// After close, enqueue is a no-op.
	u.Enqueue(map[string]any{"z": 3})

	u.mu.Lock()
	defer u.mu.Unlock()
	if u.pending != nil {
		t.Errorf("pending should be nil after close, got %v", u.pending)
	}
	if !u.closed {
		t.Error("closed should be true")
	}
}

func TestWorkerStateUploader_CloseStopsRetry(t *testing.T) {
	// Send always fails.
	var callCount atomic.Int32
	firstFail := make(chan struct{})

	u := NewWorkerStateUploader(WorkerStateUploaderConfig{
		Send: func(body map[string]any) bool {
			n := callCount.Add(1)
			if n == 1 {
				close(firstFail)
			}
			return false
		},
		BaseDelay: 50 * time.Millisecond,
		MaxDelay:  100 * time.Millisecond,
		Jitter:    0,
	})

	u.Enqueue(map[string]any{"x": 1})

	select {
	case <-firstFail:
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	u.Close()
	time.Sleep(20 * time.Millisecond)

	// After close + small sleep, no more sends should happen.
	countAtClose := callCount.Load()
	time.Sleep(150 * time.Millisecond) // longer than MaxDelay
	countAfter := callCount.Load()

	if countAfter > countAtClose+1 {
		t.Errorf("sends after close: before=%d, after=%d", countAtClose, countAfter)
	}
}
