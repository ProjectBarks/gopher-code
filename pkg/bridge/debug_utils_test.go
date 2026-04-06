package bridge

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/analytics"
)

// ---------------------------------------------------------------------------
// RedactSecrets
// ---------------------------------------------------------------------------

func TestRedactSecrets_ShortValueFullyRedacted(t *testing.T) {
	input := `{"token":"abc123"}`
	got := RedactSecrets(input)
	want := `{"token":"[REDACTED]"}`
	if got != want {
		t.Errorf("RedactSecrets short value:\n got %s\nwant %s", got, want)
	}
}

func TestRedactSecrets_LongValuePartialRedact(t *testing.T) {
	// 20-char value — longer than RedactMinLength (16).
	input := `{"access_token":"ABCDEFGHIJKLMNOPQRST"}`
	got := RedactSecrets(input)
	// first 8 + "..." + last 4
	want := `{"access_token":"ABCDEFGH...QRST"}`
	if got != want {
		t.Errorf("RedactSecrets long value:\n got %s\nwant %s", got, want)
	}
}

func TestRedactSecrets_AllFieldNames(t *testing.T) {
	for _, field := range SecretFieldNames {
		input := `{"` + field + `":"tiny"}`
		got := RedactSecrets(input)
		if !strings.Contains(got, "[REDACTED]") {
			t.Errorf("field %q not redacted in %s", field, got)
		}
	}
}

func TestRedactSecrets_NonSecretFieldUntouched(t *testing.T) {
	input := `{"username":"alice","token":"short"}`
	got := RedactSecrets(input)
	if !strings.Contains(got, `"username":"alice"`) {
		t.Errorf("non-secret field was modified: %s", got)
	}
}

func TestRedactSecrets_ExactBoundary(t *testing.T) {
	// Value of exactly 15 chars (< 16) should be fully redacted.
	val15 := "123456789012345" // len=15
	input := `{"secret":"` + val15 + `"}`
	got := RedactSecrets(input)
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("15-char value not fully redacted: %s", got)
	}

	// Value of exactly 16 chars (>= 16) should be partially redacted.
	val16 := "1234567890123456" // len=16
	input = `{"secret":"` + val16 + `"}`
	got = RedactSecrets(input)
	if strings.Contains(got, "[REDACTED]") {
		t.Errorf("16-char value should be partially redacted, got: %s", got)
	}
	if !strings.Contains(got, "12345678...3456") {
		t.Errorf("16-char value wrong partial redaction: %s", got)
	}
}

func TestRedactSecrets_SpacesAroundColon(t *testing.T) {
	input := `{"token" : "short"}`
	got := RedactSecrets(input)
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("spaces around colon not handled: %s", got)
	}
}

// ---------------------------------------------------------------------------
// DebugTruncate
// ---------------------------------------------------------------------------

func TestDebugTruncate_ShortString(t *testing.T) {
	got := DebugTruncate("hello")
	if got != "hello" {
		t.Errorf("short string changed: %q", got)
	}
}

func TestDebugTruncate_NewlineCollapse(t *testing.T) {
	got := DebugTruncate("line1\nline2\nline3")
	want := `line1\nline2\nline3`
	if got != want {
		t.Errorf("newline collapse:\n got %q\nwant %q", got, want)
	}
}

func TestDebugTruncate_LongStringTruncated(t *testing.T) {
	long := strings.Repeat("x", 3000)
	got := DebugTruncate(long)
	if len(got) < DebugMsgLimit {
		t.Fatal("truncated string too short")
	}
	if !strings.HasSuffix(got, "(3000 chars)") {
		t.Errorf("missing suffix: %s", got[len(got)-30:])
	}
	// Must start with the original prefix.
	if got[:DebugMsgLimit] != long[:DebugMsgLimit] {
		t.Error("truncated prefix does not match original")
	}
}

func TestDebugTruncate_ExactLimit(t *testing.T) {
	exact := strings.Repeat("y", DebugMsgLimit)
	got := DebugTruncate(exact)
	if got != exact {
		t.Error("exact-limit string should not be truncated")
	}
}

// ---------------------------------------------------------------------------
// DebugBody
// ---------------------------------------------------------------------------

func TestDebugBody_StringInput(t *testing.T) {
	got := DebugBody(`{"token":"short","msg":"hi"}`)
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("string input not redacted: %s", got)
	}
	if !strings.Contains(got, `"msg":"hi"`) {
		t.Errorf("non-secret field missing: %s", got)
	}
}

func TestDebugBody_MapInput(t *testing.T) {
	m := map[string]string{"token": "tiny", "key": "value"}
	got := DebugBody(m)
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("map input not redacted: %s", got)
	}
}

func TestDebugBody_TruncatesLong(t *testing.T) {
	long := strings.Repeat("a", 3000)
	got := DebugBody(long)
	if !strings.Contains(got, "... (") {
		t.Error("long string not truncated")
	}
}

// ---------------------------------------------------------------------------
// HTTPError + ExtractHTTPStatus
// ---------------------------------------------------------------------------

func TestExtractHTTPStatus_HTTPError(t *testing.T) {
	err := &HTTPError{StatusCode: 403, Msg: "forbidden"}
	code, ok := ExtractHTTPStatus(err)
	if !ok || code != 403 {
		t.Errorf("expected 403, got %d ok=%v", code, ok)
	}
}

func TestExtractHTTPStatus_NonHTTPError(t *testing.T) {
	err := &stringError{"nope"}
	code, ok := ExtractHTTPStatus(err)
	if ok || code != 0 {
		t.Errorf("expected 0/false, got %d/%v", code, ok)
	}
}

type stringError struct{ s string }

func (e *stringError) Error() string { return e.s }

// ---------------------------------------------------------------------------
// ExtractErrorDetail
// ---------------------------------------------------------------------------

func TestExtractErrorDetail_TopLevelMessage(t *testing.T) {
	body := []byte(`{"message":"rate limited"}`)
	got := ExtractErrorDetail(body)
	if got != "rate limited" {
		t.Errorf("expected 'rate limited', got %q", got)
	}
}

func TestExtractErrorDetail_NestedErrorMessage(t *testing.T) {
	body := []byte(`{"error":{"message":"bad request"}}`)
	got := ExtractErrorDetail(body)
	if got != "bad request" {
		t.Errorf("expected 'bad request', got %q", got)
	}
}

func TestExtractErrorDetail_NoMessage(t *testing.T) {
	body := []byte(`{"status":"error"}`)
	got := ExtractErrorDetail(body)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractErrorDetail_EmptyBody(t *testing.T) {
	got := ExtractErrorDetail(nil)
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestExtractErrorDetail_InvalidJSON(t *testing.T) {
	got := ExtractErrorDetail([]byte("not json"))
	if got != "" {
		t.Errorf("expected empty for invalid JSON, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// DescribeHTTPError
// ---------------------------------------------------------------------------

func TestDescribeHTTPError_WithDetail(t *testing.T) {
	err := &HTTPError{
		StatusCode: 429,
		Msg:        "HTTP 429",
		Body:       []byte(`{"message":"rate limited"}`),
	}
	got := DescribeHTTPError(err)
	want := "HTTP 429: rate limited"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDescribeHTTPError_NoBody(t *testing.T) {
	err := &HTTPError{StatusCode: 500, Msg: "HTTP 500"}
	got := DescribeHTTPError(err)
	if got != "HTTP 500" {
		t.Errorf("got %q, want %q", got, "HTTP 500")
	}
}

func TestDescribeHTTPError_PlainError(t *testing.T) {
	err := &stringError{"connection reset"}
	got := DescribeHTTPError(err)
	if got != "connection reset" {
		t.Errorf("got %q, want %q", got, "connection reset")
	}
}

// ---------------------------------------------------------------------------
// LogBridgeSkip
// ---------------------------------------------------------------------------

func TestLogBridgeSkip_EmitsAnalyticsEvent(t *testing.T) {
	analytics.ResetForTesting()
	recorder := &eventRecorder{}
	analytics.AttachSink(recorder)
	defer analytics.ResetForTesting()

	v2 := true
	LogBridgeSkip("no_token", "", &v2)

	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(recorder.events))
	}
	ev := recorder.events[0]
	if ev.name != "tengu_bridge_repl_skipped" {
		t.Errorf("event name: %q", ev.name)
	}
	if ev.meta["reason"] != "no_token" {
		t.Errorf("reason: %v", ev.meta["reason"])
	}
	if ev.meta["v2"] != true {
		t.Errorf("v2: %v", ev.meta["v2"])
	}
}

func TestLogBridgeSkip_NoV2(t *testing.T) {
	analytics.ResetForTesting()
	recorder := &eventRecorder{}
	analytics.AttachSink(recorder)
	defer analytics.ResetForTesting()

	LogBridgeSkip("disabled", "", nil)

	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(recorder.events))
	}
	if _, has := recorder.events[0].meta["v2"]; has {
		t.Error("v2 key should be absent when nil")
	}
}

// ---------------------------------------------------------------------------
// Constants sanity
// ---------------------------------------------------------------------------

func TestConstants(t *testing.T) {
	if DebugMsgLimit != 2000 {
		t.Errorf("DebugMsgLimit = %d, want 2000", DebugMsgLimit)
	}
	if RedactMinLength != 16 {
		t.Errorf("RedactMinLength = %d, want 16", RedactMinLength)
	}
	if len(SecretFieldNames) != 5 {
		t.Errorf("SecretFieldNames has %d entries, want 5", len(SecretFieldNames))
	}
}

// ---------------------------------------------------------------------------
// test helpers
// ---------------------------------------------------------------------------

type recordedEvent struct {
	name string
	meta analytics.EventMetadata
}

type eventRecorder struct {
	events []recordedEvent
}

func (r *eventRecorder) LogEvent(name string, meta analytics.EventMetadata) {
	r.events = append(r.events, recordedEvent{name, meta})
}

func (r *eventRecorder) LogEventAsync(name string, meta analytics.EventMetadata) {
	r.events = append(r.events, recordedEvent{name, meta})
}

func (r *eventRecorder) Shutdown() {}
