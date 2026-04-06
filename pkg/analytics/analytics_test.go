package analytics

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// --- helpers ---

// spySink records all events sent to it for assertion.
type spySink struct {
	mu     sync.Mutex
	events []spyEvent
}
type spyEvent struct {
	name     string
	metadata EventMetadata
	async    bool
}

func (s *spySink) LogEvent(name string, m EventMetadata)      { s.record(name, m, false) }
func (s *spySink) LogEventAsync(name string, m EventMetadata)  { s.record(name, m, true) }
func (s *spySink) Shutdown()                                   {}
func (s *spySink) record(name string, m EventMetadata, a bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, spyEvent{name, m, a})
}
func (s *spySink) Events() []spyEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]spyEvent, len(s.events))
	copy(out, s.events)
	return out
}

// --- E2E: emit event + flush + verify buffered ---

func TestLogEvent_QueuedBeforeSinkAttach(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	// Log events before any sink is attached.
	LogEvent("tengu_init", EventMetadata{"version": 1})
	LogEvent("tengu_started", EventMetadata{"model": "claude-3"})
	LogEventAsync("tengu_exit", EventMetadata{"code": 0})

	// Verify nothing was lost: events must be queued.
	mu.Lock()
	qLen := len(eventQueue)
	mu.Unlock()
	if qLen != 3 {
		t.Fatalf("expected 3 queued events, got %d", qLen)
	}

	// Attach a spy sink — queued events should drain.
	spy := &spySink{}
	AttachSink(spy)

	events := spy.Events()
	if len(events) != 3 {
		t.Fatalf("expected 3 drained events, got %d", len(events))
	}
	if events[0].name != "tengu_init" {
		t.Errorf("first event = %q, want %q", events[0].name, "tengu_init")
	}
	if events[1].name != "tengu_started" {
		t.Errorf("second event = %q, want %q", events[1].name, "tengu_started")
	}
	if events[2].name != "tengu_exit" || !events[2].async {
		t.Errorf("third event = %q async=%v, want tengu_exit async=true", events[2].name, events[2].async)
	}

	// Subsequent events go directly to the sink.
	LogEvent("tengu_api_success", nil)
	events = spy.Events()
	if len(events) != 4 {
		t.Fatalf("expected 4 events after post-attach log, got %d", len(events))
	}
}

func TestAttachSink_Idempotent(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	spy1 := &spySink{}
	spy2 := &spySink{}

	AttachSink(spy1)
	AttachSink(spy2) // no-op

	LogEvent("test_event", nil)

	if len(spy1.Events()) != 1 {
		t.Error("spy1 should have received the event")
	}
	if len(spy2.Events()) != 0 {
		t.Error("spy2 should not have received any events (attach was no-op)")
	}
}

// --- E2E: opt-out disables emission ---

func TestIsAnalyticsDisabled_OptOut(t *testing.T) {
	// Telemetry disabled via env var.
	t.Setenv("CLAUDE_CODE_DISABLE_TELEMETRY", "1")
	if !IsAnalyticsDisabled() {
		t.Error("expected analytics to be disabled when CLAUDE_CODE_DISABLE_TELEMETRY=1")
	}
}

func TestIsAnalyticsDisabled_Bedrock(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "true")
	t.Setenv("CLAUDE_CODE_DISABLE_TELEMETRY", "")
	t.Setenv("GO_TEST", "")
	t.Setenv("NODE_ENV", "")
	if !IsAnalyticsDisabled() {
		t.Error("expected analytics to be disabled for Bedrock")
	}
}

func TestIsAnalyticsDisabled_Vertex(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "yes")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
	t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "")
	t.Setenv("CLAUDE_CODE_DISABLE_TELEMETRY", "")
	t.Setenv("GO_TEST", "")
	t.Setenv("NODE_ENV", "")
	if !IsAnalyticsDisabled() {
		t.Error("expected analytics to be disabled for Vertex")
	}
}

func TestIsAnalyticsDisabled_Foundry(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_FOUNDRY", "1")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "")
	t.Setenv("CLAUDE_CODE_USE_VERTEX", "")
	t.Setenv("CLAUDE_CODE_DISABLE_TELEMETRY", "")
	t.Setenv("GO_TEST", "")
	t.Setenv("NODE_ENV", "")
	if !IsAnalyticsDisabled() {
		t.Error("expected analytics to be disabled for Foundry")
	}
}

func TestIsFeedbackSurveyDisabled_AllowsThirdParty(t *testing.T) {
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "true")
	t.Setenv("CLAUDE_CODE_DISABLE_TELEMETRY", "")
	t.Setenv("GO_TEST", "")
	t.Setenv("NODE_ENV", "")
	if IsFeedbackSurveyDisabled() {
		t.Error("feedback survey should NOT be disabled for Bedrock (3P providers are allowed)")
	}
}

// --- StripProtoFields ---

func TestStripProtoFields(t *testing.T) {
	m := EventMetadata{
		"version":           "1.0",
		"_PROTO_user_email": "test@example.com",
		"_PROTO_raw_input":  "secret",
		"model":             "claude-3",
	}
	stripped := StripProtoFields(m)
	if _, ok := stripped["_PROTO_user_email"]; ok {
		t.Error("_PROTO_user_email should have been stripped")
	}
	if _, ok := stripped["_PROTO_raw_input"]; ok {
		t.Error("_PROTO_raw_input should have been stripped")
	}
	if stripped["version"] != "1.0" {
		t.Error("non-proto field 'version' should be preserved")
	}
	if stripped["model"] != "claude-3" {
		t.Error("non-proto field 'model' should be preserved")
	}
}

func TestStripProtoFields_NoProtoKeysReturnsSameRef(t *testing.T) {
	m := EventMetadata{"a": 1, "b": 2}
	stripped := StripProtoFields(m)
	// Should return the same map (no copy needed).
	m["c"] = 3
	if stripped["c"] != 3 {
		t.Error("expected same reference when no _PROTO_ keys present")
	}
}

// --- Metadata ---

func TestSanitizeToolNameForAnalytics(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Bash", "Bash"},
		{"Read", "Read"},
		{"mcp__slack__read_channel", "mcp_tool"},
		{"mcp__custom_server__do_thing", "mcp_tool"},
		{"FileEdit", "FileEdit"},
	}
	for _, tt := range tests {
		got := SanitizeToolNameForAnalytics(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeToolNameForAnalytics(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractMCPToolDetails(t *testing.T) {
	tests := []struct {
		input      string
		wantServer string
		wantTool   string
	}{
		{"mcp__slack__read_channel", "slack", "read_channel"},
		{"mcp__github__create_pr", "github", "create_pr"},
		{"mcp__srv__tool__with__underscores", "srv", "tool__with__underscores"},
		{"Bash", "", ""},
		{"mcp__", "", ""},
		{"mcp__server__", "", ""},
	}
	for _, tt := range tests {
		server, tool := ExtractMCPToolDetails(tt.input)
		if server != tt.wantServer || tool != tt.wantTool {
			t.Errorf("ExtractMCPToolDetails(%q) = (%q, %q), want (%q, %q)",
				tt.input, server, tool, tt.wantServer, tt.wantTool)
		}
	}
}

func TestGetFileExtensionForAnalytics(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/foo/bar.go", "go"},
		{"/foo/bar.TS", "ts"},
		{"/foo/bar", ""},
		{"/foo/.gitignore", "gitignore"},
		{"/foo/bar.verylongextensionname", "other"},
		{"", ""},
	}
	for _, tt := range tests {
		got := GetFileExtensionForAnalytics(tt.input)
		if got != tt.want {
			t.Errorf("GetFileExtensionForAnalytics(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Datadog sink batching ---

func TestDatadogSink_BatchAndFlush(t *testing.T) {
	var (
		receivedMu sync.Mutex
		received   [][]DatadogLog
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var logs []DatadogLog
		if err := json.Unmarshal(body, &logs); err != nil {
			t.Errorf("bad JSON: %v", err)
			w.WriteHeader(400)
			return
		}
		receivedMu.Lock()
		received = append(received, logs)
		receivedMu.Unlock()
		w.WriteHeader(202)
	}))
	defer srv.Close()

	dd := NewDatadogSink(
		WithDatadogEndpoint(srv.URL),
		WithDatadogFlushInterval(50*time.Millisecond),
	)

	// Enqueue a few events.
	dd.Enqueue(DatadogLog{Message: "ev1", Service: "test"})
	dd.Enqueue(DatadogLog{Message: "ev2", Service: "test"})

	if dd.PendingCount() != 2 {
		t.Fatalf("pending = %d, want 2", dd.PendingCount())
	}

	// Wait for the flush timer.
	time.Sleep(200 * time.Millisecond)

	receivedMu.Lock()
	n := len(received)
	receivedMu.Unlock()

	if n == 0 {
		t.Fatal("expected at least one flush to the server")
	}

	receivedMu.Lock()
	total := 0
	for _, batch := range received {
		total += len(batch)
	}
	receivedMu.Unlock()
	if total != 2 {
		t.Errorf("received %d total events, want 2", total)
	}
}

func TestDatadogSink_FlushOnBatchFull(t *testing.T) {
	flushCount := 0
	var flushMu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flushMu.Lock()
		flushCount++
		flushMu.Unlock()
		w.WriteHeader(202)
	}))
	defer srv.Close()

	dd := NewDatadogSink(
		WithDatadogEndpoint(srv.URL),
		WithDatadogFlushInterval(1*time.Hour), // very long — should not trigger
	)

	// Enqueue MaxBatchSize events to trigger immediate flush.
	for i := 0; i < MaxBatchSize; i++ {
		dd.Enqueue(DatadogLog{Message: "event", Service: "test"})
	}

	// Wait briefly for the background goroutine.
	time.Sleep(200 * time.Millisecond)

	flushMu.Lock()
	c := flushCount
	flushMu.Unlock()

	if c == 0 {
		t.Error("expected immediate flush when batch hits max size")
	}
}

// --- Killswitch ---

func TestIsSinkKilled_DefaultFalse(t *testing.T) {
	SetKillswitchProvider(nil)
	if IsSinkKilled(SinkDatadog) {
		t.Error("expected false when no killswitch provider is set")
	}
}

func TestIsSinkKilled_RemoteKill(t *testing.T) {
	SetKillswitchProvider(func() map[SinkName]bool {
		return map[SinkName]bool{SinkDatadog: true, SinkFirstParty: false}
	})
	defer SetKillswitchProvider(nil)

	if !IsSinkKilled(SinkDatadog) {
		t.Error("expected datadog to be killed")
	}
	if IsSinkKilled(SinkFirstParty) {
		t.Error("expected firstParty to be alive")
	}
}

// --- CompositeSink with Datadog ---

func TestCompositeSink_DatadogAllowlistAndGate(t *testing.T) {
	var received []DatadogLog
	var recMu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var logs []DatadogLog
		json.Unmarshal(body, &logs)
		recMu.Lock()
		received = append(received, logs...)
		recMu.Unlock()
		w.WriteHeader(202)
	}))
	defer srv.Close()

	dd := NewDatadogSink(
		WithDatadogEndpoint(srv.URL),
		WithDatadogFlushInterval(50*time.Millisecond),
	)

	comp := NewCompositeSink(CompositeSinkConfig{
		Datadog:     dd,
		GateChecker: func(name string) bool { return name == DatadogGateName },
	})

	// Allowed event.
	comp.LogEvent("tengu_init", EventMetadata{"version": 1})
	// Not-allowed event — should be dropped.
	comp.LogEvent("not_in_allowlist", EventMetadata{"x": 1})

	time.Sleep(200 * time.Millisecond)

	recMu.Lock()
	n := len(received)
	recMu.Unlock()

	if n != 1 {
		t.Fatalf("expected 1 event sent to Datadog, got %d", n)
	}
	if received[0].Message != "tengu_init" {
		t.Errorf("event message = %q, want %q", received[0].Message, "tengu_init")
	}
}

func TestCompositeSink_DatadogDisabledByGate(t *testing.T) {
	dd := NewDatadogSink(WithDatadogFlushInterval(50 * time.Millisecond))
	comp := NewCompositeSink(CompositeSinkConfig{
		Datadog:     dd,
		GateChecker: func(string) bool { return false }, // gate OFF
	})

	comp.LogEvent("tengu_init", EventMetadata{"version": 1})
	time.Sleep(100 * time.Millisecond)

	if dd.PendingCount() != 0 {
		t.Error("no events should be buffered when gate is off")
	}
}

// --- Session metadata ---

func TestSessionInfo_RoundTrip(t *testing.T) {
	info := SessionInfo{
		SessionID:     "sess-123",
		Model:         "claude-3-opus",
		Version:       "2.0.1",
		UserType:      "ant",
		ClientType:    "cli",
		IsInteractive: true,
	}
	SetSessionInfo(info)
	got := GetSessionInfo()
	if got.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "sess-123")
	}
	if got.Model != "claude-3-opus" {
		t.Errorf("Model = %q, want %q", got.Model, "claude-3-opus")
	}
}

func TestGetEventMetadata_IncludesCoreFields(t *testing.T) {
	SetSessionInfo(SessionInfo{
		SessionID: "test-session",
		Model:     "claude-3",
		UserType:  "external",
	})
	m := GetEventMetadata()
	if m["sessionId"] != "test-session" {
		t.Errorf("sessionId = %v, want %q", m["sessionId"], "test-session")
	}
	if m["model"] != "claude-3" {
		t.Errorf("model = %v, want %q", m["model"], "claude-3")
	}
	if m["platform"] == nil || m["platform"] == "" {
		t.Error("platform should not be empty")
	}
}

// --- Sampling ---

func TestCompositeSink_SamplingDropsEvent(t *testing.T) {
	dd := NewDatadogSink(WithDatadogFlushInterval(50 * time.Millisecond))
	comp := NewCompositeSink(CompositeSinkConfig{
		Datadog:     dd,
		GateChecker: func(string) bool { return true },
		SamplingConfig: func() map[string]EventSamplingEntry {
			return map[string]EventSamplingEntry{
				"tengu_api_success": {SampleRate: 0}, // 0% — always drop
			}
		},
	})

	comp.LogEvent("tengu_api_success", EventMetadata{"status": 200})
	time.Sleep(100 * time.Millisecond)

	if dd.PendingCount() != 0 {
		t.Error("event with sample_rate=0 should have been dropped")
	}
}

func TestCompositeSink_SamplingPassesUnconfiguredEvent(t *testing.T) {
	var received []DatadogLog
	var recMu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var logs []DatadogLog
		json.Unmarshal(body, &logs)
		recMu.Lock()
		received = append(received, logs...)
		recMu.Unlock()
		w.WriteHeader(202)
	}))
	defer srv.Close()

	dd := NewDatadogSink(
		WithDatadogEndpoint(srv.URL),
		WithDatadogFlushInterval(50*time.Millisecond),
	)
	comp := NewCompositeSink(CompositeSinkConfig{
		Datadog:     dd,
		GateChecker: func(string) bool { return true },
		SamplingConfig: func() map[string]EventSamplingEntry {
			return map[string]EventSamplingEntry{} // empty → no sampling
		},
	})

	comp.LogEvent("tengu_init", EventMetadata{"x": 1})
	time.Sleep(200 * time.Millisecond)

	recMu.Lock()
	n := len(received)
	recMu.Unlock()

	if n != 1 {
		t.Fatalf("unconfigured event should pass through, got %d", n)
	}
}
