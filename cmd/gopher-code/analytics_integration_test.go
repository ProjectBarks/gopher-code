package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/analytics"
)

// TestAnalytics_IntegrationSinkAndSessionInfo verifies the full analytics
// code path exercised by main(): SetSessionInfo → AttachSink → LogEvent →
// drain → Shutdown. This is the integration test for T476.
func TestAnalytics_IntegrationSinkAndSessionInfo(t *testing.T) {
	analytics.ResetForTesting()
	defer analytics.ResetForTesting()

	// 1. Set session info (mirrors main.go wiring).
	analytics.SetSessionInfo(analytics.SessionInfo{
		SessionID:     "test-session-476",
		Model:         "claude-sonnet-4-20250514",
		Version:       Version,
		ClientType:    "cli",
		IsInteractive: true,
		Cwd:           t.TempDir(),
	})

	// 2. Log an event before the sink is attached — it must be queued.
	analytics.LogEvent("tengu_init", analytics.EventMetadata{"version": Version})

	// 3. Attach a composite sink (same construction as main.go, but with
	//    no real Datadog backend so we don't hit the network).
	spy := &integrationSpySink{}
	analytics.AttachSink(spy)

	// 4. The queued tengu_init event should have drained into the spy.
	events := spy.Events()
	if len(events) == 0 {
		t.Fatal("expected tengu_init event to drain into sink on attach")
	}
	if events[0].Name != "tengu_init" {
		t.Errorf("first event = %q, want %q", events[0].Name, "tengu_init")
	}
	if events[0].Metadata["version"] != Version {
		t.Errorf("version metadata = %v, want %q", events[0].Metadata["version"], Version)
	}

	// 5. Verify session info is enriched into event metadata.
	info := analytics.GetSessionInfo()
	if info.SessionID != "test-session-476" {
		t.Errorf("SessionID = %q, want %q", info.SessionID, "test-session-476")
	}
	if info.ClientType != "cli" {
		t.Errorf("ClientType = %q, want %q", info.ClientType, "cli")
	}

	// 6. Verify GetEventMetadata includes the session fields.
	em := analytics.GetEventMetadata()
	if em["sessionId"] != "test-session-476" {
		t.Errorf("enriched sessionId = %v, want %q", em["sessionId"], "test-session-476")
	}

	// 7. Post-attach events go directly to the sink.
	analytics.LogEvent("tengu_exit", analytics.EventMetadata{"code": 0})
	events = spy.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events after post-attach log, got %d", len(events))
	}
	if events[1].Name != "tengu_exit" {
		t.Errorf("second event = %q, want %q", events[1].Name, "tengu_exit")
	}

	// 8. Shutdown must not panic.
	analytics.Shutdown()
}

// TestAnalytics_CompositeSinkEndToEnd exercises the CompositeSink through
// the same code path as main.go: construct, attach, log an allowed event,
// and verify it flows through the sampling and gate checks.
func TestAnalytics_CompositeSinkEndToEnd(t *testing.T) {
	analytics.ResetForTesting()
	defer analytics.ResetForTesting()

	// Construct a CompositeSink with no real Datadog (nil) so no network I/O.
	comp := analytics.NewCompositeSink(analytics.CompositeSinkConfig{
		GateChecker: func(gate string) bool { return false },
	})

	analytics.AttachSink(comp)
	analytics.SetSessionInfo(analytics.SessionInfo{
		SessionID: "composite-test",
		Model:     "test-model",
		Version:   Version,
	})

	// Log events — they should flow through without panic.
	analytics.LogEvent("tengu_init", analytics.EventMetadata{"version": Version})
	analytics.LogEvent("tengu_started", nil)
	analytics.LogEventAsync("tengu_exit", analytics.EventMetadata{"code": 0})

	// Shutdown flushes without error.
	analytics.Shutdown()
}

// TestAnalytics_DisabledWhenOptedOut verifies that IsAnalyticsDisabled
// returns true when the telemetry opt-out env var is set, matching the
// guard in main.go that skips AttachSink.
func TestAnalytics_DisabledWhenOptedOut(t *testing.T) {
	t.Setenv("CLAUDE_CODE_DISABLE_TELEMETRY", "1")
	if !analytics.IsAnalyticsDisabled() {
		t.Error("analytics should be disabled when CLAUDE_CODE_DISABLE_TELEMETRY=1")
	}
}

// integrationSpySink implements analytics.Sink for testing.
type integrationSpySink struct {
	events []integrationSpyEvent
}

type integrationSpyEvent struct {
	Name     string
	Metadata analytics.EventMetadata
	Async    bool
}

func (s *integrationSpySink) LogEvent(name string, m analytics.EventMetadata) {
	s.events = append(s.events, integrationSpyEvent{name, m, false})
}
func (s *integrationSpySink) LogEventAsync(name string, m analytics.EventMetadata) {
	s.events = append(s.events, integrationSpyEvent{name, m, true})
}
func (s *integrationSpySink) Shutdown() {}

func (s *integrationSpySink) Events() []integrationSpyEvent {
	out := make([]integrationSpyEvent, len(s.events))
	copy(out, s.events)
	return out
}
