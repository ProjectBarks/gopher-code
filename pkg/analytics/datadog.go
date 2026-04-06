package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Datadog HTTP logs API constants.
const (
	DatadogLogsEndpoint = "https://http-intake.logs.us5.datadoghq.com/api/v2/logs"
	DatadogClientToken  = "pubbbf48e6d78dae54bceaa4acf463299bf"
	DefaultFlushInterval = 15 * time.Second
	MaxBatchSize         = 100
	NetworkTimeout       = 5 * time.Second
)

// DatadogAllowedEvents is the allowlist of event names that may be sent to
// Datadog. Events not in this set are silently dropped to prevent PII leaks.
var DatadogAllowedEvents = map[string]bool{
	"chrome_bridge_connection_succeeded":       true,
	"chrome_bridge_connection_failed":          true,
	"chrome_bridge_disconnected":               true,
	"chrome_bridge_tool_call_completed":        true,
	"chrome_bridge_tool_call_error":            true,
	"chrome_bridge_tool_call_started":          true,
	"chrome_bridge_tool_call_timeout":          true,
	"tengu_api_error":                          true,
	"tengu_api_success":                        true,
	"tengu_brief_mode_enabled":                 true,
	"tengu_brief_mode_toggled":                 true,
	"tengu_brief_send":                         true,
	"tengu_cancel":                             true,
	"tengu_compact_failed":                     true,
	"tengu_exit":                               true,
	"tengu_flicker":                            true,
	"tengu_init":                               true,
	"tengu_model_fallback_triggered":           true,
	"tengu_oauth_error":                        true,
	"tengu_oauth_success":                      true,
	"tengu_oauth_token_refresh_failure":        true,
	"tengu_oauth_token_refresh_success":        true,
	"tengu_oauth_token_refresh_lock_acquiring": true,
	"tengu_oauth_token_refresh_lock_acquired":  true,
	"tengu_oauth_token_refresh_starting":       true,
	"tengu_oauth_token_refresh_completed":      true,
	"tengu_oauth_token_refresh_lock_releasing": true,
	"tengu_oauth_token_refresh_lock_released":  true,
	"tengu_query_error":                        true,
	"tengu_session_file_read":                  true,
	"tengu_started":                            true,
	"tengu_tool_use_error":                     true,
	"tengu_tool_use_granted_in_prompt_permanent":  true,
	"tengu_tool_use_granted_in_prompt_temporary":  true,
	"tengu_tool_use_rejected_in_prompt":           true,
	"tengu_tool_use_success":                      true,
	"tengu_uncaught_exception":                    true,
	"tengu_unhandled_rejection":                   true,
	"tengu_voice_recording_started":               true,
	"tengu_voice_toggled":                         true,
	"tengu_team_mem_sync_pull":                    true,
	"tengu_team_mem_sync_push":                    true,
	"tengu_team_mem_sync_started":                 true,
	"tengu_team_mem_entries_capped":               true,
}

// DatadogLog is the payload shape for the Datadog HTTP Logs API.
type DatadogLog struct {
	DDSource string `json:"ddsource"`
	DDTags   string `json:"ddtags"`
	Message  string `json:"message"`
	Service  string `json:"service"`
	Hostname string `json:"hostname"`
	// Extra fields are added as top-level keys during serialization.
	Extra map[string]any `json:"-"`
}

// MarshalJSON flattens Extra into the top-level JSON object.
func (d DatadogLog) MarshalJSON() ([]byte, error) {
	type plain DatadogLog
	b, err := json.Marshal(plain(d))
	if err != nil {
		return nil, err
	}
	if len(d.Extra) == 0 {
		return b, nil
	}
	// Merge extra into the object.
	var base map[string]any
	if err := json.Unmarshal(b, &base); err != nil {
		return nil, err
	}
	for k, v := range d.Extra {
		base[k] = v
	}
	return json.Marshal(base)
}

// DatadogSink buffers events and flushes them to Datadog's HTTP Logs API.
type DatadogSink struct {
	mu        sync.Mutex
	batch     []DatadogLog
	client    *http.Client
	endpoint  string
	token     string
	flushInt  time.Duration
	timer     *time.Timer
	closed    bool
}

// DatadogOption configures the Datadog sink.
type DatadogOption func(*DatadogSink)

// WithDatadogEndpoint overrides the Datadog logs endpoint (for testing).
func WithDatadogEndpoint(endpoint string) DatadogOption {
	return func(d *DatadogSink) { d.endpoint = endpoint }
}

// WithDatadogFlushInterval overrides the flush interval.
func WithDatadogFlushInterval(interval time.Duration) DatadogOption {
	return func(d *DatadogSink) { d.flushInt = interval }
}

// WithDatadogHTTPClient overrides the HTTP client.
func WithDatadogHTTPClient(c *http.Client) DatadogOption {
	return func(d *DatadogSink) { d.client = c }
}

// NewDatadogSink creates a sink that batches and sends events to Datadog.
func NewDatadogSink(opts ...DatadogOption) *DatadogSink {
	d := &DatadogSink{
		endpoint: DatadogLogsEndpoint,
		token:    DatadogClientToken,
		flushInt: DefaultFlushInterval,
		client:   &http.Client{Timeout: NetworkTimeout},
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Enqueue adds a log entry to the batch. If the batch is full, it flushes
// immediately. Otherwise it schedules a flush after the flush interval.
func (d *DatadogSink) Enqueue(log DatadogLog) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}

	d.batch = append(d.batch, log)

	if len(d.batch) >= MaxBatchSize {
		d.flushLocked()
	} else {
		d.scheduleFlushLocked()
	}
}

// Flush sends all buffered logs to Datadog.
func (d *DatadogSink) Flush() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.flushLocked()
}

func (d *DatadogSink) flushLocked() {
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	if len(d.batch) == 0 {
		return
	}
	logs := d.batch
	d.batch = nil

	// Fire-and-forget: telemetry failures must not block the CLI.
	go d.send(logs)
}

func (d *DatadogSink) scheduleFlushLocked() {
	if d.timer != nil {
		return
	}
	d.timer = time.AfterFunc(d.flushInt, func() {
		d.mu.Lock()
		d.timer = nil
		d.flushLocked()
		d.mu.Unlock()
	})
}

func (d *DatadogSink) send(logs []DatadogLog) {
	body, err := json.Marshal(logs)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), NetworkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", d.token)

	resp, err := d.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// Close flushes remaining events and marks the sink as closed.
func (d *DatadogSink) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.flushLocked()
}

// PendingCount returns the number of buffered events (for testing).
func (d *DatadogSink) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.batch)
}
