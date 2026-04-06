// Package bridge — debug formatting and secret-redaction utilities.
// Source: src/bridge/debugUtils.ts
package bridge

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/analytics"
)

// ---------------------------------------------------------------------------
// Constants (verbatim match with TS source)
// ---------------------------------------------------------------------------

// DebugMsgLimit is the maximum characters before truncation.
const DebugMsgLimit = 2000

// RedactMinLength — values shorter than this are fully [REDACTED].
const RedactMinLength = 16

// SecretFieldNames lists JSON field names whose values are redacted.
var SecretFieldNames = []string{
	"session_ingress_token",
	"environment_secret",
	"access_token",
	"secret",
	"token",
}

// secretPattern matches "field":"value" pairs for secret fields.
var secretPattern = regexp.MustCompile(
	`"(` + strings.Join(SecretFieldNames, "|") + `)"` + `\s*:\s*"([^"]*)"`,
)

// ---------------------------------------------------------------------------
// RedactSecrets — field-value redaction in JSON strings
// ---------------------------------------------------------------------------

// RedactSecrets replaces secret field values in a JSON-like string.
// Short values (< RedactMinLength) become [REDACTED]; longer values
// keep the first 8 and last 4 characters with "..." in between.
func RedactSecrets(s string) string {
	return secretPattern.ReplaceAllStringFunc(s, func(match string) string {
		subs := secretPattern.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		field, value := subs[1], subs[2]
		if len(value) < RedactMinLength {
			return fmt.Sprintf(`"%s":"[REDACTED]"`, field)
		}
		redacted := value[:8] + "..." + value[len(value)-4:]
		return fmt.Sprintf(`"%s":"%s"`, field, redacted)
	})
}

// ---------------------------------------------------------------------------
// DebugTruncate — newline-collapse + truncation
// ---------------------------------------------------------------------------

// DebugTruncate collapses newlines to literal "\n" and truncates to
// DebugMsgLimit, appending "... (N chars)" when truncated.
func DebugTruncate(s string) string {
	flat := strings.ReplaceAll(s, "\n", `\n`)
	if len(flat) <= DebugMsgLimit {
		return flat
	}
	return flat[:DebugMsgLimit] + fmt.Sprintf("... (%d chars)", len(flat))
}

// ---------------------------------------------------------------------------
// DebugBody — JSON-stringify + redact + truncate
// ---------------------------------------------------------------------------

// DebugBody serializes data to JSON (if not already a string), redacts
// secrets, and truncates for debug logging.
func DebugBody(data any) string {
	var raw string
	switch v := data.(type) {
	case string:
		raw = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			raw = fmt.Sprintf("%v", v)
		} else {
			raw = string(b)
		}
	}
	s := RedactSecrets(raw)
	if len(s) <= DebugMsgLimit {
		return s
	}
	return s[:DebugMsgLimit] + fmt.Sprintf("... (%d chars)", len(s))
}

// ---------------------------------------------------------------------------
// HTTP error introspection
// ---------------------------------------------------------------------------

// HTTPError represents an HTTP error with status code and response body.
// Go callers construct this explicitly rather than duck-typing like TS.
type HTTPError struct {
	StatusCode int
	Body       []byte
	Msg        string
}

func (e *HTTPError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// ExtractHTTPStatus returns the HTTP status code from an error if it is
// an *HTTPError. Returns 0 and false for non-HTTP errors.
func ExtractHTTPStatus(err error) (int, bool) {
	if he, ok := err.(*HTTPError); ok && he != nil {
		return he.StatusCode, true
	}
	return 0, false
}

// ExtractErrorDetail pulls a human-readable message out of an API error
// response body. Checks data.message first, then data.error.message.
func ExtractErrorDetail(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	if msg, ok := obj["message"].(string); ok {
		return msg
	}
	if errObj, ok := obj["error"].(map[string]any); ok {
		if msg, ok := errObj["message"].(string); ok {
			return msg
		}
	}
	return ""
}

// DescribeHTTPError extracts a descriptive error message from an error.
// For HTTPError, appends the server's response body message if available.
func DescribeHTTPError(err error) string {
	msg := err.Error()
	he, ok := err.(*HTTPError)
	if !ok || he == nil || len(he.Body) == 0 {
		return msg
	}
	detail := ExtractErrorDetail(he.Body)
	if detail != "" {
		return msg + ": " + detail
	}
	return msg
}

// ---------------------------------------------------------------------------
// LogBridgeSkip — centralized bridge init skip logging
// ---------------------------------------------------------------------------

// LogBridgeSkip logs a bridge init skip event with an optional debug message
// and the "tengu_bridge_repl_skipped" analytics event.
func LogBridgeSkip(reason string, debugMsg string, v2 *bool) {
	// debugMsg logging is intentionally deferred to a later task (T189)
	// when the bridge debug logging system is implemented.
	meta := analytics.EventMetadata{"reason": reason}
	if v2 != nil {
		meta["v2"] = *v2
	}
	analytics.LogEvent("tengu_bridge_repl_skipped", meta)
}
