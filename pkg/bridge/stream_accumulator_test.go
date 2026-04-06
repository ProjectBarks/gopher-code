package bridge

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Test: text_delta coalescing produces full-so-far snapshots
// ---------------------------------------------------------------------------

func TestAccumulateStreamEvents_TextDeltaCoalescing(t *testing.T) {
	state := NewStreamAccumulator()

	// 1. message_start establishes the scope→message mapping.
	buffer := []map[string]any{
		{
			"type":               "stream_event",
			"uuid":               "uuid-start",
			"session_id":         "sess-1",
			"parent_tool_use_id": "",
			"event": map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id": "msg-1",
				},
			},
		},
		// 2. Two text_delta chunks for block 0 in the same flush.
		{
			"type":               "stream_event",
			"uuid":               "uuid-d1",
			"session_id":         "sess-1",
			"parent_tool_use_id": "",
			"event": map[string]any{
				"type":  "content_block_delta",
				"index": float64(0),
				"delta": map[string]any{"type": "text_delta", "text": "Hello"},
			},
		},
		{
			"type":               "stream_event",
			"uuid":               "uuid-d2",
			"session_id":         "sess-1",
			"parent_tool_use_id": "",
			"event": map[string]any{
				"type":  "content_block_delta",
				"index": float64(0),
				"delta": map[string]any{"type": "text_delta", "text": " World"},
			},
		},
	}

	out := AccumulateStreamEvents(buffer, state)

	// Should produce: message_start passthrough + 1 coalesced snapshot.
	if len(out) != 2 {
		t.Fatalf("expected 2 output events, got %d", len(out))
	}

	// First event is the message_start passthrough.
	if out[0]["uuid"] != "uuid-start" {
		t.Errorf("expected first event to be message_start, got uuid=%v", out[0]["uuid"])
	}

	// Second event should be a full-so-far snapshot with "Hello World".
	evt, ok := out[1]["event"].(map[string]any)
	if !ok {
		t.Fatal("second event missing 'event' field")
	}
	delta, ok := evt["delta"].(map[string]any)
	if !ok {
		t.Fatal("second event missing 'delta' field")
	}
	text, _ := delta["text"].(string)
	if text != "Hello World" {
		t.Errorf("expected coalesced text 'Hello World', got %q", text)
	}

	// UUID should come from the first delta (uuid-d1).
	if out[1]["uuid"] != "uuid-d1" {
		t.Errorf("expected snapshot uuid 'uuid-d1', got %v", out[1]["uuid"])
	}
}

// ---------------------------------------------------------------------------
// Test: second flush accumulates across flushes (full-so-far)
// ---------------------------------------------------------------------------

func TestAccumulateStreamEvents_AcrossFlushes(t *testing.T) {
	state := NewStreamAccumulator()

	// Flush 1: message_start + first chunk.
	buf1 := []map[string]any{
		{
			"type":               "stream_event",
			"uuid":               "uuid-start",
			"session_id":         "s1",
			"parent_tool_use_id": "",
			"event": map[string]any{
				"type":    "message_start",
				"message": map[string]any{"id": "msg-1"},
			},
		},
		{
			"type":               "stream_event",
			"uuid":               "uuid-d1",
			"session_id":         "s1",
			"parent_tool_use_id": "",
			"event": map[string]any{
				"type":  "content_block_delta",
				"index": float64(0),
				"delta": map[string]any{"type": "text_delta", "text": "A"},
			},
		},
	}
	AccumulateStreamEvents(buf1, state)

	// Flush 2: another chunk for the same block.
	buf2 := []map[string]any{
		{
			"type":               "stream_event",
			"uuid":               "uuid-d2",
			"session_id":         "s1",
			"parent_tool_use_id": "",
			"event": map[string]any{
				"type":  "content_block_delta",
				"index": float64(0),
				"delta": map[string]any{"type": "text_delta", "text": "B"},
			},
		},
	}
	out2 := AccumulateStreamEvents(buf2, state)

	if len(out2) != 1 {
		t.Fatalf("expected 1 event in second flush, got %d", len(out2))
	}
	evt, _ := out2[0]["event"].(map[string]any)
	delta, _ := evt["delta"].(map[string]any)
	text, _ := delta["text"].(string)
	if text != "AB" {
		t.Errorf("expected full-so-far text 'AB', got %q", text)
	}
}

// ---------------------------------------------------------------------------
// Test: non-text-delta events pass through unchanged
// ---------------------------------------------------------------------------

func TestAccumulateStreamEvents_NonTextDeltaPassthrough(t *testing.T) {
	state := NewStreamAccumulator()

	buffer := []map[string]any{
		{
			"type":               "stream_event",
			"uuid":               "uuid-1",
			"session_id":         "s1",
			"parent_tool_use_id": "",
			"event": map[string]any{
				"type": "content_block_stop",
			},
		},
	}

	out := AccumulateStreamEvents(buffer, state)
	if len(out) != 1 {
		t.Fatalf("expected 1 passthrough event, got %d", len(out))
	}
	if out[0]["uuid"] != "uuid-1" {
		t.Errorf("passthrough event uuid mismatch")
	}
}

// ---------------------------------------------------------------------------
// Test: ClearStreamAccumulatorForMessage cleans up
// ---------------------------------------------------------------------------

func TestClearStreamAccumulatorForMessage(t *testing.T) {
	state := NewStreamAccumulator()

	// Set up state.
	state.ByMessage["msg-1"] = [][]string{{"hello"}}
	state.ScopeToMessage["s1:"] = "msg-1"

	ClearStreamAccumulatorForMessage(state, "s1", "", "msg-1")

	if _, ok := state.ByMessage["msg-1"]; ok {
		t.Error("expected msg-1 to be cleared from ByMessage")
	}
	if _, ok := state.ScopeToMessage["s1:"]; ok {
		t.Error("expected scope to be cleared from ScopeToMessage")
	}
}

// ---------------------------------------------------------------------------
// Test: delta without preceding message_start passes through raw
// ---------------------------------------------------------------------------

func TestAccumulateStreamEvents_DeltaWithoutMessageStart(t *testing.T) {
	state := NewStreamAccumulator()

	buffer := []map[string]any{
		{
			"type":               "stream_event",
			"uuid":               "uuid-orphan",
			"session_id":         "s1",
			"parent_tool_use_id": "",
			"event": map[string]any{
				"type":  "content_block_delta",
				"index": float64(0),
				"delta": map[string]any{"type": "text_delta", "text": "orphan"},
			},
		},
	}

	out := AccumulateStreamEvents(buffer, state)
	if len(out) != 1 {
		t.Fatalf("expected 1 passthrough event, got %d", len(out))
	}
	// Should be the raw event, not a snapshot.
	if out[0]["uuid"] != "uuid-orphan" {
		t.Error("expected raw passthrough for orphan delta")
	}
}
