package bridge

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// testLogger returns a slog.Logger that writes to the given buffer.
func testLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// fixedClock returns a now func that always returns t.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestLogMessagesGoToBuffer(t *testing.T) {
	var buf bytes.Buffer
	d := NewBridgeDebug(LogLevelDebug, 16, testLogger(&buf))

	d.LogStatus("connected", map[string]string{"env": "e1"})
	d.LogTransition("idle", "polling", "timer")
	d.LogError("poll failed", errors.New("timeout"))
	d.LogAPI("POST", "/poll", 200)
	d.LogSession("sess-1", "created")

	entries := d.Entries()
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	// Verify each entry's message content.
	if entries[0].Message != "connected" {
		t.Errorf("entry 0: got %q", entries[0].Message)
	}
	if !strings.Contains(entries[1].Message, "transition") {
		t.Errorf("entry 1: got %q", entries[1].Message)
	}
	if entries[2].Message != "poll failed" {
		t.Errorf("entry 2: got %q", entries[2].Message)
	}
	if entries[2].Attrs["error"] != "timeout" {
		t.Errorf("entry 2 attrs: %v", entries[2].Attrs)
	}
	if !strings.Contains(entries[3].Message, "API POST /poll") {
		t.Errorf("entry 3: got %q", entries[3].Message)
	}
	if !strings.Contains(entries[4].Message, "session created") {
		t.Errorf("entry 4: got %q", entries[4].Message)
	}

	// Verify slog output contains [bridge] prefix.
	slogOut := buf.String()
	if !strings.Contains(slogOut, "[bridge]") {
		t.Errorf("slog output missing [bridge] prefix: %s", slogOut)
	}
}

func TestBufferSizeLimit(t *testing.T) {
	d := NewBridgeDebug(LogLevelDebug, 4, testLogger(&bytes.Buffer{}))

	// Write 7 entries into a buffer of size 4.
	for i := 0; i < 7; i++ {
		d.LogStatus("msg", map[string]string{"i": string(rune('0' + i))})
	}

	entries := d.Entries()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries (buffer cap), got %d", len(entries))
	}

	// The oldest retained entry should be i=3 (entries 0-2 evicted).
	if entries[0].Attrs["i"] != "3" {
		t.Errorf("oldest retained entry: attrs=%v, want i=3", entries[0].Attrs)
	}
	if entries[3].Attrs["i"] != "6" {
		t.Errorf("newest entry: attrs=%v, want i=6", entries[3].Attrs)
	}
}

func TestLogLevelsFilterCorrectly(t *testing.T) {
	var buf bytes.Buffer
	d := NewBridgeDebug(LogLevelWarn, 16, testLogger(&buf))

	// Debug and Info should be filtered out.
	d.LogAPI("GET", "/test", 200)           // debug level
	d.LogStatus("ok", nil)                  // info level
	d.LogSession("s1", "started")           // info level
	d.LogTransition("a", "b", "test")       // info level
	d.LogError("boom", errors.New("fatal")) // error level

	entries := d.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (only error), got %d", len(entries))
	}
	if entries[0].Level != LogLevelError {
		t.Errorf("expected error level, got %s", entries[0].Level)
	}
	if entries[0].Message != "boom" {
		t.Errorf("expected 'boom', got %q", entries[0].Message)
	}
}

func TestSetLevelAtRuntime(t *testing.T) {
	d := NewBridgeDebug(LogLevelError, 16, testLogger(&bytes.Buffer{}))

	d.LogStatus("ignored", nil) // info < error → filtered
	if len(d.Entries()) != 0 {
		t.Fatal("expected 0 entries before level change")
	}

	d.SetLevel(LogLevelInfo)
	d.LogStatus("visible", nil) // info >= info → kept
	if len(d.Entries()) != 1 {
		t.Fatal("expected 1 entry after level change")
	}
}

func TestLogLevelString(t *testing.T) {
	cases := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}
	for _, c := range cases {
		if got := c.level.String(); got != c.want {
			t.Errorf("LogLevel(%d).String() = %q, want %q", c.level, got, c.want)
		}
	}
}

func TestEntryStringFormat(t *testing.T) {
	ts := time.Date(2026, 1, 15, 14, 30, 45, 123000000, time.UTC)
	e := LogEntry{
		Time:    ts,
		Level:   LogLevelInfo,
		Message: "connected",
		Attrs:   map[string]string{"env": "e1"},
	}
	s := e.String()
	if !strings.HasPrefix(s, "14:30:45.123") {
		t.Errorf("timestamp format wrong: %s", s)
	}
	if !strings.Contains(s, "[bridge]") {
		t.Errorf("missing [bridge] prefix: %s", s)
	}
	if !strings.Contains(s, "INFO") {
		t.Errorf("missing level: %s", s)
	}
	if !strings.Contains(s, "env=e1") {
		t.Errorf("missing attr: %s", s)
	}
}

func TestEmptyBuffer(t *testing.T) {
	d := NewBridgeDebug(LogLevelDebug, 8, testLogger(&bytes.Buffer{}))
	entries := d.Entries()
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestDefaultBufferSize(t *testing.T) {
	d := NewBridgeDebug(LogLevelDebug, 0, nil)
	if d.bufSize != DefaultBufferSize {
		t.Errorf("expected default buffer size %d, got %d", DefaultBufferSize, d.bufSize)
	}
}

func TestTimestampFromInjectedClock(t *testing.T) {
	ts := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	d := NewBridgeDebug(LogLevelDebug, 4, testLogger(&bytes.Buffer{}))
	d.now = fixedClock(ts)

	d.LogStatus("test", nil)
	entries := d.Entries()
	if !entries[0].Time.Equal(ts) {
		t.Errorf("expected time %v, got %v", ts, entries[0].Time)
	}
}

func TestBufferWraparound(t *testing.T) {
	// Exactly fill and wrap once to verify chronological ordering.
	d := NewBridgeDebug(LogLevelDebug, 3, testLogger(&bytes.Buffer{}))

	d.log(LogLevelInfo, "a", nil)
	d.log(LogLevelInfo, "b", nil)
	d.log(LogLevelInfo, "c", nil)
	d.log(LogLevelInfo, "d", nil) // evicts "a"

	entries := d.Entries()
	if len(entries) != 3 {
		t.Fatalf("want 3, got %d", len(entries))
	}
	want := []string{"b", "c", "d"}
	for i, e := range entries {
		if e.Message != want[i] {
			t.Errorf("entry %d: got %q, want %q", i, e.Message, want[i])
		}
	}
}
