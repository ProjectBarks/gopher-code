// BridgeDebug structured logging with circular buffer.
// Source: src/bridge/bridgeDebug.ts (logging subset)
package bridge

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	apperrors "github.com/projectbarks/gopher-code/pkg/errors"
)

// ---------------------------------------------------------------------------
// Log levels
// ---------------------------------------------------------------------------

// LogLevel controls which messages are emitted by BridgeDebug.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// String returns the human label for a LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// toSlogLevel maps our LogLevel to slog.Level.
func (l LogLevel) toSlogLevel() slog.Level {
	switch l {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ---------------------------------------------------------------------------
// LogEntry — one item in the circular buffer
// ---------------------------------------------------------------------------

// LogEntry is a single structured log message retained in the circular buffer.
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
	Attrs   map[string]string
}

// String formats a LogEntry for human display.
func (e LogEntry) String() string {
	ts := e.Time.Format("15:04:05.000")
	var b strings.Builder
	fmt.Fprintf(&b, "%s [bridge] %s %s", ts, e.Level, e.Message)
	for k, v := range e.Attrs {
		fmt.Fprintf(&b, " %s=%s", k, v)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Default buffer capacity
// ---------------------------------------------------------------------------

// DefaultBufferSize is the default circular buffer capacity.
const DefaultBufferSize = 256

// ---------------------------------------------------------------------------
// BridgeDebug — structured logger with circular buffer
// ---------------------------------------------------------------------------

// BridgeDebug provides structured logging for the bridge subsystem.
// All messages carry the [bridge] prefix and are stored in a fixed-size
// circular buffer so /bridge-kick status dumps can show recent activity.
type BridgeDebug struct {
	mu      sync.Mutex
	level   LogLevel
	buf     []LogEntry
	bufSize int
	pos     int   // next write position in the ring
	count   int   // total entries written (for distinguishing empty vs full)
	logger  *slog.Logger
	now     func() time.Time // injectable clock for tests
}

// NewBridgeDebug creates a BridgeDebug with the given minimum log level,
// buffer capacity, and slog.Logger backend. If logger is nil, slog.Default()
// is used.
func NewBridgeDebug(level LogLevel, bufSize int, logger *slog.Logger) *BridgeDebug {
	if bufSize <= 0 {
		bufSize = DefaultBufferSize
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &BridgeDebug{
		level:   level,
		buf:     make([]LogEntry, bufSize),
		bufSize: bufSize,
		logger:  logger,
		now:     time.Now,
	}
}

// SetLevel changes the minimum log level at runtime.
func (d *BridgeDebug) SetLevel(l LogLevel) {
	d.mu.Lock()
	d.level = l
	d.mu.Unlock()
}

// Level returns the current minimum log level.
func (d *BridgeDebug) Level() LogLevel {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.level
}

// log is the internal workhorse. It filters by level, appends to the ring
// buffer, and emits via slog.
func (d *BridgeDebug) log(level LogLevel, msg string, attrs map[string]string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if level < d.level {
		return
	}

	entry := LogEntry{
		Time:    d.now(),
		Level:   level,
		Message: msg,
		Attrs:   attrs,
	}

	// Write into circular buffer.
	d.buf[d.pos] = entry
	d.pos = (d.pos + 1) % d.bufSize
	d.count++

	// Emit to slog with the [bridge] group prefix.
	slogAttrs := make([]any, 0, len(attrs)*2)
	for k, v := range attrs {
		slogAttrs = append(slogAttrs, k, v)
	}
	d.logger.Log(nil, level.toSlogLevel(), "[bridge] "+msg, slogAttrs...) //nolint:staticcheck
}

// Entries returns a copy of the buffered log entries in chronological order.
func (d *BridgeDebug) Entries() []LogEntry {
	d.mu.Lock()
	defer d.mu.Unlock()

	n := d.count
	if n > d.bufSize {
		n = d.bufSize
	}
	out := make([]LogEntry, n)
	if n == 0 {
		return out
	}

	start := d.pos - n
	if start < 0 {
		start += d.bufSize
	}
	for i := 0; i < n; i++ {
		out[i] = d.buf[(start+i)%d.bufSize]
	}
	return out
}

// ---------------------------------------------------------------------------
// Structured log methods — typed convenience wrappers
// ---------------------------------------------------------------------------

// LogStatus logs a bridge status message at info level.
func (d *BridgeDebug) LogStatus(msg string, attrs map[string]string) {
	d.log(LogLevelInfo, msg, attrs)
}

// LogTransition logs a state transition at info level.
func (d *BridgeDebug) LogTransition(from, to, reason string) {
	d.log(LogLevelInfo, fmt.Sprintf("transition %s → %s", from, to), map[string]string{
		"from":   from,
		"to":     to,
		"reason": reason,
	})
}

// LogError logs an error at error level.
func (d *BridgeDebug) LogError(msg string, err error) {
	attrs := map[string]string{}
	if err != nil {
		attrs["error"] = err.Error()
	}
	d.log(LogLevelError, msg, attrs)
}

// LogErrorWithID logs an error at error level and attaches a numeric error ID
// from pkg/errors so that production traces can identify the call site without
// leaking internal names. Mirrors the TS pattern:
//
//	err.cause = { errorId: E_TOOL_USE_SUMMARY_GENERATION_FAILED }
//	logError(err)
func (d *BridgeDebug) LogErrorWithID(msg string, err error, errorID int) {
	attrs := map[string]string{
		"error_id": apperrors.FormatErrorID(errorID),
	}
	if err != nil {
		attrs["error"] = err.Error()
	}
	d.log(LogLevelError, msg, attrs)
}

// LogAPI logs an API call at debug level.
func (d *BridgeDebug) LogAPI(method, path string, status int) {
	d.log(LogLevelDebug, fmt.Sprintf("API %s %s", method, path), map[string]string{
		"status": fmt.Sprintf("%d", status),
	})
}

// LogSession logs a session-related event at info level.
func (d *BridgeDebug) LogSession(sessionID, event string) {
	d.log(LogLevelInfo, fmt.Sprintf("session %s", event), map[string]string{
		"session_id": sessionID,
	})
}

// ---------------------------------------------------------------------------
// Global BridgeDebug singleton — set once during CLI bootstrap (T189)
// ---------------------------------------------------------------------------

var (
	globalDebugMu sync.Mutex
	globalDebug   *BridgeDebug
)

// SetGlobalBridgeDebug stores a process-wide BridgeDebug instance.
// Typically called once during CLI startup in the remote-control path.
func SetGlobalBridgeDebug(d *BridgeDebug) {
	globalDebugMu.Lock()
	globalDebug = d
	globalDebugMu.Unlock()
}

// GlobalBridgeDebug returns the process-wide BridgeDebug, or nil if not set.
func GlobalBridgeDebug() *BridgeDebug {
	globalDebugMu.Lock()
	defer globalDebugMu.Unlock()
	return globalDebug
}

// ResetGlobalBridgeDebugForTesting clears the global BridgeDebug (test helper).
func ResetGlobalBridgeDebugForTesting() {
	globalDebugMu.Lock()
	globalDebug = nil
	globalDebugMu.Unlock()
}
