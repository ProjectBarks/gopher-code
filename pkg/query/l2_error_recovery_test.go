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

func TestFallbackModelSwitch(t *testing.T) {
	// Source: query.ts:894-951 — FallbackTriggeredError switches model and retries
	// Source: services/api/withRetry.ts:160-168 — FallbackTriggeredError definition

	t.Run("switches_model_on_fallback_error", func(t *testing.T) {
		// Turn 1: provider returns FallbackTriggeredError
		// Turn 2: provider succeeds with fallback model
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(&query.FallbackTriggeredError{
				OriginalModel: "claude-opus-4-20250514",
				FallbackModel: "claude-sonnet-4-20250514",
			}),
			testharness.MakeTextTurn("ok from fallback", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		sess.Config.Model = "claude-opus-4-20250514"
		sess.Config.FallbackModel = "claude-sonnet-4-20250514"

		var events []query.QueryEvent
		callback := func(evt query.QueryEvent) {
			events = append(events, evt)
		}

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Model should have been switched
		if sess.Config.Model != "claude-sonnet-4-20250514" {
			t.Errorf("expected model to be switched to fallback, got %s", sess.Config.Model)
		}

		// Should have emitted a switch notification
		found := false
		for _, evt := range events {
			if evt.Type == query.QEventTextDelta && strings.Contains(evt.Text, "Switched to") {
				found = true
			}
		}
		if !found {
			t.Error("expected a 'Switched to' notification event")
		}

		// Second request should use the fallback model
		requests := prov.CapturedRequests
		if len(requests) < 2 {
			t.Fatalf("expected 2 requests, got %d", len(requests))
		}
		if requests[1].Model != "claude-sonnet-4-20250514" {
			t.Errorf("second request should use fallback model, got %s", requests[1].Model)
		}
	})

	t.Run("no_fallback_without_config", func(t *testing.T) {
		// Without FallbackModel configured, FallbackTriggeredError is a regular error
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(&query.FallbackTriggeredError{
				OriginalModel: "claude-opus-4-20250514",
				FallbackModel: "claude-sonnet-4-20250514",
			}),
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		sess.Config.Model = "claude-opus-4-20250514"
		sess.Config.FallbackModel = "" // No fallback configured

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err == nil {
			t.Fatal("expected error when no fallback configured")
		}
	})
}

// TestTypedAPIErrorIntegration exercises the typed *provider.APIError classification
// path through the query loop, ensuring ClassifyHTTPError, UserFacingMessage, and
// the typed error helpers are wired into the real code path.
// Source: services/api/errors.ts — full error classification
func TestTypedAPIErrorIntegration(t *testing.T) {

	t.Run("typed_429_retries_and_succeeds", func(t *testing.T) {
		// Use a real classified APIError, not a plain fmt.Errorf
		apiErr429 := provider.ClassifyHTTPError(429, []byte(`{"type":"rate_limit_error"}`), "")
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(apiErr429),
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()
		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err != nil {
			t.Fatalf("should succeed after retrying typed 429 APIError, got: %v", err)
		}

		reqs := prov.Requests()
		if len(reqs) != 2 {
			t.Fatalf("expected 2 requests (1 retry + 1 success), got %d", len(reqs))
		}
	})

	t.Run("typed_529_retries_limited_to_3", func(t *testing.T) {
		// 529 has a lower retry limit (Max529Retries=3)
		apiErr529 := provider.ClassifyHTTPError(529, []byte(`{"type":"overloaded_error"}`), "")
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(apiErr529),
			testharness.MakeErrorTurn(apiErr529),
			testharness.MakeErrorTurn(apiErr529),
			testharness.MakeErrorTurn(apiErr529), // 4th attempt — should exceed limit
		)
		reg, orch := setupRegistry()
		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err == nil {
			t.Fatal("expected error after exceeding 529 retry limit")
		}

		var agentErr *query.AgentError
		if !errors.As(err, &agentErr) {
			t.Fatalf("expected AgentError, got %T: %v", err, err)
		}
		if agentErr.Kind != query.ErrProvider {
			t.Fatalf("expected ErrProvider, got kind=%d", agentErr.Kind)
		}
	})

	t.Run("typed_401_auth_fails_immediately", func(t *testing.T) {
		apiErr401 := provider.ClassifyHTTPError(401, []byte(`invalid x-api-key`), "")
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(apiErr401),
			testharness.MakeTextTurn("should not reach", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()
		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err == nil {
			t.Fatal("expected error for typed 401 APIError")
		}

		var agentErr *query.AgentError
		if !errors.As(err, &agentErr) {
			t.Fatalf("expected AgentError, got %T: %v", err, err)
		}
		if agentErr.Kind != query.ErrProvider {
			t.Fatalf("expected ErrProvider, got kind=%d", agentErr.Kind)
		}
		// User message should be populated from UserFacingMessage()
		if agentErr.UserMessage == "" {
			t.Fatal("expected UserMessage to be populated for typed APIError")
		}
		if agentErr.UserMessage != provider.InvalidAPIKeyErrorMessage {
			t.Errorf("UserMessage = %q, want %q", agentErr.UserMessage, provider.InvalidAPIKeyErrorMessage)
		}

		reqs := prov.Requests()
		if len(reqs) != 1 {
			t.Fatalf("expected 1 request (no retry for auth), got %d", len(reqs))
		}
	})

	t.Run("typed_prompt_too_long_compacts_and_retries", func(t *testing.T) {
		apiErrPrompt := provider.ClassifyHTTPError(400, []byte(`prompt is too long: 250000 tokens > 200000 maximum`), "")
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(apiErrPrompt),
			testharness.MakeTextTurn("ok after compact", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()
		sess := testharness.MakeSession()
		// Pad session for compaction
		for i := 0; i < 20; i++ {
			sess.PushMessage(message.UserMessage(fmt.Sprintf("msg %d", i)))
			sess.PushMessage(message.Message{
				Role:    message.RoleAssistant,
				Content: []message.ContentBlock{message.TextBlock(fmt.Sprintf("reply %d", i))},
			})
		}

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err != nil {
			t.Fatalf("should recover via compact for typed prompt_too_long, got: %v", err)
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

	t.Run("typed_credit_balance_low_fails_immediately", func(t *testing.T) {
		apiErrBilling := provider.ClassifyHTTPError(400, []byte(`Credit balance is too low`), "")
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(apiErrBilling),
			testharness.MakeTextTurn("should not reach", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()
		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err == nil {
			t.Fatal("expected error for billing error")
		}

		var agentErr *query.AgentError
		if !errors.As(err, &agentErr) {
			t.Fatalf("expected AgentError, got %T: %v", err, err)
		}
		if agentErr.UserMessage != provider.CreditBalanceTooLowErrorMessage {
			t.Errorf("UserMessage = %q, want %q", agentErr.UserMessage, provider.CreditBalanceTooLowErrorMessage)
		}

		reqs := prov.Requests()
		if len(reqs) != 1 {
			t.Fatalf("expected 1 request (no retry for billing), got %d", len(reqs))
		}
	})

	t.Run("typed_5xx_retries_and_succeeds", func(t *testing.T) {
		apiErr500 := provider.ClassifyHTTPError(500, []byte(`Internal Server Error`), "")
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(apiErr500),
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()
		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err != nil {
			t.Fatalf("should succeed after retrying typed 500 APIError, got: %v", err)
		}

		reqs := prov.Requests()
		if len(reqs) != 2 {
			t.Fatalf("expected 2 requests (1 retry + 1 success), got %d", len(reqs))
		}
	})

	t.Run("typed_token_revoked_fails_immediately", func(t *testing.T) {
		apiErr403 := provider.ClassifyHTTPError(403, []byte(`OAuth token has been revoked`), "")
		// Override: token_revoked is classified as retryable (for refresh), but in the
		// query loop auth errors should fail immediately.
		prov := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(apiErr403),
			testharness.MakeTextTurn("should not reach", provider.StopReasonEndTurn),
		)
		reg, orch := setupRegistry()
		sess := testharness.MakeSession()

		err := query.Query(context.Background(), sess, prov, reg, orch, nil)
		if err == nil {
			t.Fatal("expected error for token revoked")
		}

		var agentErr *query.AgentError
		if !errors.As(err, &agentErr) {
			t.Fatalf("expected AgentError, got %T: %v", err, err)
		}
		if agentErr.UserMessage != provider.TokenRevokedErrorMessage {
			t.Errorf("UserMessage = %q, want %q", agentErr.UserMessage, provider.TokenRevokedErrorMessage)
		}
	})
}
