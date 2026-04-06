// Package ide provides IDE integration hooks: connection management,
// at-mention extraction, selection forwarding, and logging relay.
//
// In the TS codebase these are five separate React hooks
// (useIDEIntegration, useIdeConnectionStatus, useIdeAtMentioned,
// useIdeSelection, useIdeLogging). In Go they collapse into a
// handful of structs with methods.
package ide

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Connection state
// ---------------------------------------------------------------------------

// ConnState represents the IDE WebSocket/SSE connection lifecycle.
type ConnState int

const (
	// Disconnected means no IDE connection exists.
	Disconnected ConnState = iota
	// Connecting means a connection attempt is in progress.
	Connecting
	// Connected means the IDE extension is reachable.
	Connected
)

// String implements fmt.Stringer.
func (s ConnState) String() string {
	switch s {
	case Disconnected:
		return "disconnected"
	case Connecting:
		return "connecting"
	case Connected:
		return "connected"
	default:
		return fmt.Sprintf("ConnState(%d)", int(s))
	}
}

// TransportType distinguishes WebSocket from SSE IDE connections.
type TransportType int

const (
	TransportNone TransportType = iota
	TransportWS                 // ws: URL → WebSocket
	TransportSSE                // http/https URL → SSE
)

// TransportFromURL returns the transport type based on the URL scheme prefix,
// matching the TS logic: url.startsWith("ws:") → ws-ide, else → sse-ide.
func TransportFromURL(url string) TransportType {
	if strings.HasPrefix(url, "ws:") || strings.HasPrefix(url, "wss:") {
		return TransportWS
	}
	return TransportSSE
}

// ---------------------------------------------------------------------------
// IDEConnection — mirrors useIdeConnectionStatus
// ---------------------------------------------------------------------------

// IDEConnection tracks the lifecycle of a connection to an IDE extension.
// It is safe for concurrent use.
type IDEConnection struct {
	mu        sync.RWMutex
	state     ConnState
	ideName   string
	url       string
	authToken string
	transport TransportType
}

// NewIDEConnection returns a new connection in the Disconnected state.
func NewIDEConnection() *IDEConnection {
	return &IDEConnection{state: Disconnected}
}

// State returns the current connection state.
func (c *IDEConnection) State() ConnState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// IDEName returns the name of the connected IDE (e.g. "vscode"), or "".
func (c *IDEConnection) IDEName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ideName
}

// URL returns the connection URL, or "".
func (c *IDEConnection) URL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.url
}

// Transport returns the transport type derived from the URL.
func (c *IDEConnection) Transport() TransportType {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.transport
}

// SetConnecting transitions to Connecting and records IDE metadata.
func (c *IDEConnection) SetConnecting(ideName, url, authToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = Connecting
	c.ideName = ideName
	c.url = url
	c.authToken = authToken
	c.transport = TransportFromURL(url)
}

// SetConnected transitions to Connected. It is a no-op if the connection
// has not been through SetConnecting first.
func (c *IDEConnection) SetConnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == Connecting {
		c.state = Connected
	}
}

// SetDisconnected resets the connection to Disconnected and clears metadata.
func (c *IDEConnection) SetDisconnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = Disconnected
	c.ideName = ""
	c.url = ""
	c.authToken = ""
	c.transport = TransportNone
}

// ---------------------------------------------------------------------------
// AtMention — mirrors useIdeAtMentioned
// ---------------------------------------------------------------------------

// AtMention represents a file reference extracted from user input text
// (e.g. "@src/main.go:10-20") or received via an IDE notification.
type AtMention struct {
	FilePath  string
	LineStart int // 1-based; 0 means unset
	LineEnd   int // 1-based; 0 means unset
}

// atMentionRe matches @-references in user input text.
// Supports:
//
//	@path/to/file
//	@path/to/file:10
//	@path/to/file:10-20
var atMentionRe = regexp.MustCompile(`@([\w./_\-]+(?::(\d+)(?:-(\d+))?)?)\b`)

// ExtractAtMentions parses @file references from input text.
// Line numbers in the returned AtMention structs are 1-based.
func ExtractAtMentions(input string) []AtMention {
	matches := atMentionRe.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return nil
	}
	result := make([]AtMention, 0, len(matches))
	for _, m := range matches {
		full := m[1] // capture group 1: path possibly with :line(-line)
		am := AtMention{}

		// Split off optional :line(-line) suffix.
		if idx := strings.LastIndex(full, ":"); idx >= 0 {
			am.FilePath = full[:idx]
			lineSpec := full[idx+1:]
			am.LineStart, am.LineEnd = parseLineSpec(lineSpec)
		} else {
			am.FilePath = full
		}
		result = append(result, am)
	}
	return result
}

// parseLineSpec parses "10" or "10-20" into 1-based line numbers.
func parseLineSpec(s string) (start, end int) {
	if i := strings.IndexByte(s, '-'); i >= 0 {
		start = atoi(s[:i])
		end = atoi(s[i+1:])
	} else {
		start = atoi(s)
	}
	return
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// AtMentionFromNotification creates an AtMention from an IDE notification,
// converting 0-based line numbers to 1-based (matching TS behaviour:
// lineStart + 1, lineEnd + 1).
func AtMentionFromNotification(filePath string, lineStart, lineEnd int, hasStart, hasEnd bool) AtMention {
	am := AtMention{FilePath: filePath}
	if hasStart {
		am.LineStart = lineStart + 1
	}
	if hasEnd {
		am.LineEnd = lineEnd + 1
	}
	return am
}

// ---------------------------------------------------------------------------
// Selection — mirrors useIdeSelection
// ---------------------------------------------------------------------------

// SelectionPoint is a cursor position in the IDE editor (0-based).
type SelectionPoint struct {
	Line      int
	Character int
}

// Selection represents the current text selection in the IDE.
type Selection struct {
	LineCount int
	LineStart int    // 0-based; -1 means unset
	Text      string // may be empty
	FilePath  string // may be empty
}

// EmptySelection is the zero-value sentinel for "no selection".
var EmptySelection = Selection{LineStart: -1}

// SelectionFromRange computes a Selection from IDE start/end points, matching
// the TS logic: lineCount = end.line - start.line + 1, but if end.character
// is 0 the final line is not counted.
func SelectionFromRange(start, end SelectionPoint, text, filePath string) Selection {
	lineCount := end.Line - start.Line + 1
	if end.Character == 0 {
		lineCount--
	}
	return Selection{
		LineCount: lineCount,
		LineStart: start.Line,
		Text:      text,
		FilePath:  filePath,
	}
}

// ---------------------------------------------------------------------------
// LogEvent — mirrors useIdeLogging
// ---------------------------------------------------------------------------

// LogEvent represents a log_event notification from the IDE extension.
type LogEvent struct {
	EventName string
	EventData map[string]any
}

// PrefixedName returns the event name with the "tengu_ide_" prefix,
// matching the TS convention.
func (e LogEvent) PrefixedName() string {
	return "tengu_ide_" + e.EventName
}
