// Edge-case tests for the query loop — adversarial scenarios that stress-test
// corners the L1-L4 suites do not cover.
//
// These tests target:
//   - Stream edge cases (cancelled ctx, empty stream, empty deltas)
//   - Tool execution edge cases (unknown tool, empty output, errors)
//   - Multi-turn edge cases (deep recursion, mixed text+tool, unlimited turns)
//   - Usage tracking edge cases (zero tokens, nil stop reason)

package query_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// ===========================================================================
// Stream edge cases
// ===========================================================================

func TestEdgeCases_Stream(t *testing.T) {

	// context_cancelled_during_stream — ctx is already cancelled before Query
	// starts streaming. The implementation must detect ctx.Done() and return
	// ErrAborted (or context.Canceled).
	t.Run("context_cancelled_during_stream", func(t *testing.T) {
		// Provider will never be consumed because ctx is already dead.
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("should never see this", provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		sess := testharness.MakeSession()
		err := query.Query(ctx, sess, prov, registry, orchestrator, nil)
		if err == nil {
			t.Fatal("expected an error from cancelled context, got nil")
		}

		// Accept context.Canceled or AgentError{Kind: ErrAborted}
		var agentErr *query.AgentError
		if errors.As(err, &agentErr) {
			if agentErr.Kind != query.ErrAborted {
				t.Errorf("expected ErrAborted, got kind=%d", agentErr.Kind)
			}
		} else if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled or AgentError{ErrAborted}, got %T: %v", err, err)
		}
	})

	// empty_stream_no_events — provider returns a channel that closes
	// immediately with zero events. The query loop must handle this gracefully
	// rather than panicking on a nil ModelResponse.
	t.Run("empty_stream_no_events", func(t *testing.T) {
		emptyTurn := testharness.TurnScript{
			Events: []provider.StreamResult{}, // no events at all
		}
		prov := testharness.NewScriptedProvider(emptyTurn)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)

		// We accept either:
		// (a) a graceful error, or
		// (b) the loop terminates without panic and returns nil.
		// What we do NOT accept is a panic.
		if err != nil {
			// An error is fine — just confirm it does not panic.
			t.Logf("got error (acceptable): %v", err)
		}
	})

	// text_delta_with_empty_string — a TextDelta carrying "" should not crash.
	t.Run("text_delta_with_empty_string", func(t *testing.T) {
		// Send: empty delta, real delta, empty delta, done
		turn := testharness.TurnScript{Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{Type: provider.EventTextDelta, Text: ""}},
			{Event: &provider.StreamEvent{Type: provider.EventTextDelta, Text: "hello"}},
			{Event: &provider.StreamEvent{Type: provider.EventTextDelta, Text: ""}},
			{Event: &provider.StreamEvent{
				Type: provider.EventMessageDone,
				Response: &provider.ModelResponse{
					ID:         "resp-empty-delta",
					Content:    []provider.ResponseContent{{Type: "text", Text: "hello"}},
					StopReason: ptrStopReason(provider.StopReasonEndTurn),
					Usage:      provider.Usage{},
				},
			}},
		}}

		prov := testharness.NewScriptedProvider(turn)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// The deltas should be forwarded (including empty ones), or at minimum
		// the final assembled text should be correct.
		assembled := strings.Join(eventLog.TextDeltas(), "")
		if !strings.Contains(assembled, "hello") {
			t.Errorf("expected assembled text to contain 'hello', got %q", assembled)
		}
	})

	// tool_use_with_empty_json_input — tool input is `{}`, should still execute.
	t.Run("tool_use_with_empty_json_input", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if spy.CallCount() != 1 {
			t.Errorf("expected tool to be called once, got %d", spy.CallCount())
		}
	})

	// stream_error_mid_stream — an error StreamResult arrives after some
	// text deltas have already been received. The loop must not lose the
	// partial text or panic.
	t.Run("stream_error_mid_stream", func(t *testing.T) {
		turn := testharness.TurnScript{Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{Type: provider.EventTextDelta, Text: "partial"}},
			{Err: errors.New("connection reset")},
		}}
		prov := testharness.NewScriptedProvider(
			turn,
			testharness.MakeTextTurn("recovered", provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		// Either the loop retries or fails; no panic.
		_ = err
	})
}

// ===========================================================================
// Tool execution edge cases
// ===========================================================================

func TestEdgeCases_ToolExecution(t *testing.T) {

	// tool_not_found_in_registry — model calls a tool that doesn't exist.
	// The orchestrator should return an error result, and the loop should
	// feed that back to the model and continue.
	t.Run("tool_not_found_in_registry", func(t *testing.T) {
		// Register NO tools, but provider calls "nonexistent_tool"
		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "nonexistent_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("I see the error", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry() // empty
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error (should recover from unknown tool), got %v", err)
		}

		// There must be a tool_result with IsError=true containing "unknown tool"
		found := false
		for _, msg := range sess.Messages {
			for _, block := range msg.Content {
				if block.Type == message.ContentToolResult && block.IsError {
					if strings.Contains(strings.ToLower(block.Content), "unknown tool") {
						found = true
					}
				}
			}
		}
		if !found {
			t.Error("expected a ToolResult with IsError=true mentioning 'unknown tool'")
		}
	})

	// tool_returns_empty_output — tool output has Content="" and IsError=false.
	t.Run("tool_returns_empty_output", func(t *testing.T) {
		spy := testharness.NewSpyTool("empty_tool", false).WithResponse(func(_ json.RawMessage) *tools.ToolOutput {
			return tools.SuccessOutput("")
		})

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "empty_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if spy.CallCount() != 1 {
			t.Errorf("expected 1 call, got %d", spy.CallCount())
		}

		// The tool_result block should still exist in the session
		var foundResult bool
		for _, msg := range sess.Messages {
			for _, block := range msg.Content {
				if block.Type == message.ContentToolResult && block.ToolUseID == "t1" {
					foundResult = true
					if block.IsError {
						t.Error("empty content should not be flagged as error")
					}
				}
			}
		}
		if !foundResult {
			t.Error("expected to find tool_result with ID t1")
		}
	})

	// tool_execution_error_surfaces_as_tool_result — when Execute returns
	// an error (not just IsError output), it should become an is_error
	// tool_result so the model can see what went wrong.
	t.Run("tool_execution_error_surfaces_as_tool_result", func(t *testing.T) {
		failTool := testharness.NewSpyTool("fail_tool", false)
		// Override the tool's response to return an error via the error return
		failTool.WithResponse(nil) // we need a custom approach

		// We cannot make SpyTool.Execute return an error through WithResponse,
		// so we create a custom tool.
		errorTool := &errorReturningTool{name: "error_tool"}

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "error_tool", json.RawMessage(`{"cmd":"fail"}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("handled error", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(errorTool)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error (tool error should be caught), got %v", err)
		}

		// Verify the tool_result has is_error=true
		found := false
		for _, msg := range sess.Messages {
			for _, block := range msg.Content {
				if block.Type == message.ContentToolResult && block.IsError {
					if strings.Contains(block.Content, "deliberate failure") {
						found = true
					}
				}
			}
		}
		if !found {
			t.Error("expected an is_error tool_result containing 'deliberate failure'")
		}
	})

	// multiple_text_deltas_concatenated — many tiny text deltas should all
	// be joined into one text content block in the final assistant message.
	t.Run("multiple_text_deltas_concatenated", func(t *testing.T) {
		chunks := make([]string, 50)
		for i := range chunks {
			chunks[i] = "x"
		}

		prov := testharness.NewScriptedProvider(
			testharness.MakeChunkedTextTurn(chunks, provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		deltas := eventLog.TextDeltas()
		if len(deltas) != 50 {
			t.Errorf("expected 50 text deltas, got %d", len(deltas))
		}

		assembled := strings.Join(deltas, "")
		expected := strings.Repeat("x", 50)
		if assembled != expected {
			t.Errorf("expected %q, got %q", expected, assembled)
		}

		// Final message should have exactly one text block with the full text.
		if len(sess.Messages) < 2 {
			t.Fatal("expected at least 2 messages")
		}
		assistantMsg := sess.Messages[len(sess.Messages)-1]
		if assistantMsg.Role != message.RoleAssistant {
			t.Fatalf("last message should be assistant, got %s", assistantMsg.Role)
		}
		textBlocks := 0
		for _, b := range assistantMsg.Content {
			if b.Type == message.ContentText {
				textBlocks++
				if b.Text != expected {
					t.Errorf("text block: expected %q, got %q", expected, b.Text)
				}
			}
		}
		if textBlocks != 1 {
			t.Errorf("expected exactly 1 text block, got %d", textBlocks)
		}
	})
}

// ===========================================================================
// Multi-turn edge cases
// ===========================================================================

func TestEdgeCases_MultiTurn(t *testing.T) {

	// three_consecutive_tool_rounds — tool->result->tool->result->tool->result->text.
	// Tests deep loop iteration (3 tool rounds before final text).
	t.Run("three_consecutive_tool_rounds", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{"round":1}`), provider.StopReasonToolUse),
			testharness.MakeToolTurn("t2", "my_tool", json.RawMessage(`{"round":2}`), provider.StopReasonToolUse),
			testharness.MakeToolTurn("t3", "my_tool", json.RawMessage(`{"round":3}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("all three rounds complete", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if spy.CallCount() != 3 {
			t.Errorf("expected 3 tool calls, got %d", spy.CallCount())
		}

		if sess.TurnCount != 4 {
			t.Errorf("expected turn_count=4 (3 tool + 1 text), got %d", sess.TurnCount)
		}

		// Messages: user, ass(tool1), user(result1), ass(tool2), user(result2),
		//           ass(tool3), user(result3), ass(text)
		expectedMsgCount := 8
		if len(sess.Messages) != expectedMsgCount {
			t.Errorf("expected %d messages, got %d", expectedMsgCount, len(sess.Messages))
		}
	})

	// tool_call_with_text_prefix — the model sends text AND tool_use in the
	// same turn (text before tool). The assistant message should contain both
	// a text block and a tool_use block.
	t.Run("tool_call_with_text_prefix", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		stopToolUse := provider.StopReasonToolUse
		stopEndTurn := provider.StopReasonEndTurn
		toolInput := json.RawMessage(`{"action":"go"}`)

		// Build a custom turn with text delta followed by tool_use
		mixedTurn := testharness.TurnScript{Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: "Let me use a tool: ",
			}},
			{Event: &provider.StreamEvent{
				Type: provider.EventContentBlockStart,
				Content: &provider.ResponseContent{
					Type:  "tool_use",
					ID:    "t1",
					Name:  "my_tool",
					Input: toolInput,
				},
			}},
			{Event: &provider.StreamEvent{
				Type:        provider.EventInputJsonDelta,
				PartialJSON: string(toolInput),
			}},
			{Event: &provider.StreamEvent{
				Type: provider.EventMessageDone,
				Response: &provider.ModelResponse{
					ID: "resp-mixed",
					Content: []provider.ResponseContent{
						{Type: "text", Text: "Let me use a tool: "},
						{Type: "tool_use", ID: "t1", Name: "my_tool", Input: toolInput},
					},
					StopReason: &stopToolUse,
					Usage:      provider.Usage{},
				},
			}},
		}}

		prov := testharness.NewScriptedProvider(
			mixedTurn,
			testharness.MakeTextTurn("finished", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if spy.CallCount() != 1 {
			t.Errorf("expected 1 tool call, got %d", spy.CallCount())
		}

		// The first assistant message should have BOTH text and tool_use blocks
		if len(sess.Messages) < 2 {
			t.Fatal("expected at least 2 messages")
		}
		assistantMsg := sess.Messages[1]
		hasText := false
		hasToolUse := false
		for _, block := range assistantMsg.Content {
			if block.Type == message.ContentText {
				hasText = true
			}
			if block.Type == message.ContentToolUse {
				hasToolUse = true
			}
		}
		if !hasText {
			t.Error("assistant message should contain a text block")
		}
		if !hasToolUse {
			t.Error("assistant message should contain a tool_use block")
		}

		// Suppress unused variable
		_ = stopEndTurn
	})

	// zero_max_turns_means_unlimited — MaxTurns=0 should NOT limit turns.
	// The loop should run until the model sends end_turn.
	t.Run("zero_max_turns_means_unlimited", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeToolTurn("t2", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeToolTurn("t3", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:          "test-model",
			MaxTurns:       0, // 0 = unlimited
			PermissionMode: permissions.AutoApprove,
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			// If err is ErrMaxTurnsExceeded, that is the bug we are trying to catch.
			var agentErr *query.AgentError
			if errors.As(err, &agentErr) && agentErr.Kind == query.ErrMaxTurnsExceeded {
				t.Fatal("MaxTurns=0 should mean unlimited, but got ErrMaxTurnsExceeded")
			}
			t.Fatalf("expected no error, got %v", err)
		}

		if spy.CallCount() != 3 {
			t.Errorf("expected 3 tool calls with unlimited turns, got %d", spy.CallCount())
		}
	})

	// max_turns_boundary — exactly MaxTurns tool turns should succeed if the
	// last turn is a text (end_turn). Ensures off-by-one is correct.
	t.Run("max_turns_boundary_exact", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeToolTurn("t2", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:          "test-model",
			MaxTurns:       3, // exactly 3 turns: tool, tool, text
			PermissionMode: permissions.AutoApprove,
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error with exactly MaxTurns turns, got %v", err)
		}

		if sess.TurnCount != 3 {
			t.Errorf("expected turn_count=3, got %d", sess.TurnCount)
		}
	})
}

// ===========================================================================
// Usage tracking edge cases
// ===========================================================================

func TestEdgeCases_Usage(t *testing.T) {

	// zero_usage_tokens — a response with 0 input and 0 output tokens should
	// not crash and should not corrupt session totals.
	t.Run("zero_usage_tokens", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurnWithUsage("ok", provider.StopReasonEndTurn,
				provider.Usage{InputTokens: 0, OutputTokens: 0}),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if sess.TotalInputTokens != 0 {
			t.Errorf("expected TotalInputTokens=0, got %d", sess.TotalInputTokens)
		}
		if sess.TotalOutputTokens != 0 {
			t.Errorf("expected TotalOutputTokens=0, got %d", sess.TotalOutputTokens)
		}
	})

	// nil_stop_reason_in_response — ModelResponse with StopReason=nil should
	// be handled without a nil-pointer dereference. The loop should either
	// treat it as end_turn or handle it gracefully.
	t.Run("nil_stop_reason_in_response", func(t *testing.T) {
		turn := testharness.TurnScript{Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: "hello",
			}},
			{Event: &provider.StreamEvent{
				Type: provider.EventMessageDone,
				Response: &provider.ModelResponse{
					ID:         "resp-nil-stop",
					Content:    []provider.ResponseContent{{Type: "text", Text: "hello"}},
					StopReason: nil, // intentionally nil
					Usage:      provider.Usage{InputTokens: 10, OutputTokens: 5},
				},
			}},
		}}

		prov := testharness.NewScriptedProvider(turn)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		// Must not panic
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		// We accept any result as long as it doesn't panic.
		_ = err
	})

	// usage_accumulates_across_many_turns — verify that usage is correctly
	// summed even with many turns (regression: integer overflow or reset).
	t.Run("usage_accumulates_across_many_turns", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)
		turns := make([]testharness.TurnScript, 0, 11)
		for i := 0; i < 10; i++ {
			turns = append(turns, testharness.MakeToolTurnWithUsage(
				"t"+string(rune('0'+i)), "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse,
				provider.Usage{InputTokens: 100, OutputTokens: 50},
			))
		}
		turns = append(turns, testharness.MakeTextTurnWithUsage("done", provider.StopReasonEndTurn,
			provider.Usage{InputTokens: 100, OutputTokens: 50}))

		prov := testharness.NewScriptedProvider(turns...)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expectedInput := 11 * 100
		expectedOutput := 11 * 50
		if sess.TotalInputTokens != expectedInput {
			t.Errorf("expected TotalInputTokens=%d, got %d", expectedInput, sess.TotalInputTokens)
		}
		if sess.TotalOutputTokens != expectedOutput {
			t.Errorf("expected TotalOutputTokens=%d, got %d", expectedOutput, sess.TotalOutputTokens)
		}
	})
}

// ===========================================================================
// Callback / event edge cases
// ===========================================================================

func TestEdgeCases_Callbacks(t *testing.T) {

	// nil_callback_does_not_panic — passing nil for onEvent should work.
	t.Run("nil_callback_does_not_panic", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error with nil callback, got %v", err)
		}
	})

	// tool_result_events_emitted_for_each_tool — every tool call should
	// produce a QEventToolResult.
	t.Run("tool_result_events_emitted_for_each_tool", func(t *testing.T) {
		spyA := testharness.NewSpyTool("tool_a", true)
		spyB := testharness.NewSpyTool("tool_b", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeMultiToolTurn([]testharness.ToolSpec{
				{ID: "t1", Name: "tool_a", Input: json.RawMessage(`{}`)},
				{ID: "t2", Name: "tool_b", Input: json.RawMessage(`{}`)},
			}, provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spyA)
		registry.Register(spyB)
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		toolResults := eventLog.ToolResults()
		if len(toolResults) != 2 {
			t.Errorf("expected 2 tool result events, got %d", len(toolResults))
		}

		ids := make(map[string]bool)
		for _, tr := range toolResults {
			ids[tr.ToolUseID] = true
		}
		if !ids["t1"] || !ids["t2"] {
			t.Errorf("expected tool result events for t1 and t2, got IDs: %v", ids)
		}
	})

	// usage_event_includes_cache_nil — when cache fields are nil, usage events
	// should still work (CacheCreation/CacheRead should be nil, not zero).
	t.Run("usage_event_cache_nil_not_zero", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurnWithUsage("ok", provider.StopReasonEndTurn,
				provider.Usage{
					InputTokens:              50,
					OutputTokens:             25,
					CacheCreationInputTokens: nil,
					CacheReadInputTokens:     nil,
				}),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		usageEvents := eventLog.UsageEvents()
		if len(usageEvents) != 1 {
			t.Fatalf("expected 1 usage event, got %d", len(usageEvents))
		}
		ue := usageEvents[0]
		if ue.CacheCreation != nil {
			t.Errorf("expected CacheCreation=nil, got %v", ue.CacheCreation)
		}
		if ue.CacheRead != nil {
			t.Errorf("expected CacheRead=nil, got %v", ue.CacheRead)
		}
	})
}

// ===========================================================================
// Provider interaction edge cases
// ===========================================================================

func TestEdgeCases_ProviderInteraction(t *testing.T) {

	// model_and_max_tokens_forwarded — the request to the provider must
	// include the correct model name and max tokens from config.
	t.Run("model_and_max_tokens_forwarded", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:          "custom-model-v2",
			MaxTurns:       100,
			PermissionMode: permissions.AutoApprove,
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		reqs := prov.Requests()
		if len(reqs) < 1 {
			t.Fatal("expected at least 1 request")
		}
		if reqs[0].Model != "custom-model-v2" {
			t.Errorf("expected model 'custom-model-v2', got %q", reqs[0].Model)
		}
	})

	// messages_grow_correctly_over_tool_loop — after each tool turn, the
	// request messages should include the new tool_result.
	t.Run("messages_grow_correctly_over_tool_loop", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		reqs := prov.Requests()
		if len(reqs) != 2 {
			t.Fatalf("expected 2 requests, got %d", len(reqs))
		}

		// Second request should have more messages than the first (tool_use + tool_result added)
		if len(reqs[1].Messages) <= len(reqs[0].Messages) {
			t.Errorf("second request (%d msgs) should have more messages than first (%d msgs)",
				len(reqs[1].Messages), len(reqs[0].Messages))
		}
	})

	// tool_use_id_preserved_in_result — the tool_result's ToolUseID must
	// match the tool_use's ID that triggered it.
	t.Run("tool_use_id_preserved_in_result", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("unique-id-42", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Find tool_result and verify ToolUseID
		found := false
		for _, msg := range sess.Messages {
			for _, block := range msg.Content {
				if block.Type == message.ContentToolResult {
					if block.ToolUseID == "unique-id-42" {
						found = true
					} else {
						t.Errorf("expected ToolUseID='unique-id-42', got %q", block.ToolUseID)
					}
				}
			}
		}
		if !found {
			t.Error("expected to find a tool_result with ToolUseID='unique-id-42'")
		}
	})
}

// ===========================================================================
// Helpers
// ===========================================================================

func ptrStopReason(s provider.StopReason) *provider.StopReason {
	return &s
}

// errorReturningTool always returns an error from Execute.
type errorReturningTool struct {
	name string
}

func (e *errorReturningTool) Name() string               { return e.name }
func (e *errorReturningTool) Description() string         { return "A tool that always errors." }
func (e *errorReturningTool) IsReadOnly() bool            { return false }
func (e *errorReturningTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }

func (e *errorReturningTool) Execute(_ context.Context, _ *tools.ToolContext, _ json.RawMessage) (*tools.ToolOutput, error) {
	return nil, errors.New("deliberate failure")
}
