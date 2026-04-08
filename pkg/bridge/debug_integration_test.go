package bridge

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/analytics"
	apperrors "github.com/projectbarks/gopher-code/pkg/errors"
)

// TestDebugIntegration_LevelsAndBuffer verifies that a BridgeDebug instance
// correctly filters messages at different levels and retains them in the
// circular buffer. This exercises the end-to-end path from typed log methods
// through the ring buffer and slog backend.
func TestDebugIntegration_LevelsAndBuffer(t *testing.T) {
	var buf bytes.Buffer
	d := NewBridgeDebug(LogLevelInfo, 32, testLogger(&buf))

	// Debug-level messages should be filtered out at Info threshold.
	d.LogAPI("GET", "/health", 200)

	// Info-level messages should pass through.
	d.LogStatus("bridge ready", map[string]string{"env": "test"})
	d.LogTransition("idle", "polling", "timer")
	d.LogSession("sess-42", "created")

	// Error-level messages should pass through.
	d.LogError("connection lost", nil)

	entries := d.Entries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries (1 debug filtered), got %d", len(entries))
	}

	// Verify chronological order and levels.
	wantLevels := []LogLevel{LogLevelInfo, LogLevelInfo, LogLevelInfo, LogLevelError}
	for i, e := range entries {
		if e.Level != wantLevels[i] {
			t.Errorf("entry %d: level=%s, want %s", i, e.Level, wantLevels[i])
		}
	}

	// Verify slog backend received the messages.
	slogOut := buf.String()
	if !strings.Contains(slogOut, "[bridge]") {
		t.Error("slog output missing [bridge] prefix")
	}
	if !strings.Contains(slogOut, "bridge ready") {
		t.Error("slog output missing 'bridge ready'")
	}

	// Now change level to Error and verify Info is filtered.
	d.SetLevel(LogLevelError)
	d.LogStatus("should be filtered", nil)
	d.LogError("visible error", nil)

	entries = d.Entries()
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries after level change, got %d", len(entries))
	}
	if entries[4].Message != "visible error" {
		t.Errorf("last entry: got %q, want 'visible error'", entries[4].Message)
	}

	// Change back to Debug — everything should pass.
	d.SetLevel(LogLevelDebug)
	d.LogAPI("POST", "/poll", 200)

	entries = d.Entries()
	if len(entries) != 6 {
		t.Fatalf("expected 6 entries after debug level, got %d", len(entries))
	}
	if !strings.Contains(entries[5].Message, "API POST /poll") {
		t.Errorf("last entry: got %q", entries[5].Message)
	}
}

// TestDebugIntegration_LogErrorWithID verifies that LogErrorWithID attaches a
// numeric error ID from pkg/errors to the log entry attrs, matching the TS
// pattern where err.cause = { errorId: E_TOOL_USE_SUMMARY_GENERATION_FAILED }.
func TestDebugIntegration_LogErrorWithID(t *testing.T) {
	var buf bytes.Buffer
	d := NewBridgeDebug(LogLevelDebug, 32, testLogger(&buf))

	// Log an error with the tool-use summary generation error ID.
	d.LogErrorWithID(
		"summary generation failed",
		errors.New("context deadline exceeded"),
		apperrors.EToolUseSummaryGenerationFailed,
	)

	entries := d.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Level != LogLevelError {
		t.Errorf("level = %s, want ERROR", e.Level)
	}
	if e.Message != "summary generation failed" {
		t.Errorf("message = %q, want 'summary generation failed'", e.Message)
	}
	if e.Attrs["error_id"] != "344" {
		t.Errorf("error_id attr = %q, want '344'", e.Attrs["error_id"])
	}
	if e.Attrs["error"] != "context deadline exceeded" {
		t.Errorf("error attr = %q, want 'context deadline exceeded'", e.Attrs["error"])
	}

	// Verify slog output contains the error ID.
	slogOut := buf.String()
	if !strings.Contains(slogOut, "344") {
		t.Error("slog output missing error ID '344'")
	}

	// Also test with nil error — only error_id should be in attrs.
	d.LogErrorWithID("nil error case", nil, apperrors.EToolUseSummaryGenerationFailed)
	entries = d.Entries()
	last := entries[len(entries)-1]
	if _, hasErr := last.Attrs["error"]; hasErr {
		t.Error("expected no 'error' attr when err is nil")
	}
	if last.Attrs["error_id"] != "344" {
		t.Errorf("error_id = %q, want '344'", last.Attrs["error_id"])
	}
}

// TestDebugIntegration_GlobalSingleton verifies SetGlobalBridgeDebug /
// GlobalBridgeDebug round-trip and that LogBridgeSkip uses it.
func TestDebugIntegration_GlobalSingleton(t *testing.T) {
	// Clean slate.
	ResetGlobalBridgeDebugForTesting()
	defer ResetGlobalBridgeDebugForTesting()

	// Before setting global, LogBridgeSkip should not panic.
	analytics.ResetForTesting()
	rec := &eventRecorder{}
	analytics.AttachSink(rec)
	defer analytics.ResetForTesting()

	LogBridgeSkip("test_no_global", "debug msg without global", nil)
	if len(rec.events) != 1 {
		t.Fatalf("expected 1 analytics event, got %d", len(rec.events))
	}

	// Now set a global and verify LogBridgeSkip writes to it.
	d := NewBridgeDebug(LogLevelDebug, 16, testLogger(&bytes.Buffer{}))
	SetGlobalBridgeDebug(d)

	LogBridgeSkip("test_with_global", "debug msg with global", nil)

	entries := d.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 debug entry, got %d", len(entries))
	}
	if entries[0].Message != "debug msg with global" {
		t.Errorf("entry message: %q", entries[0].Message)
	}
	if entries[0].Attrs["skip_reason"] != "test_with_global" {
		t.Errorf("entry attrs: %v", entries[0].Attrs)
	}
}

// TestDebugIntegration_GlobalNilSafe verifies GlobalBridgeDebug returns nil
// when unset.
func TestDebugIntegration_GlobalNilSafe(t *testing.T) {
	ResetGlobalBridgeDebugForTesting()
	defer ResetGlobalBridgeDebugForTesting()

	if d := GlobalBridgeDebug(); d != nil {
		t.Error("expected nil global debug, got non-nil")
	}
}
