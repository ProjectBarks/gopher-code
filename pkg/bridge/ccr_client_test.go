package bridge

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test: writeEvent batching — events accumulate and flush in a single POST
// ---------------------------------------------------------------------------

func TestWriteEvent_Batching(t *testing.T) {
	var (
		mu       sync.Mutex
		received []json.RawMessage
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/code/sessions/test-sess/worker/events" {
			body, _ := io.ReadAll(r.Body)
			var req struct {
				WorkerEpoch int               `json:"worker_epoch"`
				Events      []json.RawMessage `json:"events"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("unmarshal: %v", err)
				w.WriteHeader(400)
				return
			}
			mu.Lock()
			received = append(received, req.Events...)
			mu.Unlock()
			w.WriteHeader(200)
			return
		}
		// Default: accept PUT /worker for init
		if r.Method == "PUT" {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	client, err := NewCCRClient(srv.URL+"/v1/code/sessions/test-sess", CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer tok"}
		},
		HeartbeatIntervalMS: 600_000, // very long — don't fire during test
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}
	defer client.Close()

	if err := client.Initialize(1); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Write 3 non-stream events — they accumulate in pendingEvents.
	client.WriteEvent(map[string]any{"type": "text", "text": "hello"})
	client.WriteEvent(map[string]any{"type": "text", "text": "world"})
	client.WriteEvent(map[string]any{"type": "text", "text": "!"})

	// Flush should POST all 3 in one batch.
	client.Flush()

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 3 {
		t.Errorf("expected 3 events in batch, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Test: 409 epoch mismatch triggers onEpochMismatch callback
// ---------------------------------------------------------------------------

func TestRequest_409_EpochMismatch(t *testing.T) {
	var epochMismatchCalled atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && r.URL.Path == "/v1/code/sessions/sess/worker" {
			// First call (init) succeeds, subsequent calls return 409.
			if r.Header.Get("X-Test-Init") == "true" {
				w.WriteHeader(200)
				return
			}
			w.WriteHeader(409)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client, err := NewCCRClient(srv.URL+"/v1/code/sessions/sess", CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer tok"}
		},
		OnEpochMismatch: func() {
			epochMismatchCalled.Add(1)
		},
		HeartbeatIntervalMS: 600_000,
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}
	defer client.Close()

	// Manually set epoch to skip init validation (init PUT would also 409).
	client.mu.Lock()
	client.workerEpoch = 1
	client.mu.Unlock()

	// ReportState triggers a PUT /worker which will get 409.
	client.ReportState(SessionStateProcessing, nil)

	// Give async request time to complete.
	time.Sleep(100 * time.Millisecond)

	if epochMismatchCalled.Load() == 0 {
		t.Error("expected onEpochMismatch to be called on 409, but it was not")
	}
}

// ---------------------------------------------------------------------------
// Test: heartbeat fires on schedule
// ---------------------------------------------------------------------------

func TestHeartbeat_Scheduling(t *testing.T) {
	var heartbeatCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/code/sessions/sess/worker/heartbeat" {
			heartbeatCount.Add(1)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client, err := NewCCRClient(srv.URL+"/v1/code/sessions/sess", CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer tok"}
		},
		HeartbeatIntervalMS: 50, // 50ms for fast testing
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}
	defer client.Close()

	if err := client.Initialize(1); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Wait enough for at least 2 heartbeats (50ms interval).
	time.Sleep(200 * time.Millisecond)

	count := heartbeatCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 heartbeats in 200ms with 50ms interval, got %d", count)
	}

	// Close should stop heartbeats.
	client.Close()
	countAfterClose := heartbeatCount.Load()
	time.Sleep(150 * time.Millisecond)

	if heartbeatCount.Load() > countAfterClose+1 {
		t.Error("heartbeats continued after Close()")
	}
}

// ---------------------------------------------------------------------------
// Test: consecutive auth failures counter
// ---------------------------------------------------------------------------

func TestRequest_ConsecutiveAuthFailures(t *testing.T) {
	var epochMismatchCalled atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && r.URL.Path == "/v1/code/sessions/sess/worker" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client, err := NewCCRClient(srv.URL+"/v1/code/sessions/sess", CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer tok"}
		},
		OnEpochMismatch: func() {
			epochMismatchCalled.Add(1)
		},
		HeartbeatIntervalMS: 600_000,
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}
	defer client.Close()

	client.mu.Lock()
	client.workerEpoch = 1
	client.mu.Unlock()

	// Send 10 requests — each returns 401 and increments the counter.
	for i := 0; i < MaxConsecutiveAuthFailures; i++ {
		client.ReportState(SessionStateProcessing, nil)
		// Force state change so ReportState actually fires.
		client.mu.Lock()
		client.currentState = SessionStateIdle
		client.mu.Unlock()
	}

	// Give requests time to complete.
	time.Sleep(200 * time.Millisecond)

	if epochMismatchCalled.Load() == 0 {
		t.Error("expected onEpochMismatch after 10 consecutive auth failures")
	}
}

// ---------------------------------------------------------------------------
// Test: CCRInitError reasons
// ---------------------------------------------------------------------------

func TestInitialize_NoAuthHeaders(t *testing.T) {
	client, err := NewCCRClient("https://example.com/v1/code/sessions/s1", CCRClientOpts{
		GetAuthHeaders: func() map[string]string { return nil },
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}
	defer client.Close()

	err = client.Initialize(1)
	initErr, ok := err.(*CCRInitError)
	if !ok {
		t.Fatalf("expected *CCRInitError, got %T: %v", err, err)
	}
	if initErr.Reason != InitFailNoAuthHeaders {
		t.Errorf("expected reason %q, got %q", InitFailNoAuthHeaders, initErr.Reason)
	}
}

func TestInitialize_MissingEpoch(t *testing.T) {
	client, err := NewCCRClient("https://example.com/v1/code/sessions/s1", CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer tok"}
		},
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}
	defer client.Close()

	err = client.Initialize(0) // 0 is invalid
	initErr, ok := err.(*CCRInitError)
	if !ok {
		t.Fatalf("expected *CCRInitError, got %T: %v", err, err)
	}
	if initErr.Reason != InitFailMissingEpoch {
		t.Errorf("expected reason %q, got %q", InitFailMissingEpoch, initErr.Reason)
	}
}

// ---------------------------------------------------------------------------
// Test: ReportDelivery batching
// ---------------------------------------------------------------------------

func TestReportDelivery_BatchPost(t *testing.T) {
	var (
		mu          sync.Mutex
		deliverySeen int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/code/sessions/sess/worker/events/delivery" {
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Updates []json.RawMessage `json:"updates"`
			}
			json.Unmarshal(body, &req)
			mu.Lock()
			deliverySeen += len(req.Updates)
			mu.Unlock()
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client, err := NewCCRClient(srv.URL+"/v1/code/sessions/sess", CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer tok"}
		},
		HeartbeatIntervalMS: 600_000,
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}
	defer client.Close()

	client.mu.Lock()
	client.workerEpoch = 1
	client.mu.Unlock()

	client.ReportDelivery("evt-1", DeliveryReceived)
	client.ReportDelivery("evt-2", DeliveryProcessing)
	client.ReportDelivery("evt-3", DeliveryProcessed)

	client.Flush()

	mu.Lock()
	count := deliverySeen
	mu.Unlock()

	if count != 3 {
		t.Errorf("expected 3 delivery updates, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Test: 429 returns RetryAfterMS
// ---------------------------------------------------------------------------

func TestRequest_429_RetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
	}))
	defer srv.Close()

	client, err := NewCCRClient(srv.URL+"/v1/code/sessions/sess", CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer tok"}
		},
		HeartbeatIntervalMS: 600_000,
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}
	defer client.Close()

	client.mu.Lock()
	client.workerEpoch = 1
	client.mu.Unlock()

	result := client.request("POST", "/worker/events", map[string]any{}, "test", 5*time.Second)
	if result.OK {
		t.Error("expected not OK on 429")
	}
	if result.RetryAfterMS != 30_000 {
		t.Errorf("expected RetryAfterMS=30000, got %d", result.RetryAfterMS)
	}
}

// ---------------------------------------------------------------------------
// Test: Close stops heartbeat and cancels context
// ---------------------------------------------------------------------------

func TestClose_StopsHeartbeat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client, err := NewCCRClient(srv.URL+"/v1/code/sessions/sess", CCRClientOpts{
		GetAuthHeaders: func() map[string]string {
			return map[string]string{"Authorization": "Bearer tok"}
		},
		HeartbeatIntervalMS: 50,
	})
	if err != nil {
		t.Fatalf("NewCCRClient: %v", err)
	}

	if err := client.Initialize(1); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	client.Close()

	client.mu.Lock()
	closed := client.closed
	timer := client.heartbeatTimer
	client.mu.Unlock()

	if !closed {
		t.Error("expected closed=true after Close()")
	}
	if timer != nil {
		t.Error("expected heartbeatTimer=nil after Close()")
	}
}

// ---------------------------------------------------------------------------
// Test: URL parsing
// ---------------------------------------------------------------------------

func TestNewCCRClient_InvalidScheme(t *testing.T) {
	_, err := NewCCRClient("ftp://example.com/sessions/s1", CCRClientOpts{})
	if err == nil {
		t.Error("expected error for ftp:// scheme")
	}
}
