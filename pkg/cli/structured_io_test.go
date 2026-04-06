package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helper: create a StructuredIO wired to in-memory buffers
// ---------------------------------------------------------------------------

func newTestSIO(input string) (*StructuredIO, *bytes.Buffer) {
	out := &bytes.Buffer{}
	sio, err := NewStructuredIO(StructuredIOConfig{
		Input:  strings.NewReader(input),
		Output: out,
	})
	if err != nil {
		panic(err)
	}
	return sio, out
}

// drain reads all messages from the channel until it closes or timeout.
func drain(ch <-chan StdinMessage, timeout time.Duration) []StdinMessage {
	var msgs []StdinMessage
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return msgs
			}
			msgs = append(msgs, msg)
		case <-timer.C:
			return msgs
		}
	}
}

// ---------------------------------------------------------------------------
// Test: ping control_request → pong response
// ---------------------------------------------------------------------------

func TestStructuredIO_PingPong(t *testing.T) {
	// Build a control_request with subtype=ping.
	pingMsg := StdinMessage{
		Type:      "control_request",
		RequestID: "req-ping-1",
		Request:   &ControlRequestBody{Subtype: "ping"},
	}

	sio, _ := newTestSIO("")
	defer sio.Close()

	resp, err := sio.HandleControlRequest(pingMsg)
	if err != nil {
		t.Fatalf("HandleControlRequest(ping) error: %v", err)
	}

	m, ok := resp.(StdoutMessage)
	if !ok {
		t.Fatalf("expected StdoutMessage, got %T", resp)
	}
	if m["type"] != "pong" {
		t.Errorf("ping response type = %q, want %q", m["type"], "pong")
	}
}

// ---------------------------------------------------------------------------
// Test: permission response routing (pending request resolved by control_response)
// ---------------------------------------------------------------------------

func TestStructuredIO_PermissionResponseRouting(t *testing.T) {
	// We'll feed a control_response through the input after sending a
	// control_request. Use a pipe so we can write after construction.
	pr, pw := io.Pipe()
	out := &bytes.Buffer{}

	sio, err := NewStructuredIO(StructuredIOConfig{
		Input:  pr,
		Output: out,
	})
	if err != nil {
		t.Fatalf("NewStructuredIO: %v", err)
	}
	defer sio.Close()

	// Send a control_request in the background.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var (
		wg       sync.WaitGroup
		gotResp  json.RawMessage
		gotErr   error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		gotResp, gotErr = sio.SendControlRequest(ctx, "req-abc", map[string]any{
			"subtype":     "can_use_tool",
			"tool_name":   "Bash",
			"tool_use_id": "tu-123",
			"input":       map[string]any{"command": "ls"},
		})
	}()

	// Give the request time to register.
	time.Sleep(50 * time.Millisecond)

	// Verify the pending request is tracked.
	pending := sio.GetPendingPermissionRequests()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending permission request, got %d", len(pending))
	}
	if pending[0].RequestID != "req-abc" {
		t.Errorf("pending request_id = %q, want %q", pending[0].RequestID, "req-abc")
	}

	// Write a matching control_response to stdin.
	resp := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"request_id": "req-abc",
			"subtype":    "success",
			"response":   map[string]any{"behavior": "allow", "toolUseID": "tu-123"},
		},
	}
	respBytes, _ := json.Marshal(resp)
	_, _ = pw.Write(append(respBytes, '\n'))

	wg.Wait()

	if gotErr != nil {
		t.Fatalf("SendControlRequest returned error: %v", gotErr)
	}

	// Verify we got the response payload.
	var payload struct {
		Behavior string `json:"behavior"`
	}
	if err := json.Unmarshal(gotResp, &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Behavior != "allow" {
		t.Errorf("response behavior = %q, want %q", payload.Behavior, "allow")
	}

	// Verify pending requests are now empty.
	if n := len(sio.GetPendingPermissionRequests()); n != 0 {
		t.Errorf("expected 0 pending requests after resolve, got %d", n)
	}

	// Close the pipe to let the read loop finish.
	_ = pw.Close()
}

// ---------------------------------------------------------------------------
// Test: stream-json input parsing (line-delimited JSON)
// ---------------------------------------------------------------------------

func TestStructuredIO_StreamJsonInputParsing(t *testing.T) {
	lines := []string{
		`{"type":"user","session_id":"s1","message":{"role":"user","content":"hello"}}`,
		``,
		`{"type":"keep_alive"}`,
		`{"type":"user","session_id":"s2","message":{"role":"user","content":"world"}}`,
		`{"type":"assistant"}`,
	}
	input := strings.Join(lines, "\n") + "\n"

	sio, _ := newTestSIO(input)
	msgs := drain(sio.Messages(), 2*time.Second)

	// We expect: user(hello), user(world), assistant — keep_alive and empty
	// lines are filtered.
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d: %+v", len(msgs), msgs)
	}

	if msgs[0].Type != "user" || msgs[0].Message.Content != "hello" {
		t.Errorf("msg[0] = %+v, want user/hello", msgs[0])
	}
	if msgs[1].Type != "user" || msgs[1].Message.Content != "world" {
		t.Errorf("msg[1] = %+v, want user/world", msgs[1])
	}
	if msgs[2].Type != "assistant" {
		t.Errorf("msg[2].Type = %q, want assistant", msgs[2].Type)
	}
}

// ---------------------------------------------------------------------------
// Test: Write emits valid NDJSON
// ---------------------------------------------------------------------------

func TestStructuredIO_WriteNDJSON(t *testing.T) {
	sio, out := newTestSIO("")
	defer sio.Close()

	msg := StdoutMessage{"type": "result", "content": "ok"}
	if err := sio.Write(msg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	line := strings.TrimSpace(out.String())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, line)
	}
	if parsed["type"] != "result" {
		t.Errorf("type = %v, want result", parsed["type"])
	}
}

// ---------------------------------------------------------------------------
// Test: error control_response rejects pending request
// ---------------------------------------------------------------------------

func TestStructuredIO_ErrorControlResponse(t *testing.T) {
	pr, pw := io.Pipe()
	out := &bytes.Buffer{}

	sio, err := NewStructuredIO(StructuredIOConfig{
		Input:  pr,
		Output: out,
	})
	if err != nil {
		t.Fatalf("NewStructuredIO: %v", err)
	}
	defer sio.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var (
		wg     sync.WaitGroup
		gotErr error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, gotErr = sio.SendControlRequest(ctx, "req-err", map[string]any{
			"subtype": "can_use_tool",
		})
	}()

	time.Sleep(50 * time.Millisecond)

	// Send error response.
	resp := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"request_id": "req-err",
			"subtype":    "error",
			"error":      "permission denied by policy",
		},
	}
	respBytes, _ := json.Marshal(resp)
	_, _ = pw.Write(append(respBytes, '\n'))

	wg.Wait()

	if gotErr == nil {
		t.Fatal("expected error from SendControlRequest, got nil")
	}
	if !strings.Contains(gotErr.Error(), "permission denied by policy") {
		t.Errorf("error = %q, want to contain 'permission denied by policy'", gotErr.Error())
	}

	_ = pw.Close()
}

// ---------------------------------------------------------------------------
// Test: input stream close rejects pending requests
// ---------------------------------------------------------------------------

func TestStructuredIO_InputCloseRejectsPending(t *testing.T) {
	pr, pw := io.Pipe()
	out := &bytes.Buffer{}

	sio, err := NewStructuredIO(StructuredIOConfig{
		Input:  pr,
		Output: out,
	})
	if err != nil {
		t.Fatalf("NewStructuredIO: %v", err)
	}
	defer sio.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var (
		wg     sync.WaitGroup
		gotErr error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, gotErr = sio.SendControlRequest(ctx, "req-close", map[string]any{
			"subtype": "can_use_tool",
		})
	}()

	time.Sleep(50 * time.Millisecond)

	// Close the input — should reject pending requests.
	_ = pw.Close()

	wg.Wait()

	if gotErr == nil {
		t.Fatal("expected error when input closes, got nil")
	}
	if !strings.Contains(gotErr.Error(), "stream closed") {
		t.Errorf("error = %q, want to contain 'stream closed'", gotErr.Error())
	}
}

// ---------------------------------------------------------------------------
// Test: duplicate control_response for resolved tool_use_id is ignored
// ---------------------------------------------------------------------------

func TestStructuredIO_DuplicateControlResponseIgnored(t *testing.T) {
	pr, pw := io.Pipe()
	out := &bytes.Buffer{}

	sio, err := NewStructuredIO(StructuredIOConfig{
		Input:  pr,
		Output: out,
	})
	if err != nil {
		t.Fatalf("NewStructuredIO: %v", err)
	}
	defer sio.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = sio.SendControlRequest(ctx, "req-dup", map[string]any{
			"subtype":     "can_use_tool",
			"tool_use_id": "tu-dup",
		})
	}()

	time.Sleep(50 * time.Millisecond)

	// Send the first (legitimate) response.
	resp1 := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"request_id": "req-dup",
			"subtype":    "success",
			"response":   map[string]any{"behavior": "allow", "toolUseID": "tu-dup"},
		},
	}
	b1, _ := json.Marshal(resp1)
	_, _ = pw.Write(append(b1, '\n'))
	wg.Wait()

	// Track whether the unexpected callback fires for the duplicate.
	var unexpectedCalled bool
	sio.SetUnexpectedResponseCallback(func(msg json.RawMessage) {
		unexpectedCalled = true
	})

	// Send a duplicate response for the same tool_use_id.
	resp2 := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"request_id": "req-dup-2",
			"subtype":    "success",
			"response":   map[string]any{"behavior": "allow", "toolUseID": "tu-dup"},
		},
	}
	b2, _ := json.Marshal(resp2)
	_, _ = pw.Write(append(b2, '\n'))

	// Give the read loop time to process.
	time.Sleep(100 * time.Millisecond)

	if unexpectedCalled {
		t.Error("unexpected response callback should NOT fire for duplicate tool_use_id")
	}

	_ = pw.Close()
}

// ---------------------------------------------------------------------------
// Test: LRU eviction beyond MaxResolvedToolUseIDs
// ---------------------------------------------------------------------------

func TestStructuredIO_LRUEviction(t *testing.T) {
	sio, _ := newTestSIO("")
	defer sio.Close()

	// Fill the LRU beyond capacity.
	for i := 0; i < MaxResolvedToolUseIDs+10; i++ {
		sio.resolvedToolUseIDs.Add(
			"tu-"+strings.Repeat("0", 5-len(itoa(i)))+itoa(i),
			struct{}{},
		)
	}

	// The cache should be at capacity (oldest entries evicted).
	if sio.resolvedToolUseIDs.Len() != MaxResolvedToolUseIDs {
		t.Errorf("LRU size = %d, want %d", sio.resolvedToolUseIDs.Len(), MaxResolvedToolUseIDs)
	}

	// The first entries should have been evicted.
	if sio.resolvedToolUseIDs.Contains("tu-00000") {
		t.Error("expected tu-00000 to be evicted from LRU")
	}

	// Recent entries should still be present.
	last := "tu-0" + itoa(MaxResolvedToolUseIDs+9)
	if !sio.resolvedToolUseIDs.Contains(last) {
		t.Errorf("expected %s to be in LRU", last)
	}
}

func itoa(n int) string {
	return strings.TrimLeft(fmt.Sprintf("%d", n), " ")
}

// ---------------------------------------------------------------------------
// Test: HandleControlRequest routes all 9 subtypes without error
// ---------------------------------------------------------------------------

func TestStructuredIO_ControlRequestSubtypes(t *testing.T) {
	subtypes := []string{
		ControlSubtypePing,
		ControlSubtypeSetModel,
		ControlSubtypeInterrupt,
		ControlSubtypeResume,
		ControlSubtypeMCPSetServers,
		ControlSubtypeMCPMessage,
		ControlSubtypePermissionResponse,
		ControlSubtypeSetMetadata,
		ControlSubtypeReloadPlugins,
	}

	sio, _ := newTestSIO("")
	defer sio.Close()

	for _, st := range subtypes {
		t.Run(st, func(t *testing.T) {
			msg := StdinMessage{
				Type:      "control_request",
				RequestID: "req-" + st,
				Request:   &ControlRequestBody{Subtype: st},
			}
			resp, err := sio.HandleControlRequest(msg)
			if err != nil {
				t.Fatalf("HandleControlRequest(%s) error: %v", st, err)
			}
			if resp == nil {
				t.Fatalf("HandleControlRequest(%s) returned nil", st)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: unknown subtype returns error
// ---------------------------------------------------------------------------

func TestStructuredIO_UnknownSubtypeError(t *testing.T) {
	sio, _ := newTestSIO("")
	defer sio.Close()

	msg := StdinMessage{
		Type:      "control_request",
		RequestID: "req-unknown",
		Request:   &ControlRequestBody{Subtype: "totally_unknown"},
	}
	_, err := sio.HandleControlRequest(msg)
	if err == nil {
		t.Fatal("expected error for unknown subtype")
	}
	if !strings.Contains(err.Error(), "totally_unknown") {
		t.Errorf("error = %q, want to mention the unknown subtype", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Test: constants
// ---------------------------------------------------------------------------

func TestConstants(t *testing.T) {
	if SandboxNetworkAccessToolName != "SandboxNetworkAccess" {
		t.Errorf("SandboxNetworkAccessToolName = %q, want SandboxNetworkAccess", SandboxNetworkAccessToolName)
	}
	if MaxResolvedToolUseIDs != 1000 {
		t.Errorf("MaxResolvedToolUseIDs = %d, want 1000", MaxResolvedToolUseIDs)
	}
}
