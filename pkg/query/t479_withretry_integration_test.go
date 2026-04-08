// T479: Integration test verifying provider.WithRetry and provider.StreamWithRetry
// are wired into the binary through the query loop.
//
// These tests exercise the real code path from query.Query -> provider.StreamWithRetry
// -> provider.WithRetry, verifying exponential backoff retry behavior with typed
// APIError classification.

package query_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// TestT479_StreamWithRetryBackoff verifies that StreamWithRetry applies
// exponential backoff delays when retrying transient errors. The test
// measures wall-clock time to confirm delays actually occur.
// Source: services/api/withRetry.ts:170-517
func TestT479_StreamWithRetryBackoff(t *testing.T) {
	// 2 consecutive 500 errors, then success. WithRetry should apply
	// exponential backoff between attempts.
	apiErr500 := provider.ClassifyHTTPError(500, []byte(`Internal Server Error`), "")

	prov := testharness.NewScriptedProvider(
		testharness.MakeErrorTurn(apiErr500),
		testharness.MakeErrorTurn(apiErr500),
		testharness.MakeTextTurn("recovered", provider.StopReasonEndTurn),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()
	// Use 5ms base delay — enough to measure but fast for tests.
	sess.Config.RetryBaseDelay = 5 * time.Millisecond

	start := time.Now()
	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}

	// With 5ms base delay and 2 retries, expect at least ~5ms total delay
	// (attempt 1 delay = 5ms, attempt 2 delay = 10ms, minus jitter variance).
	if elapsed < 4*time.Millisecond {
		t.Errorf("expected measurable backoff delay, got %v", elapsed)
	}

	// All 3 turns should have been consumed (2 errors + 1 success).
	reqs := prov.Requests()
	if len(reqs) != 3 {
		t.Fatalf("expected 3 requests (2 retries + 1 success), got %d", len(reqs))
	}
}

// TestT479_StreamWithRetryNonRetryableImmediate verifies that non-retryable
// errors (auth, billing) are returned immediately without retry delays.
// Source: services/api/withRetry.ts:696-770
func TestT479_StreamWithRetryNonRetryableImmediate(t *testing.T) {
	apiErr401 := provider.ClassifyHTTPError(401, []byte(`invalid x-api-key`), "")

	prov := testharness.NewScriptedProvider(
		testharness.MakeErrorTurn(apiErr401),
		testharness.MakeTextTurn("should not reach", provider.StopReasonEndTurn),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()

	start := time.Now()
	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for auth failure")
	}

	// Non-retryable errors should return almost immediately (no backoff).
	if elapsed > 100*time.Millisecond {
		t.Errorf("non-retryable error took too long: %v", elapsed)
	}

	// Only 1 request should have been made.
	reqs := prov.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request (no retry), got %d", len(reqs))
	}
}

// TestT479_StreamWithRetryFallbackTriggered verifies that WithRetry triggers
// a FallbackTriggeredError after consecutive 529 errors, and the query loop
// switches to the fallback model.
// Source: services/api/withRetry.ts:327-365
func TestT479_StreamWithRetryFallbackTriggered(t *testing.T) {
	apiErr529 := provider.ClassifyHTTPError(529, []byte(`{"type":"overloaded_error"}`), "")

	// 3 consecutive 529s trigger fallback, then success on fallback model.
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
	sess.Config.QuerySource = provider.QuerySourceREPLMainThread

	var events []query.QueryEvent
	callback := func(evt query.QueryEvent) {
		events = append(events, evt)
	}

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}

	// Model should have been switched to fallback.
	if sess.Config.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want fallback %q", sess.Config.Model, "claude-sonnet-4-20250514")
	}

	// Should have emitted a "Switched to" notification.
	found := false
	for _, evt := range events {
		if evt.Type == query.QEventTextDelta && strings.Contains(evt.Text, "Switched to") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Switched to' notification event")
	}
}

// TestT479_StreamWithRetryContextCancellation verifies that WithRetry
// respects context cancellation during backoff sleep.
// Source: services/api/withRetry.ts:465-470
func TestT479_StreamWithRetryContextCancellation(t *testing.T) {
	apiErr500 := provider.ClassifyHTTPError(500, []byte(`Internal Server Error`), "")

	// Many error turns — but context will be cancelled before exhausting them.
	var turns []testharness.TurnScript
	for i := 0; i < 20; i++ {
		turns = append(turns, testharness.MakeErrorTurn(apiErr500))
	}
	prov := testharness.NewScriptedProvider(turns...)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()
	sess.Config.RetryBaseDelay = 50 * time.Millisecond // longer delay to ensure cancel during backoff

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := query.Query(ctx, sess, prov, registry, orchestrator, nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	// Should not have consumed all 20 turns — context cancellation should stop early.
	reqs := prov.Requests()
	if len(reqs) >= 10 {
		t.Errorf("expected early termination from context cancel, got %d requests", len(reqs))
	}
}

// TestT479_WithRetryCannotRetryErrorUnwrapped verifies that when WithRetry
// returns a CannotRetryError, the query loop unwraps it to extract the
// original APIError for proper classification and user-facing messages.
func TestT479_WithRetryCannotRetryErrorUnwrapped(t *testing.T) {
	apiErrBilling := provider.ClassifyHTTPError(400, []byte(`Credit balance is too low`), "")

	prov := testharness.NewScriptedProvider(
		testharness.MakeErrorTurn(apiErrBilling),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)
	sess := testharness.MakeSession()

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err == nil {
		t.Fatal("expected error for billing failure")
	}

	var agentErr *query.AgentError
	if !errors.As(err, &agentErr) {
		t.Fatalf("expected AgentError, got %T: %v", err, err)
	}

	// The UserMessage should be set from the unwrapped APIError, not the
	// CannotRetryError wrapper.
	if agentErr.UserMessage != provider.CreditBalanceTooLowErrorMessage {
		t.Errorf("UserMessage = %q, want %q", agentErr.UserMessage, provider.CreditBalanceTooLowErrorMessage)
	}
}
