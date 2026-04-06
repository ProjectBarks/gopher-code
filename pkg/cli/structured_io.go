// StructuredIO — stdin/stdout SDK protocol handler for stream-JSON I/O.
// Parses line-delimited JSON from stdin, dispatches control_request/response
// messages, and writes StdoutMessages as NDJSON lines to stdout.
// Source: src/cli/structuredIO.ts
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

// SandboxNetworkAccessToolName is the synthetic tool name used when forwarding
// sandbox network permission requests via the can_use_tool control_request
// protocol. SDK hosts see this as a normal tool permission prompt.
const SandboxNetworkAccessToolName = "SandboxNetworkAccess"

// MaxResolvedToolUseIDs is the maximum number of resolved tool_use IDs to
// track. Once exceeded the oldest entry is evicted. This bounds memory in
// very long sessions while keeping enough history to catch duplicate
// control_response deliveries.
const MaxResolvedToolUseIDs = 1000

// ---------------------------------------------------------------------------
// Message types — SDK protocol envelope
// ---------------------------------------------------------------------------

// StdinMessage is the union type for messages received on stdin.
// The Type field discriminates: "user", "assistant", "system",
// "control_request", "control_response", "keep_alive",
// "update_environment_variables".
type StdinMessage struct {
	Type string `json:"type"`

	// For type=="user"
	SessionID       string          `json:"session_id,omitempty"`
	Message         *UserMessage    `json:"message,omitempty"`
	ParentToolUseID *string         `json:"parent_tool_use_id,omitempty"`

	// For type=="control_request"
	Request *ControlRequestBody `json:"request,omitempty"`
	// control_request and control_response share request_id at top level
	RequestID string `json:"request_id,omitempty"`

	// For type=="control_response"
	Response *ControlResponseBody `json:"response,omitempty"`
	UUID     string               `json:"uuid,omitempty"`

	// For type=="update_environment_variables"
	Variables map[string]string `json:"variables,omitempty"`

	// Raw preserves the original JSON for passthrough / unknown fields.
	Raw json.RawMessage `json:"-"`
}

// UserMessage is the inner message payload for type=="user".
type UserMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ControlRequestBody is the inner request payload for type=="control_request".
type ControlRequestBody struct {
	Subtype string `json:"subtype"`
	// Remaining fields depend on subtype; stored as raw JSON for dispatch.
	Raw json.RawMessage `json:"-"`
}

// ControlResponseBody is the inner response payload for type=="control_response".
type ControlResponseBody struct {
	RequestID string          `json:"request_id"`
	Subtype   string          `json:"subtype"` // "success" or "error"
	Response  json.RawMessage `json:"response,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// StdoutMessage is a JSON-serializable message written to stdout.
type StdoutMessage map[string]any

// SDKControlRequest is the outbound control_request envelope.
type SDKControlRequest struct {
	Type      string         `json:"type"`       // always "control_request"
	RequestID string         `json:"request_id"`
	Request   map[string]any `json:"request"`
}

// ---------------------------------------------------------------------------
// PendingRequest — tracks an in-flight control_request awaiting response
// ---------------------------------------------------------------------------

// PendingRequest represents a control_request that has been sent and is
// awaiting a control_response from the SDK host.
type PendingRequest struct {
	// ch receives exactly one result (or is closed on error).
	ch chan PendingResult
	// Request is the original outbound control_request (for introspection).
	Request SDKControlRequest
}

// PendingResult carries either a successful response payload or an error.
type PendingResult struct {
	Response json.RawMessage
	Err      error
}

// ---------------------------------------------------------------------------
// StructuredIO
// ---------------------------------------------------------------------------

// StructuredIO provides structured line-delimited JSON I/O between the SDK
// parent process and Claude Code. It reads StdinMessages from an io.Reader,
// dispatches control_request routing, tracks pending permission prompts, and
// writes StdoutMessages as NDJSON lines to an io.Writer.
type StructuredIO struct {
	mu     sync.Mutex
	logger *slog.Logger

	// input is the source of NDJSON lines (typically os.Stdin).
	input io.Reader
	// output is the destination for NDJSON lines (typically os.Stdout).
	output io.Writer

	// pendingRequests maps request_id → PendingRequest for in-flight
	// control_requests awaiting a control_response.
	pendingRequests map[string]*PendingRequest

	// resolvedToolUseIDs is an LRU cache of tool_use IDs that have been
	// resolved through the normal permission flow. Duplicate control_response
	// deliveries for these IDs are silently ignored.
	resolvedToolUseIDs *lru.Cache[string, struct{}]

	// inputClosed is set when the input stream ends.
	inputClosed bool

	// replayUserMessages controls whether control_response messages are
	// propagated to the message channel when true.
	replayUserMessages bool

	// unexpectedResponseFn is called when a control_response arrives for
	// an unknown request_id (and it's not a known duplicate).
	unexpectedResponseFn func(msg json.RawMessage)

	// onControlRequestSent is called when a can_use_tool control_request
	// is written to stdout.
	onControlRequestSent func(req SDKControlRequest)

	// onControlRequestResolved is called when a can_use_tool control_response
	// arrives from the SDK consumer.
	onControlRequestResolved func(requestID string)

	// messages is the channel that Read() yields parsed messages into.
	messages chan StdinMessage
	// readErr stores any fatal error from the read goroutine.
	readErr error
	// readDone is closed when the read goroutine exits.
	readDone chan struct{}
	// cancel stops the read goroutine.
	cancel context.CancelFunc
}

// StructuredIOConfig holds construction parameters.
type StructuredIOConfig struct {
	Input              io.Reader
	Output             io.Writer
	ReplayUserMessages bool
	Logger             *slog.Logger
}

// NewStructuredIO creates a StructuredIO and starts the background read loop.
func NewStructuredIO(cfg StructuredIOConfig) (*StructuredIO, error) {
	if cfg.Input == nil {
		return nil, errors.New("structured_io: Input is required")
	}
	if cfg.Output == nil {
		return nil, errors.New("structured_io: Output is required")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	cache, err := lru.New[string, struct{}](MaxResolvedToolUseIDs)
	if err != nil {
		return nil, fmt.Errorf("structured_io: create LRU cache: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sio := &StructuredIO{
		logger:             logger,
		input:              cfg.Input,
		output:             cfg.Output,
		pendingRequests:    make(map[string]*PendingRequest),
		resolvedToolUseIDs: cache,
		replayUserMessages: cfg.ReplayUserMessages,
		messages:           make(chan StdinMessage, 64),
		readDone:           make(chan struct{}),
		cancel:             cancel,
	}

	go sio.readLoop(ctx)
	return sio, nil
}

// Messages returns the channel of parsed inbound messages.
func (s *StructuredIO) Messages() <-chan StdinMessage {
	return s.messages
}

// Close stops the read loop and rejects all pending requests.
func (s *StructuredIO) Close() {
	s.cancel()
	<-s.readDone
}

// Write serializes msg as a single NDJSON line to stdout.
func (s *StructuredIO) Write(msg any) error {
	line := NdjsonSafeStringify(msg) + "\n"
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := io.WriteString(s.output, line)
	return err
}

// SendControlRequest sends an outbound control_request and blocks until the
// corresponding control_response arrives (or ctx is cancelled).
func (s *StructuredIO) SendControlRequest(ctx context.Context, requestID string, request map[string]any) (json.RawMessage, error) {
	msg := SDKControlRequest{
		Type:      "control_request",
		RequestID: requestID,
		Request:   request,
	}

	s.mu.Lock()
	if s.inputClosed {
		s.mu.Unlock()
		return nil, errors.New("stream closed")
	}

	pr := &PendingRequest{
		ch:      make(chan PendingResult, 1),
		Request: msg,
	}
	s.pendingRequests[requestID] = pr
	s.mu.Unlock()

	// Write the request to stdout.
	if err := s.Write(msg); err != nil {
		s.mu.Lock()
		delete(s.pendingRequests, requestID)
		s.mu.Unlock()
		return nil, fmt.Errorf("write control_request: %w", err)
	}

	// Notify callback if this is a can_use_tool request.
	if request["subtype"] == "can_use_tool" {
		s.mu.Lock()
		cb := s.onControlRequestSent
		s.mu.Unlock()
		if cb != nil {
			cb(msg)
		}
	}

	// Wait for result or cancellation.
	select {
	case result := <-pr.ch:
		return result.Response, result.Err
	case <-ctx.Done():
		// Send cancel to host.
		_ = s.Write(StdoutMessage{
			"type":       "control_cancel_request",
			"request_id": requestID,
		})
		s.mu.Lock()
		// Track resolved tool use ID before removing.
		if request["subtype"] == "can_use_tool" {
			if id, ok := request["tool_use_id"].(string); ok {
				s.resolvedToolUseIDs.Add(id, struct{}{})
			}
		}
		delete(s.pendingRequests, requestID)
		s.mu.Unlock()
		return nil, ctx.Err()
	}
}

// GetPendingPermissionRequests returns the SDKControlRequests for all
// pending can_use_tool requests.
func (s *StructuredIO) GetPendingPermissionRequests() []SDKControlRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []SDKControlRequest
	for _, pr := range s.pendingRequests {
		if sub, _ := pr.Request.Request["subtype"].(string); sub == "can_use_tool" {
			out = append(out, pr.Request)
		}
	}
	return out
}

// SetUnexpectedResponseCallback registers a handler for control_response
// messages that don't match any pending request.
func (s *StructuredIO) SetUnexpectedResponseCallback(fn func(msg json.RawMessage)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unexpectedResponseFn = fn
}

// SetOnControlRequestSent registers a callback invoked when a can_use_tool
// control_request is written.
func (s *StructuredIO) SetOnControlRequestSent(fn func(req SDKControlRequest)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onControlRequestSent = fn
}

// SetOnControlRequestResolved registers a callback invoked when a
// can_use_tool control_response arrives.
func (s *StructuredIO) SetOnControlRequestResolved(fn func(requestID string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onControlRequestResolved = fn
}

// ---------------------------------------------------------------------------
// readLoop — background goroutine that parses NDJSON lines from input
// ---------------------------------------------------------------------------

func (s *StructuredIO) readLoop(ctx context.Context) {
	defer close(s.readDone)
	defer close(s.messages)

	scanner := bufio.NewScanner(s.input)
	// Allow large lines (16 MiB max).
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			s.rejectAllPending("read loop cancelled")
			return
		default:
		}

		line := scanner.Text()
		msg, emit := s.processLine(line)
		if emit {
			select {
			case s.messages <- msg:
			case <-ctx.Done():
				s.rejectAllPending("read loop cancelled")
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		s.readErr = err
	}

	s.mu.Lock()
	s.inputClosed = true
	s.mu.Unlock()
	s.rejectAllPending("tool permission stream closed before response received")
}

// processLine parses a single NDJSON line and routes control messages.
// Returns the parsed message and whether it should be emitted to the
// messages channel.
func (s *StructuredIO) processLine(line string) (StdinMessage, bool) {
	if line == "" {
		return StdinMessage{}, false
	}

	var raw json.RawMessage
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		s.logger.Error("error parsing streaming input line", "line", line, "err", err)
		return StdinMessage{}, false
	}

	// Partial decode to get the type field.
	var envelope struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(raw, &envelope)

	switch envelope.Type {
	case "keep_alive":
		// Silently ignore keep-alive messages.
		return StdinMessage{}, false

	case "update_environment_variables":
		// Not implemented in Go yet — log and skip.
		s.logger.Debug("ignoring update_environment_variables (not implemented)")
		return StdinMessage{}, false

	case "control_response":
		return s.handleControlResponse(raw)

	case "control_request":
		return s.parseFullMessage(raw)

	case "user", "assistant", "system":
		return s.parseFullMessage(raw)

	default:
		s.logger.Warn("ignoring unknown message type", "type", envelope.Type)
		return StdinMessage{}, false
	}
}

// parseFullMessage unmarshals a complete StdinMessage from raw JSON.
func (s *StructuredIO) parseFullMessage(raw json.RawMessage) (StdinMessage, bool) {
	var msg StdinMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		s.logger.Error("error unmarshalling message", "err", err)
		return StdinMessage{}, false
	}
	msg.Raw = raw

	// Validate control_request has a request body.
	if msg.Type == "control_request" && msg.Request == nil {
		s.logger.Error("missing request on control_request")
		return StdinMessage{}, false
	}

	// Validate user messages have role=="user".
	if msg.Type == "user" && msg.Message != nil && msg.Message.Role != "user" {
		s.logger.Error("expected message role 'user'", "got", msg.Message.Role)
		return StdinMessage{}, false
	}

	return msg, true
}

// handleControlResponse processes a control_response message, resolving or
// rejecting the matching pending request.
func (s *StructuredIO) handleControlResponse(raw json.RawMessage) (StdinMessage, bool) {
	var resp struct {
		Type     string               `json:"type"`
		UUID     string               `json:"uuid,omitempty"`
		Response ControlResponseBody  `json:"response"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		s.logger.Error("error parsing control_response", "err", err)
		return StdinMessage{}, false
	}

	requestID := resp.Response.RequestID

	s.mu.Lock()
	pr, ok := s.pendingRequests[requestID]
	if !ok {
		// Check if this is a duplicate for an already-resolved tool use.
		s.mu.Unlock()

		// Check tool_use_id in the response payload for dedup.
		if resp.Response.Subtype == "success" && resp.Response.Response != nil {
			var payload struct {
				ToolUseID string `json:"toolUseID"`
			}
			if json.Unmarshal(resp.Response.Response, &payload) == nil && payload.ToolUseID != "" {
				if s.resolvedToolUseIDs.Contains(payload.ToolUseID) {
					s.logger.Debug("ignoring duplicate control_response for already-resolved toolUseID",
						"toolUseID", payload.ToolUseID, "request_id", requestID)
					return StdinMessage{}, false
				}
			}
		}

		// Unknown request — invoke unexpected callback if set.
		s.mu.Lock()
		cb := s.unexpectedResponseFn
		s.mu.Unlock()
		if cb != nil {
			cb(raw)
		}
		return StdinMessage{}, false
	}

	// Track resolved tool_use_id.
	if sub, _ := pr.Request.Request["subtype"].(string); sub == "can_use_tool" {
		if id, _ := pr.Request.Request["tool_use_id"].(string); id != "" {
			s.resolvedToolUseIDs.Add(id, struct{}{})
		}
	}
	delete(s.pendingRequests, requestID)

	// Notify callback for can_use_tool resolution.
	var resolvedCb func(string)
	if sub, _ := pr.Request.Request["subtype"].(string); sub == "can_use_tool" {
		resolvedCb = s.onControlRequestResolved
	}
	s.mu.Unlock()

	if resolvedCb != nil {
		resolvedCb(requestID)
	}

	// Resolve or reject the pending request.
	if resp.Response.Subtype == "error" {
		pr.ch <- PendingResult{Err: errors.New(resp.Response.Error)}
	} else {
		pr.ch <- PendingResult{Response: resp.Response.Response}
	}

	// Propagate control_response when replay is enabled.
	if s.replayUserMessages {
		msg := StdinMessage{Type: "control_response", Raw: raw}
		return msg, true
	}
	return StdinMessage{}, false
}

// InjectControlResponse programmatically resolves a pending request (used by
// the bridge to feed permission responses from claude.ai into the SDK flow).
func (s *StructuredIO) InjectControlResponse(requestID string, subtype string, response json.RawMessage, errMsg string) {
	s.mu.Lock()
	pr, ok := s.pendingRequests[requestID]
	if !ok {
		s.mu.Unlock()
		return
	}

	// Track resolved tool_use_id.
	if sub, _ := pr.Request.Request["subtype"].(string); sub == "can_use_tool" {
		if id, _ := pr.Request.Request["tool_use_id"].(string); id != "" {
			s.resolvedToolUseIDs.Add(id, struct{}{})
		}
	}
	delete(s.pendingRequests, requestID)
	s.mu.Unlock()

	// Send cancel to SDK consumer.
	_ = s.Write(StdoutMessage{
		"type":       "control_cancel_request",
		"request_id": requestID,
	})

	if subtype == "error" {
		pr.ch <- PendingResult{Err: errors.New(errMsg)}
	} else {
		pr.ch <- PendingResult{Response: response}
	}
}

// rejectAllPending rejects all outstanding pending requests with the given
// message.
func (s *StructuredIO) rejectAllPending(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, pr := range s.pendingRequests {
		pr.ch <- PendingResult{Err: errors.New(msg)}
		delete(s.pendingRequests, id)
	}
}

// ---------------------------------------------------------------------------
// SDKControlRequest subtypes — routing helpers
// ---------------------------------------------------------------------------

// ControlSubtype enumerates the known SDKControlRequest subtypes.
const (
	ControlSubtypePermissionResponse = "permission_response"
	ControlSubtypeSetModel           = "set_model"
	ControlSubtypeInterrupt          = "interrupt"
	ControlSubtypeResume             = "resume"
	ControlSubtypeMCPSetServers      = "mcp_set_servers"
	ControlSubtypeMCPMessage         = "mcp_message"
	ControlSubtypePing               = "ping"
	ControlSubtypeSetMetadata        = "set_metadata"
	ControlSubtypeReloadPlugins      = "reload_plugins"
)

// HandleControlRequest routes an inbound control_request to the appropriate
// handler based on subtype. Returns a response payload to send back, or an
// error.
func (s *StructuredIO) HandleControlRequest(msg StdinMessage) (any, error) {
	if msg.Request == nil {
		return nil, errors.New("missing request body")
	}

	// Re-parse the subtype from Raw if available; fall back to Subtype field.
	subtype := msg.Request.Subtype
	if subtype == "" && msg.Raw != nil {
		var envelope struct {
			Request struct {
				Subtype string `json:"subtype"`
			} `json:"request"`
		}
		if json.Unmarshal(msg.Raw, &envelope) == nil {
			subtype = envelope.Request.Subtype
		}
	}

	switch subtype {
	case ControlSubtypePing:
		return s.handlePing(msg)
	case ControlSubtypeSetModel,
		ControlSubtypeInterrupt,
		ControlSubtypeResume,
		ControlSubtypeMCPSetServers,
		ControlSubtypeMCPMessage,
		ControlSubtypePermissionResponse,
		ControlSubtypeSetMetadata,
		ControlSubtypeReloadPlugins:
		// Stub: these subtypes are recognized but not yet implemented.
		s.logger.Debug("control_request subtype recognized but not yet implemented", "subtype", subtype)
		return StdoutMessage{"status": "ok"}, nil
	default:
		return nil, fmt.Errorf("unknown control_request subtype: %q", subtype)
	}
}

// handlePing responds to a ping control_request with a pong.
func (s *StructuredIO) handlePing(msg StdinMessage) (any, error) {
	return StdoutMessage{"type": "pong"}, nil
}
