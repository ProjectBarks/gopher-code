// L2: Error recovery scenario tests.
//
// These tests define the expected behavior for P0 error recovery capabilities
// that are NOT yet implemented. They serve as behavioral contracts -- remove
// the expected-failure comments as each feature lands.
//
// Capability mapping:
//   - 1.5  ContextTooLong recovery
//   - 1.6  MaxOutputTokens auto-continue
//   - 3.2  Stream error classification (429, 5xx, auth)
//   - (bonus) Malformed tool JSON graceful fallback

package query_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func setupRegistry() (*tools.ToolRegistry, *tools.ToolOrchestrator) {
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)
	return reg, orch
}

func setupRegistryWithTool(spy *testharness.SpyTool) (*tools.ToolRegistry, *tools.ToolOrchestrator) {
	reg := tools.NewRegistry()
	reg.Register(spy)
	orch := tools.NewOrchestrator(reg)
	return reg, orch
}

// ---------------------------------------------------------------------------
// TestL2ErrorRecovery
// ---------------------------------------------------------------------------

func TestL2ErrorRecovery(t *testing.T) {
	// 1.5 ContextTooLong recovery ----------------------------------------

	t.Run("context_too_long_triggers_compact", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(fmt.Errorf("context_too_long")),
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()

		sess := testharness.MakeSession()
		// Pad the session with enough messages to be compactable.
		for i := 0; i < 20; i++ {
			sess.PushMessage(message.UserMessage(fmt.Sprintf("message %d", i)))
			sess.PushMessage(message.Message{
				Role:    message.RoleAssistant,
				Content: []message.ContentBlock{message.TextBlock(fmt.Sprintf("reply %d", i))},
			})
		}

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err != nil {
			t.Fatalf("should recover via compact, got: %v", err)
		}

		reqs := prov.Requests()
		if len(reqs) < 2 {
			t.Fatalf("expected at least 2 requests, got %d", len(reqs))
		}
		if len(reqs[1].Messages) >= len(reqs[0].Messages) {
			t.Fatalf("compacted request should have fewer messages: second=%d first=%d",
				len(reqs[1].Messages), len(reqs[0].Messages))
		}
	})

	t.Run("context_too_long_after_compact_fails", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(fmt.Errorf("context_too_long")),
			testharness.MakeErrorTurn(fmt.Errorf("context_too_long")),
		)
		reg, orch := setupRegistry()

		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		var agentErr *query.AgentError
		if !errors.As(err, &agentErr) {
			t.Fatalf("expected AgentError, got %T: %v", err, err)
		}
		if agentErr.Kind != query.ErrContextTooLong {
			t.Fatalf("expected ErrContextTooLong, got kind=%d", agentErr.Kind)
		}
	})

	// 1.6 MaxOutputTokens auto-continue ----------------------------------

	t.Run("max_output_tokens_auto_continue", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			// Turn 1: model is cut off mid-text
			testharness.MakeTextTurn("partial response...", provider.StopReasonMaxTokens),
			// Turn 2: model continues after auto-injected prompt
			testharness.MakeTextTurn(" and here is the rest.", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()
		callback, _ := testharness.NewEventCollector()

		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, callback)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// The session should contain the auto-continue user message between
		// the two assistant messages. Matches TS source query.ts:1226-1227.
		foundContinue := false
		for _, m := range sess.Messages {
			if m.Role == message.RoleUser {
				for _, b := range m.Content {
					if b.Type == message.ContentText && strings.Contains(b.Text, "Output token limit hit") {
						foundContinue = true
					}
				}
			}
		}
		if !foundContinue {
			t.Fatal("expected a user message containing 'Output token limit hit' for auto-continue")
		}

		reqs := prov.Requests()
		if len(reqs) != 2 {
			t.Fatalf("expected 2 requests, got %d", len(reqs))
		}
		// The second request's last message should be the auto-injected user message.
		lastMsg := reqs[1].Messages[len(reqs[1].Messages)-1]
		if lastMsg.Role != "user" {
			t.Fatalf("expected last message in second request to be user, got %q", lastMsg.Role)
		}
	})

	t.Run("max_output_tokens_with_tool_use", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			// MaxTokens but with a tool call -- tool execution takes priority.
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{"action":"do_thing"}`), provider.StopReasonMaxTokens),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistryWithTool(spy)

		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if spy.CallCount() == 0 {
			t.Fatal("expected tool to be executed (CallCount > 0)")
		}
	})

	// 3.2 Stream error classification ------------------------------------

	t.Run("stream_error_429_retries", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(fmt.Errorf("429 Too Many Requests")),
			testharness.MakeErrorTurn(fmt.Errorf("429 rate limit")),
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()

		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err != nil {
			t.Fatalf("should succeed after retrying 429, got: %v", err)
		}

		reqs := prov.Requests()
		if len(reqs) != 3 {
			t.Fatalf("expected 3 requests (2 retries + 1 success), got %d", len(reqs))
		}
	})

	t.Run("stream_error_5xx_retries", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(fmt.Errorf("500 internal server error")),
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()

		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err != nil {
			t.Fatalf("should succeed after retrying 5xx, got: %v", err)
		}

		reqs := prov.Requests()
		if len(reqs) != 2 {
			t.Fatalf("expected 2 requests (1 retry + 1 success), got %d", len(reqs))
		}
	})

	t.Run("stream_error_auth_fails_fast", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(fmt.Errorf("401 unauthorized")),
			// This turn should never be consumed.
			testharness.MakeTextTurn("should not reach", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()

		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err == nil {
			t.Fatal("expected an error for 401, got nil")
		}

		var agentErr *query.AgentError
		if !errors.As(err, &agentErr) {
			t.Fatalf("expected AgentError, got %T: %v", err, err)
		}
		if agentErr.Kind != query.ErrProvider {
			t.Fatalf("expected ErrProvider, got kind=%d", agentErr.Kind)
		}

		reqs := prov.Requests()
		if len(reqs) != 1 {
			t.Fatalf("expected 1 request (no retry), got %d", len(reqs))
		}
	})

	// Bonus: malformed tool JSON graceful fallback -----------------------

	t.Run("malformed_tool_json_graceful", func(t *testing.T) {
		spy := testharness.NewSpyTool("bash", false)

		stopToolUse := provider.StopReasonToolUse

		malformedTurn := testharness.TurnScript{Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{
				Type: provider.EventContentBlockStart,
				Content: &provider.ResponseContent{
					Type:  "tool_use",
					ID:    "t1",
					Name:  "bash",
					Input: json.RawMessage(`{}`),
				},
			}},
			{Event: &provider.StreamEvent{
				Type:        provider.EventInputJsonDelta,
				PartialJSON: "{not valid json}",
			}},
			{Event: &provider.StreamEvent{
				Type: provider.EventMessageDone,
				Response: &provider.ModelResponse{
					ID: "resp-1",
					Content: []provider.ResponseContent{{
						Type:  "tool_use",
						ID:    "t1",
						Name:  "bash",
						Input: json.RawMessage(`{}`),
					}},
					StopReason: &stopToolUse,
					Usage:      provider.Usage{},
				},
			}},
		}}

		prov := testharness.NewScriptedProvider(
			malformedTurn,
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		reg, orch := setupRegistryWithTool(spy)

		sess := testharness.MakeSession()

		// Should not panic -- invalid JSON falls back to {}.
		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err != nil {
			t.Fatalf("expected no error (graceful fallback), got: %v", err)
		}

		if spy.CallCount() == 0 {
			t.Fatal("expected tool to be called (graceful fallback to empty object)")
		}
	})
}
