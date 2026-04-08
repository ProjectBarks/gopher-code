// T477: Integration test exercising the provider package's retry budget,
// API logging, query source, and side query infrastructure through the
// real query code path.
//
// These tests verify that code in pkg/provider/ (retry_budget.go, logging.go,
// query_source.go, side_query.go) is reachable from the binary via the query
// loop and handler adapters.

package query_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/cmd/gopher-code/handlers"
	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// TestT477_RetryBudget529FallbackTrigger verifies that the query loop uses
// provider.RetryBudget to track consecutive 529 errors and trigger a model
// fallback when the budget is exhausted.
// Source: withRetry.ts:327-365
func TestT477_RetryBudget529FallbackTrigger(t *testing.T) {
	apiErr529 := provider.ClassifyHTTPError(529, []byte(`{"type":"overloaded_error"}`), "")

	// 3 consecutive 529 errors should exhaust the budget and trigger fallback.
	prov := testharness.NewScriptedProvider(
		testharness.MakeErrorTurn(apiErr529),
		testharness.MakeErrorTurn(apiErr529),
		testharness.MakeErrorTurn(apiErr529),
		testharness.MakeTextTurn("ok from fallback", provider.StopReasonEndTurn),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()
	sess.Config.Model = "claude-opus-4-20250514"
	sess.Config.FallbackModel = "claude-sonnet-4-20250514"
	// Must be a foreground source so 529 retries are attempted (not bailed immediately).
	sess.Config.QuerySource = provider.QuerySourceREPLMainThread

	var events []query.QueryEvent
	callback := func(evt query.QueryEvent) {
		events = append(events, evt)
	}

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}

	// The model should have been switched to the fallback
	if sess.Config.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want fallback %q", sess.Config.Model, "claude-sonnet-4-20250514")
	}

	// Should have emitted a switch notification
	found := false
	for _, evt := range events {
		if evt.Type == query.QEventTextDelta && strings.Contains(evt.Text, "Switched to") {
			found = true
		}
	}
	if !found {
		t.Error("expected a 'Switched to' notification event for 529 fallback")
	}
}

// TestT477_QuerySourcePropagation verifies that QuerySource set in
// SessionConfig flows through to the query loop and is accessible.
// Source: services/api/claude.ts:1066-1070
func TestT477_QuerySourcePropagation(t *testing.T) {
	prov := testharness.NewScriptedProvider(
		testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()
	sess.Config.QuerySource = provider.QuerySourceSDK

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the query source is still set (not cleared)
	if sess.Config.QuerySource != provider.QuerySourceSDK {
		t.Errorf("QuerySource = %q, want %q", sess.Config.QuerySource, provider.QuerySourceSDK)
	}
}

// TestT477_APILoggingCalled verifies that provider.LogAPIQuery and
// provider.LogAPISuccess are called during a successful query.
// We capture slog output to verify the structured log events.
// Source: logging.ts:171-233, 398-577
func TestT477_APILoggingCalled(t *testing.T) {
	// Capture slog output
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(oldDefault)

	prov := testharness.NewScriptedProvider(
		testharness.MakeTextTurnWithUsage("logged response", provider.StopReasonEndTurn, provider.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		}),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()
	sess.Config.QuerySource = provider.QuerySourceREPLMainThread

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := buf.String()

	// Verify LogAPIQuery was called
	if !strings.Contains(logOutput, "tengu_api_query") {
		t.Error("expected slog output to contain 'tengu_api_query' from LogAPIQuery")
	}

	// Verify LogAPISuccess was called
	if !strings.Contains(logOutput, "tengu_api_success") {
		t.Error("expected slog output to contain 'tengu_api_success' from LogAPISuccess")
	}
}

// TestT477_APIErrorLogging verifies that provider.LogAPIError is called
// when a stream request fails.
// Source: logging.ts:235-396
func TestT477_APIErrorLogging(t *testing.T) {
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(oldDefault)

	apiErr401 := provider.ClassifyHTTPError(401, []byte(`invalid x-api-key`), "")
	prov := testharness.NewScriptedProvider(
		testharness.MakeErrorTurn(apiErr401),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()

	_ = query.Query(context.Background(), sess, prov, registry, orchestrator, nil)

	logOutput := buf.String()

	// Verify LogAPIError was called
	if !strings.Contains(logOutput, "tengu_api_error") {
		t.Error("expected slog output to contain 'tengu_api_error' from LogAPIError")
	}
}

// TestT477_SideQueryAdapter verifies that handlers.NewProviderSideQuery
// bridges handler-level side queries to provider.QueryWithModel.
// Source: services/api/claude.ts:3300-3348
func TestT477_SideQueryAdapter(t *testing.T) {
	// Create a scripted provider that returns a simple text response
	prov := testharness.NewScriptedProvider(
		testharness.MakeTextTurn("critique result", provider.StopReasonEndTurn),
	)

	sideQuery := handlers.NewProviderSideQuery(prov)

	resp, err := sideQuery(context.Background(), handlers.SideQueryOptions{
		QuerySource: "auto_mode_critique",
		Model:       "test-model",
		System:      "You are a helpful assistant.",
		MaxTokens:   4096,
		Messages: []handlers.SideQueryMessage{
			{Role: "user", Content: "Please critique these rules."},
		},
	})
	if err != nil {
		t.Fatalf("SideQuery failed: %v", err)
	}

	if len(resp.Content) == 0 {
		t.Fatal("expected at least one content block in response")
	}

	foundText := false
	for _, block := range resp.Content {
		if block.Type == "text" && strings.Contains(block.Text, "critique result") {
			foundText = true
		}
	}
	if !foundText {
		t.Errorf("expected response to contain 'critique result', got %+v", resp.Content)
	}

	// Verify the provider received the request with correct model
	reqs := prov.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].System != "You are a helpful assistant." {
		t.Errorf("system prompt = %q, want %q", reqs[0].System, "You are a helpful assistant.")
	}
}

// TestT477_RetryBudgetExhaustion verifies that when the retry budget is
// fully exhausted (10 retries), the query loop stops retrying.
// Source: withRetry.ts:52 — DEFAULT_MAX_RETRIES = 10
func TestT477_RetryBudgetExhaustion(t *testing.T) {
	apiErr500 := provider.ClassifyHTTPError(500, []byte(`Internal Server Error`), "")

	// Create more error turns than the retry budget allows.
	// WithRetry has maxRetries+1 total attempts, so 12 errors will exhaust it.
	var turns []testharness.TurnScript
	for i := 0; i < 12; i++ {
		turns = append(turns, testharness.MakeErrorTurn(apiErr500))
	}
	// This turn should never be reached.
	turns = append(turns, testharness.MakeTextTurn("should not reach", provider.StopReasonEndTurn))

	prov := testharness.NewScriptedProvider(turns...)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err == nil {
		t.Fatal("expected error after retry budget exhaustion")
	}

	var agentErr *query.AgentError
	if !errors.As(err, &agentErr) {
		t.Fatalf("expected AgentError, got %T: %v", err, err)
	}

	// Should not have consumed all 12 error turns — budget should have stopped it.
	reqs := prov.Requests()
	if len(reqs) >= 12 {
		t.Errorf("expected retry budget to stop before 12 attempts, got %d", len(reqs))
	}
}

// TestT477_QuerySourceRetryPolicy verifies that non-foreground query sources
// are correctly identified by provider.IsAgenticQuerySource.
// Source: services/api/claude.ts:1066-1070
func TestT477_QuerySourceRetryPolicy(t *testing.T) {
	// Foreground sources should retry 529
	foreground := []provider.QuerySource{
		provider.QuerySourceREPLMainThread,
		provider.QuerySourceSDK,
		provider.QuerySourceHookAgent,
		provider.QuerySourceVerificationAgent,
	}
	for _, qs := range foreground {
		if !provider.IsAgenticQuerySource(qs) {
			t.Errorf("IsAgenticQuerySource(%q) = false, want true", qs)
		}
		if !provider.ShouldRetry529(qs) {
			t.Errorf("ShouldRetry529(%q) = false, want true", qs)
		}
	}

	// Background sources should NOT retry 529
	background := []provider.QuerySource{
		provider.QuerySourceCompact,
		provider.QuerySourceTitle,
		provider.QuerySourceMemory,
		provider.QuerySourceMicroCompact,
	}
	for _, qs := range background {
		if provider.IsAgenticQuerySource(qs) {
			t.Errorf("IsAgenticQuerySource(%q) = true, want false", qs)
		}
		if provider.ShouldRetry529(qs) {
			t.Errorf("ShouldRetry529(%q) = true, want false", qs)
		}
	}

	// Agent sub-queries
	agentSource := provider.QuerySource("agent:my_subquery")
	if !provider.IsAgenticQuerySource(agentSource) {
		t.Error("IsAgenticQuerySource('agent:my_subquery') = false, want true")
	}
}

// TestT477_QueryWithModelIntegration verifies provider.QueryWithModel
// through the real streaming path.
// Source: services/api/claude.ts:3300-3348
func TestT477_QueryWithModelIntegration(t *testing.T) {
	prov := testharness.NewScriptedProvider(
		testharness.MakeTextTurn("model response", provider.StopReasonEndTurn),
	)

	result, err := provider.QueryWithModel(context.Background(), prov, provider.QueryWithModelRequest{
		SystemPrompt: []string{"You are helpful."},
		UserPrompt:   "Hello",
		Options: provider.QueryOptions{
			Model: "test-model",
		},
	})
	if err != nil {
		t.Fatalf("QueryWithModel failed: %v", err)
	}

	if result.Response == nil {
		t.Fatal("expected non-nil response")
	}

	// Verify the response contains text
	foundText := false
	for _, c := range result.Response.Content {
		if c.Type == "text" && c.Text == "model response" {
			foundText = true
		}
	}
	if !foundText {
		t.Errorf("expected response to contain 'model response', got %+v", result.Response.Content)
	}
}

// TestT477_QueryWithModelMissingPrompt verifies validation.
func TestT477_QueryWithModelMissingPrompt(t *testing.T) {
	prov := testharness.NewScriptedProvider()

	_, err := provider.QueryWithModel(context.Background(), prov, provider.QueryWithModelRequest{
		Options: provider.QueryOptions{Model: "test-model"},
	})
	if err == nil {
		t.Fatal("expected error for missing user prompt")
	}
	if !strings.Contains(err.Error(), "userPrompt is required") {
		t.Errorf("error = %q, want to contain 'userPrompt is required'", err.Error())
	}

	_, err = provider.QueryWithModel(context.Background(), prov, provider.QueryWithModelRequest{
		UserPrompt: "hello",
	})
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if !strings.Contains(err.Error(), "model is required") {
		t.Errorf("error = %q, want to contain 'model is required'", err.Error())
	}
}

// Ensure fmt is not reported as unused.
var _ = fmt.Sprintf
