// Package bridge — ccr_client.go implements the CCR v2 HTTP client for the
// worker event protocol.
// Source: src/cli/transports/ccrClient.ts
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
)

// ---------------------------------------------------------------------------
// Constants — verbatim from TS source
// ---------------------------------------------------------------------------

// DefaultHeartbeatIntervalMS is the heartbeat period (20s; server TTL is 60s).
const DefaultHeartbeatIntervalMS = 20_000

// StreamEventFlushIntervalMS is the stream_event batch coalescing window.
const StreamEventFlushIntervalMS = 100

// MaxConsecutiveAuthFailures caps 401/403 retries with a valid-looking token.
// 10 x 20s heartbeat ~ 200s to ride out transient server auth issues.
const MaxConsecutiveAuthFailures = 10

// ---------------------------------------------------------------------------
// CCRInitError — typed init failure
// ---------------------------------------------------------------------------

// CCRInitFailReason is the reason Initialize failed.
type CCRInitFailReason string

const (
	InitFailNoAuthHeaders       CCRInitFailReason = "no_auth_headers"
	InitFailMissingEpoch        CCRInitFailReason = "missing_epoch"
	InitFailWorkerRegisterFailed CCRInitFailReason = "worker_register_failed"
)

// CCRInitError is returned by Initialize when the init handshake fails.
type CCRInitError struct {
	Reason CCRInitFailReason
}

func (e *CCRInitError) Error() string {
	return fmt.Sprintf("CCRClient init failed: %s", e.Reason)
}

// ---------------------------------------------------------------------------
// Event payload types
// ---------------------------------------------------------------------------

// EventPayload is a single event in the worker protocol. It always has a UUID
// and type; extra fields are marshalled from the map.
type EventPayload map[string]any

// ClientEvent wraps a payload for the event uploader.
type ClientEvent struct {
	Payload   EventPayload `json:"payload"`
	Ephemeral bool         `json:"ephemeral,omitempty"`
}

// WorkerEvent wraps a payload for the internal event uploader.
type WorkerEvent struct {
	Payload      EventPayload `json:"payload"`
	IsCompaction bool         `json:"is_compaction,omitempty"`
	AgentID      string       `json:"agent_id,omitempty"`
}

// InternalEvent is a persisted worker-internal event read back for resume.
type InternalEvent struct {
	EventID       string         `json:"event_id"`
	EventType     string         `json:"event_type"`
	Payload       map[string]any `json:"payload"`
	EventMetadata map[string]any `json:"event_metadata,omitempty"`
	IsCompaction  bool           `json:"is_compaction"`
	CreatedAt     string         `json:"created_at"`
	AgentID       string         `json:"agent_id,omitempty"`
}

// DeliveryStatus is the acknowledgement status for a delivered event.
type DeliveryStatus string

const (
	DeliveryReceived   DeliveryStatus = "received"
	DeliveryProcessing DeliveryStatus = "processing"
	DeliveryProcessed  DeliveryStatus = "processed"
)

// deliveryItem is a single delivery ACK waiting to be batched.
type deliveryItem struct {
	EventID string         `json:"event_id"`
	Status  DeliveryStatus `json:"status"`
}

// listInternalEventsResponse is the JSON envelope for GET internal-events.
type listInternalEventsResponse struct {
	Data       []InternalEvent `json:"data"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// RequestResult is the outcome of a single HTTP request to CCR.
type RequestResult struct {
	OK           bool
	RetryAfterMS int // >0 when the server sent Retry-After on 429
}

// ---------------------------------------------------------------------------
// EpochMismatchError — sentinel for 409
// ---------------------------------------------------------------------------

// EpochMismatchError is returned when the server responds 409.
type EpochMismatchError struct{}

func (e *EpochMismatchError) Error() string {
	return "CCRClient: epoch mismatch (409)"
}

// RequiresActionDetails provides context when state is requires_action.
type RequiresActionDetails struct {
	ToolName          string `json:"tool_name"`
	ActionDescription string `json:"action_description"`
	RequestID         string `json:"request_id"`
}

// ---------------------------------------------------------------------------
// CCRClientOpts — constructor options
// ---------------------------------------------------------------------------

// CCRClientOpts configures a CCRClient.
type CCRClientOpts struct {
	// OnEpochMismatch is called on 409. Must not return (e.g. os.Exit or
	// close the session). If nil, defaults to a no-op that returns
	// EpochMismatchError to the caller.
	OnEpochMismatch func()

	// HeartbeatIntervalMS overrides the default 20s heartbeat period.
	HeartbeatIntervalMS int

	// HeartbeatJitterFraction adds +/- fraction of the interval as jitter.
	HeartbeatJitterFraction float64

	// GetAuthHeaders returns HTTP auth headers for every request.
	// Must return at least one header for requests to proceed.
	GetAuthHeaders func() map[string]string

	// UserAgent is sent as User-Agent on every request.
	UserAgent string
}

// ---------------------------------------------------------------------------
// CCRClient
// ---------------------------------------------------------------------------

// CCRClient is an HTTP client for the CCR v2 worker event protocol. It posts
// events in batches, sends heartbeats, handles 409 epoch mismatch, and reads
// internal events for session resume.
type CCRClient struct {
	mu sync.Mutex

	sessionBaseURL string
	sessionID      string
	workerEpoch    int

	heartbeatIntervalMS     int
	heartbeatJitterFraction float64
	heartbeatTimer          *time.Timer
	heartbeatInFlight       bool

	closed                   bool
	consecutiveAuthFailures  int
	currentState             SessionState

	onEpochMismatch func()
	getAuthHeaders  func() map[string]string
	userAgent       string

	// Batch buffers — each is a simple slice guarded by mu.
	pendingEvents         []ClientEvent
	pendingInternalEvents []WorkerEvent
	pendingDeliveries     []deliveryItem

	// Stream event coalescing buffer and timer.
	streamEventBuffer []map[string]any
	streamEventTimer  *time.Timer
	streamAccumulator *StreamAccumulatorState

	// HTTP client (retryable).
	httpClient *retryablehttp.Client

	// ctx/cancel for the heartbeat goroutine.
	ctx    context.Context
	cancel context.CancelFunc
}

// NewCCRClient creates a new CCRClient for the given session URL.
// sessionURL should be like https://host/v1/code/sessions/{id}.
func NewCCRClient(sessionURL string, opts CCRClientOpts) (*CCRClient, error) {
	u, err := url.Parse(sessionURL)
	if err != nil {
		return nil, fmt.Errorf("CCRClient: invalid session URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("CCRClient: expected http(s) URL, got %s", u.Scheme)
	}

	pathname := strings.TrimRight(u.Path, "/")
	baseURL := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, pathname)

	parts := strings.Split(pathname, "/")
	sessionID := ""
	if len(parts) > 0 {
		sessionID = parts[len(parts)-1]
	}

	heartbeatMS := opts.HeartbeatIntervalMS
	if heartbeatMS <= 0 {
		heartbeatMS = DefaultHeartbeatIntervalMS
	}

	getAuth := opts.GetAuthHeaders
	if getAuth == nil {
		getAuth = func() map[string]string { return nil }
	}

	onEpoch := opts.OnEpochMismatch
	if onEpoch == nil {
		onEpoch = func() {} // caller checks EpochMismatchError
	}

	rc := retryablehttp.NewClient()
	rc.RetryMax = 0 // we manage retries ourselves
	rc.Logger = nil  // silence default logger
	rc.CheckRetry = func(_ context.Context, _ *http.Response, _ error) (bool, error) {
		return false, nil // never retry — we handle retry logic ourselves
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &CCRClient{
		sessionBaseURL:          baseURL,
		sessionID:               sessionID,
		heartbeatIntervalMS:     heartbeatMS,
		heartbeatJitterFraction: opts.HeartbeatJitterFraction,
		onEpochMismatch:         onEpoch,
		getAuthHeaders:          getAuth,
		userAgent:               opts.UserAgent,
		streamAccumulator:       NewStreamAccumulator(),
		httpClient:              rc,
		ctx:                     ctx,
		cancel:                  cancel,
	}, nil
}

// Initialize performs the CCR init handshake:
// 1. Validates auth headers and epoch
// 2. Reports state as idle
// 3. Starts heartbeat timer
func (c *CCRClient) Initialize(epoch int) error {
	headers := c.getAuthHeaders()
	if len(headers) == 0 {
		return &CCRInitError{Reason: InitFailNoAuthHeaders}
	}
	if epoch <= 0 {
		return &CCRInitError{Reason: InitFailMissingEpoch}
	}

	c.mu.Lock()
	c.workerEpoch = epoch
	c.mu.Unlock()

	result := c.request("PUT", "/worker", map[string]any{
		"worker_status": "idle",
		"worker_epoch":  epoch,
		"external_metadata": map[string]any{
			"pending_action": nil,
			"task_summary":   nil,
		},
	}, "PUT worker (init)", 10*time.Second)

	if !result.OK {
		return &CCRInitError{Reason: InitFailWorkerRegisterFailed}
	}

	c.mu.Lock()
	c.currentState = SessionStateIdle
	c.mu.Unlock()

	c.startHeartbeat()
	return nil
}

// ---------------------------------------------------------------------------
// Event writing
// ---------------------------------------------------------------------------

// WriteEvent posts a StdoutMessage as a client event. Stream events are held
// in a 100ms buffer for coalescing; other events flush the buffer first.
func (c *CCRClient) WriteEvent(message map[string]any) {
	msgType, _ := message["type"].(string)

	if msgType == "stream_event" {
		c.mu.Lock()
		c.streamEventBuffer = append(c.streamEventBuffer, message)
		if c.streamEventTimer == nil {
			c.streamEventTimer = time.AfterFunc(
				time.Duration(StreamEventFlushIntervalMS)*time.Millisecond,
				func() { c.flushStreamEventBuffer() },
			)
		}
		c.mu.Unlock()
		return
	}

	// Non-stream event: flush stream buffer first to preserve ordering.
	c.flushStreamEventBuffer()

	if msgType == "assistant" {
		c.clearAccumulatorForAssistant(message)
	}

	c.mu.Lock()
	c.pendingEvents = append(c.pendingEvents, c.toClientEvent(message))
	c.mu.Unlock()
}

// WriteInternalEvent posts a worker-internal event (transcript, compaction).
func (c *CCRClient) WriteInternalEvent(eventType string, payload map[string]any, isCompaction bool, agentID string) {
	p := make(EventPayload, len(payload)+2)
	for k, v := range payload {
		p[k] = v
	}
	p["type"] = eventType
	if _, ok := p["uuid"]; !ok {
		p["uuid"] = uuid.New().String()
	}

	evt := WorkerEvent{Payload: p}
	if isCompaction {
		evt.IsCompaction = true
	}
	if agentID != "" {
		evt.AgentID = agentID
	}

	c.mu.Lock()
	c.pendingInternalEvents = append(c.pendingInternalEvents, evt)
	c.mu.Unlock()
}

// toClientEvent wraps a message map as a ClientEvent, injecting UUID if absent.
func (c *CCRClient) toClientEvent(message map[string]any) ClientEvent {
	payload := make(EventPayload, len(message)+1)
	for k, v := range message {
		payload[k] = v
	}
	if _, ok := payload["uuid"]; !ok {
		payload["uuid"] = uuid.New().String()
	}
	return ClientEvent{Payload: payload}
}

// clearAccumulatorForAssistant clears stream accumulator state for a completed
// assistant message (the reliable end-of-stream signal).
func (c *CCRClient) clearAccumulatorForAssistant(msg map[string]any) {
	sessionID, _ := msg["session_id"].(string)
	parentToolUseID, _ := msg["parent_tool_use_id"].(string)
	msgObj, ok := msg["message"].(map[string]any)
	if !ok {
		return
	}
	msgID, _ := msgObj["id"].(string)
	if msgID == "" {
		return
	}
	ClearStreamAccumulatorForMessage(c.streamAccumulator, sessionID, parentToolUseID, msgID)
}

// ---------------------------------------------------------------------------
// Flush
// ---------------------------------------------------------------------------

// Flush drains all pending event queues. Call before Close if delivery matters.
func (c *CCRClient) Flush() {
	c.flushStreamEventBuffer()
	c.flushPendingEvents()
	c.flushPendingInternalEvents()
	c.flushPendingDeliveries()
}

// FlushInternalEvents drains only the internal event queue.
func (c *CCRClient) FlushInternalEvents() {
	c.flushPendingInternalEvents()
}

func (c *CCRClient) flushStreamEventBuffer() {
	c.mu.Lock()
	if c.streamEventTimer != nil {
		c.streamEventTimer.Stop()
		c.streamEventTimer = nil
	}
	buf := c.streamEventBuffer
	c.streamEventBuffer = nil
	c.mu.Unlock()

	if len(buf) == 0 {
		return
	}

	payloads := AccumulateStreamEvents(buf, c.streamAccumulator)
	events := make([]ClientEvent, len(payloads))
	for i, p := range payloads {
		events[i] = ClientEvent{Payload: p, Ephemeral: true}
	}

	c.mu.Lock()
	c.pendingEvents = append(c.pendingEvents, events...)
	c.mu.Unlock()

	c.flushPendingEvents()
}

func (c *CCRClient) flushPendingEvents() {
	c.mu.Lock()
	batch := c.pendingEvents
	c.pendingEvents = nil
	epoch := c.workerEpoch
	c.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	c.request("POST", "/worker/events", map[string]any{
		"worker_epoch": epoch,
		"events":       batch,
	}, "client events", 10*time.Second)
}

func (c *CCRClient) flushPendingInternalEvents() {
	c.mu.Lock()
	batch := c.pendingInternalEvents
	c.pendingInternalEvents = nil
	epoch := c.workerEpoch
	c.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	c.request("POST", "/worker/internal-events", map[string]any{
		"worker_epoch": epoch,
		"events":       batch,
	}, "internal events", 10*time.Second)
}

func (c *CCRClient) flushPendingDeliveries() {
	c.mu.Lock()
	batch := c.pendingDeliveries
	c.pendingDeliveries = nil
	epoch := c.workerEpoch
	c.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	c.request("POST", "/worker/events/delivery", map[string]any{
		"worker_epoch": epoch,
		"updates":      batch,
	}, "delivery batch", 10*time.Second)
}

// ---------------------------------------------------------------------------
// Read internal events
// ---------------------------------------------------------------------------

// ReadInternalEvents fetches foreground agent internal events (for resume).
func (c *CCRClient) ReadInternalEvents() ([]InternalEvent, error) {
	return c.paginatedGet("/worker/internal-events", nil, "internal_events")
}

// ReadSubagentInternalEvents fetches all subagent internal events.
func (c *CCRClient) ReadSubagentInternalEvents() ([]InternalEvent, error) {
	return c.paginatedGet("/worker/internal-events", map[string]string{"subagents": "true"}, "subagent_events")
}

func (c *CCRClient) paginatedGet(path string, params map[string]string, label string) ([]InternalEvent, error) {
	headers := c.getAuthHeaders()
	if len(headers) == 0 {
		return nil, nil
	}

	var all []InternalEvent
	cursor := ""

	for {
		u, err := url.Parse(c.sessionBaseURL + path)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		u.RawQuery = q.Encode()

		data, err := c.getWithRetry(u.String(), headers, label)
		if err != nil {
			return nil, err
		}
		if data == nil {
			return nil, nil
		}

		all = append(all, data.Data...)
		cursor = data.NextCursor
		if cursor == "" {
			break
		}
	}

	return all, nil
}

func (c *CCRClient) getWithRetry(rawURL string, authHeaders map[string]string, label string) (*listInternalEventsResponse, error) {
	const maxAttempts = 10

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := retryablehttp.NewRequest("GET", rawURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range authHeaders {
			req.Header.Set(k, v)
		}
		req.Header.Set("anthropic-version", "2023-06-01")
		if c.userAgent != "" {
			req.Header.Set("User-Agent", c.userAgent)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < maxAttempts {
				delay := min(500*time.Millisecond*(1<<(attempt-1)), 30*time.Second)
				delay += time.Duration(rand.Float64()*500) * time.Millisecond
				time.Sleep(delay)
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var result listInternalEventsResponse
			if err := json.Unmarshal(body, &result); err != nil {
				return nil, fmt.Errorf("CCRClient: %s JSON decode: %w", label, err)
			}
			return &result, nil
		}
		if resp.StatusCode == 409 {
			c.handleEpochMismatch()
			return nil, &EpochMismatchError{}
		}

		if attempt < maxAttempts {
			delay := min(500*time.Millisecond*(1<<(attempt-1)), 30*time.Second)
			delay += time.Duration(rand.Float64()*500) * time.Millisecond
			time.Sleep(delay)
		}
	}

	return nil, nil
}

// ---------------------------------------------------------------------------
// Report state / metadata / delivery
// ---------------------------------------------------------------------------

// ReportState sends the worker status via PUT /worker.
func (c *CCRClient) ReportState(state SessionState, details *RequiresActionDetails) {
	c.mu.Lock()
	if state == c.currentState && details == nil {
		c.mu.Unlock()
		return
	}
	c.currentState = state
	epoch := c.workerEpoch
	c.mu.Unlock()

	body := map[string]any{
		"worker_status": string(state),
		"worker_epoch":  epoch,
	}
	if details != nil {
		body["requires_action_details"] = map[string]any{
			"tool_name":          details.ToolName,
			"action_description": details.ActionDescription,
			"request_id":         details.RequestID,
		}
	} else {
		body["requires_action_details"] = nil
	}

	c.request("PUT", "/worker", body, "PUT worker", 10*time.Second)
}

// ReportMetadata sends external metadata via PUT /worker.
func (c *CCRClient) ReportMetadata(metadata map[string]any) {
	c.mu.Lock()
	epoch := c.workerEpoch
	c.mu.Unlock()

	c.request("PUT", "/worker", map[string]any{
		"worker_epoch":      epoch,
		"external_metadata": metadata,
	}, "PUT metadata", 10*time.Second)
}

// ReportDelivery enqueues a delivery ACK for batch posting.
func (c *CCRClient) ReportDelivery(eventID string, status DeliveryStatus) {
	c.mu.Lock()
	c.pendingDeliveries = append(c.pendingDeliveries, deliveryItem{
		EventID: eventID,
		Status:  status,
	})
	c.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Heartbeat
// ---------------------------------------------------------------------------

func (c *CCRClient) startHeartbeat() {
	c.stopHeartbeat()

	c.mu.Lock()
	c.scheduleHeartbeatLocked()
	c.mu.Unlock()
}

// scheduleHeartbeatLocked sets the next heartbeat timer. Caller must hold mu.
func (c *CCRClient) scheduleHeartbeatLocked() {
	interval := time.Duration(c.heartbeatIntervalMS) * time.Millisecond
	jitter := time.Duration(
		float64(interval) * c.heartbeatJitterFraction * (2*rand.Float64() - 1),
	)
	c.heartbeatTimer = time.AfterFunc(interval+jitter, func() {
		c.sendHeartbeat()

		c.mu.Lock()
		defer c.mu.Unlock()
		if c.heartbeatTimer == nil || c.closed {
			return
		}
		c.scheduleHeartbeatLocked()
	})
}

func (c *CCRClient) stopHeartbeat() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.heartbeatTimer != nil {
		c.heartbeatTimer.Stop()
		c.heartbeatTimer = nil
	}
}

func (c *CCRClient) sendHeartbeat() {
	c.mu.Lock()
	if c.heartbeatInFlight {
		c.mu.Unlock()
		return
	}
	c.heartbeatInFlight = true
	epoch := c.workerEpoch
	sid := c.sessionID
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.heartbeatInFlight = false
		c.mu.Unlock()
	}()

	c.request("POST", "/worker/heartbeat", map[string]any{
		"session_id":   sid,
		"worker_epoch": epoch,
	}, "Heartbeat", 5*time.Second)
}

// ---------------------------------------------------------------------------
// HTTP request helper
// ---------------------------------------------------------------------------

// request sends an authenticated HTTP request to CCR. Handles auth headers,
// 409 epoch mismatch, 401/403 auth failure counting, and 429 Retry-After.
func (c *CCRClient) request(method, path string, body any, label string, timeout time.Duration) RequestResult {
	headers := c.getAuthHeaders()
	if len(headers) == 0 {
		return RequestResult{OK: false}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return RequestResult{OK: false}
	}

	req, err := retryablehttp.NewRequest(method, c.sessionBaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return RequestResult{OK: false}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	// Use a context with timeout.
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RequestResult{OK: false}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.mu.Lock()
		c.consecutiveAuthFailures = 0
		c.mu.Unlock()
		return RequestResult{OK: true}
	}

	if resp.StatusCode == 409 {
		c.handleEpochMismatch()
		// Return not-OK; the epoch mismatch callback fires above.
		return RequestResult{OK: false}
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		c.mu.Lock()
		c.consecutiveAuthFailures++
		failures := c.consecutiveAuthFailures
		c.mu.Unlock()

		if failures >= MaxConsecutiveAuthFailures {
			c.onEpochMismatch()
			return RequestResult{OK: false}
		}
	}

	if resp.StatusCode == 429 {
		raw := resp.Header.Get("Retry-After")
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
			return RequestResult{OK: false, RetryAfterMS: seconds * 1000}
		}
	}

	return RequestResult{OK: false}
}

// handleEpochMismatch handles 409 Conflict — a newer worker epoch superseded us.
func (c *CCRClient) handleEpochMismatch() {
	c.onEpochMismatch()
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------

// WorkerEpoch returns the current epoch.
func (c *CCRClient) WorkerEpoch() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.workerEpoch
}

// PendingEventCount returns the number of queued client events.
func (c *CCRClient) PendingEventCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pendingEvents)
}

// PendingInternalEventCount returns the number of queued internal events.
func (c *CCRClient) PendingInternalEventCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pendingInternalEvents)
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

// Close shuts down the client: stops heartbeat, cancels in-flight requests,
// and drops pending buffers. Call Flush first if delivery matters.
func (c *CCRClient) Close() {
	c.mu.Lock()
	c.closed = true
	if c.streamEventTimer != nil {
		c.streamEventTimer.Stop()
		c.streamEventTimer = nil
	}
	c.streamEventBuffer = nil
	c.pendingEvents = nil
	c.pendingInternalEvents = nil
	c.pendingDeliveries = nil
	c.mu.Unlock()

	c.stopHeartbeat()
	c.cancel()

	// Clear accumulator state.
	c.streamAccumulator.mu.Lock()
	c.streamAccumulator.ByMessage = make(map[string][][]string)
	c.streamAccumulator.ScopeToMessage = make(map[string]string)
	c.streamAccumulator.mu.Unlock()
}
