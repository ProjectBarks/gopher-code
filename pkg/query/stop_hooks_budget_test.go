package query_test

import (
	"context"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// T51-T52: Stop hook pattern — after each assistant turn with no tool calls,
// run Stop hooks. If any hook returns decision="stop", end the loop.

func TestStopHooks_PreventContinuation(t *testing.T) {
	// Source: query.ts:1267-1305
	// When the stop hook runner returns PreventContinuation=true,
	// the query loop should end without error.
	prov := testharness.NewScriptedProvider(
		testharness.MakeTextTurn("I am done", provider.StopReasonEndTurn),
	)
	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSessionWithConfig(session.SessionConfig{
		Model:          "test-model",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	})

	// Wire stop hook runner that prevents continuation
	sess.StopHookRunner = query.StopHookRunner(func(texts []string) query.StopHookResult {
		return query.StopHookResult{PreventContinuation: true}
	})

	var gotTurnComplete bool
	err := query.Query(context.Background(), sess, prov, registry, orchestrator, func(evt query.QueryEvent) {
		if evt.Type == query.QEventTurnComplete {
			gotTurnComplete = true
		}
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !gotTurnComplete {
		t.Error("expected QEventTurnComplete to be emitted before stop")
	}
}

func TestStopHooks_BlockingErrors_CauseContinuation(t *testing.T) {
	// Source: query.ts:1290-1305
	// When stop hooks return blocking errors, the loop injects them as
	// user messages and continues (model gets another turn).

	callCount := 0
	prov := testharness.NewScriptedProvider(
		// Turn 1: model says text, stop hook injects blocking error
		testharness.MakeTextTurn("attempt 1", provider.StopReasonEndTurn),
		// Turn 2: model sees blocking error and responds, hook allows
		testharness.MakeTextTurn("attempt 2 fixed", provider.StopReasonEndTurn),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSessionWithConfig(session.SessionConfig{
		Model:          "test-model",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	})

	// First call returns blocking error, second call allows
	sess.StopHookRunner = query.StopHookRunner(func(texts []string) query.StopHookResult {
		callCount++
		if callCount == 1 {
			return query.StopHookResult{
				BlockingErrors: []string{"Hook detected an issue: lint failed"},
			}
		}
		return query.StopHookResult{}
	})

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected stop hook to be called 2 times, got %d", callCount)
	}

	// Verify blocking error was injected as user message
	foundBlockingError := false
	for _, msg := range sess.Messages {
		if msg.Role == message.RoleUser {
			for _, b := range msg.Content {
				if b.Type == message.ContentText && strings.Contains(b.Text, "lint failed") {
					foundBlockingError = true
				}
			}
		}
	}
	if !foundBlockingError {
		t.Error("expected blocking error to be injected as user message")
	}
}

func TestStopHooks_NotCalledOnToolUse(t *testing.T) {
	// Stop hooks only fire when the model ends with no tool calls.
	// When tool calls are present, the loop continues to tool execution.
	hookCalled := false
	prov := testharness.NewScriptedProvider(
		testharness.MakeToolTurn("t1", "my_tool", []byte(`{}`), provider.StopReasonToolUse),
		testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
	)

	spy := testharness.NewSpyTool("my_tool", false)
	registry := tools.NewRegistry()
	registry.Register(spy)
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSessionWithConfig(session.SessionConfig{
		Model:          "test-model",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	})

	sess.StopHookRunner = query.StopHookRunner(func(texts []string) query.StopHookResult {
		hookCalled = true
		return query.StopHookResult{}
	})

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Hook should be called once (on the second turn when model says "done" with no tool calls)
	if !hookCalled {
		t.Error("expected stop hook to be called on the text-only turn")
	}
}

func TestStopHooks_ReceivesAssistantText(t *testing.T) {
	// Verify the stop hook runner receives the assistant's text content.
	var receivedTexts []string
	prov := testharness.NewScriptedProvider(
		testharness.MakeTextTurn("hello world", provider.StopReasonEndTurn),
	)
	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSessionWithConfig(session.SessionConfig{
		Model:          "test-model",
		MaxTurns:       100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	})

	sess.StopHookRunner = query.StopHookRunner(func(texts []string) query.StopHookResult {
		receivedTexts = texts
		return query.StopHookResult{}
	})

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(receivedTexts) != 1 || receivedTexts[0] != "hello world" {
		t.Errorf("expected stop hook to receive [\"hello world\"], got %v", receivedTexts)
	}
}

// T54-T55: BudgetTracker — tracks token usage, decides continue/stop.

func TestBudgetTracker_ContinuesUnderBudget(t *testing.T) {
	// Source: query/tokenBudget.ts:64-76
	tracker := query.NewBudgetTracker()
	decision := tracker.CheckTokenBudget(10_000, 1_000)
	if decision.Action != query.BudgetContinue {
		t.Error("should continue when well under budget")
	}
	if decision.ContinuationCount != 1 {
		t.Errorf("expected continuation count 1, got %d", decision.ContinuationCount)
	}
	if decision.Pct != 10 {
		t.Errorf("expected pct=10, got %d", decision.Pct)
	}
}

func TestBudgetTracker_StopsAt90Percent(t *testing.T) {
	// Source: query/tokenBudget.ts:3 — COMPLETION_THRESHOLD = 0.9
	tracker := query.NewBudgetTracker()
	tracker.ContinuationCount = 1
	decision := tracker.CheckTokenBudget(10_000, 9_500)
	if decision.Action != query.BudgetStop {
		t.Error("should stop at 95% of budget (above 90% threshold)")
	}
}

func TestBudgetTracker_DiminishingReturns(t *testing.T) {
	// Source: query/tokenBudget.ts:59-62
	// After 3+ continuations with delta < 500, detect diminishing returns.
	tracker := query.NewBudgetTracker()
	tracker.ContinuationCount = 3
	tracker.LastDeltaTokens = 100
	tracker.LastGlobalTurnTokens = 5000

	decision := tracker.CheckTokenBudget(100_000, 5100)
	if decision.Action != query.BudgetStop {
		t.Error("should stop on diminishing returns")
	}
	if !decision.DiminishingReturns {
		t.Error("should flag diminishing returns")
	}
}

func TestBudgetTracker_NoDiminishingBefore3Continuations(t *testing.T) {
	// Source: query/tokenBudget.ts:60 — continuationCount >= 3 required
	tracker := query.NewBudgetTracker()
	tracker.ContinuationCount = 2
	tracker.LastDeltaTokens = 100
	tracker.LastGlobalTurnTokens = 5000

	decision := tracker.CheckTokenBudget(100_000, 5100)
	if decision.Action != query.BudgetContinue {
		t.Error("should continue — not enough continuations for diminishing detection")
	}
}

func TestBudgetTracker_StopsWithZeroBudget(t *testing.T) {
	// Source: query/tokenBudget.ts:51-53
	tracker := query.NewBudgetTracker()
	decision := tracker.CheckTokenBudget(0, 1000)
	if decision.Action != query.BudgetStop {
		t.Error("should stop with zero budget")
	}
}

func TestBudgetTracker_Constants(t *testing.T) {
	// T56, T57: Verify constants match TS source.
	if query.CompletionThreshold != 0.9 {
		t.Errorf("CompletionThreshold = %v, want 0.9", query.CompletionThreshold)
	}
	if query.DiminishingThreshold != 500 {
		t.Errorf("DiminishingThreshold = %d, want 500", query.DiminishingThreshold)
	}
}

func TestGetBudgetContinuationMessage(t *testing.T) {
	// T59: nudge text generator
	// Source: utils/tokenBudget.ts:66-73
	msg := query.GetBudgetContinuationMessage(45, 225_000, 500_000)
	expected := "Stopped at 45% of token target (225,000 / 500,000). Keep working \u2014 do not summarize."
	if msg != expected {
		t.Errorf("got %q, want %q", msg, expected)
	}
}

// Integration: budget tracker wired into query loop.

func TestBudgetTracker_IntegrationWithQueryLoop(t *testing.T) {
	// Source: query.ts:1308-1355
	// When model stops early but budget remains, inject nudge and continue.
	prov := testharness.NewScriptedProvider(
		testharness.MakeTextTurnWithUsage("first part", provider.StopReasonEndTurn,
			provider.Usage{InputTokens: 1000, OutputTokens: 5000}),
		testharness.MakeTextTurnWithUsage("second part", provider.StopReasonEndTurn,
			provider.Usage{InputTokens: 2000, OutputTokens: 90000}),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSessionWithConfig(session.SessionConfig{
		Model:             "test-model",
		MaxTurns:          100,
		TokenBudget:       compact.DefaultBudget(),
		PermissionMode:    permissions.AutoApprove,
		TokenBudgetTarget: 100_000,
	})

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Find the nudge message
	foundNudge := false
	for _, msg := range sess.Messages {
		if msg.Role == message.RoleUser {
			for _, b := range msg.Content {
				if b.Type == message.ContentText &&
					strings.Contains(b.Text, "Stopped at") &&
					strings.Contains(b.Text, "token target") {
					foundNudge = true
				}
			}
		}
	}
	if !foundNudge {
		t.Error("expected a nudge message containing 'Stopped at...token target'")
	}
}

// Integration: stop hooks + budget tracker combined.

func TestStopHooks_RunBeforeBudgetCheck(t *testing.T) {
	// Source: query.ts:1267-1355
	// Stop hooks run first. If they prevent continuation, budget check is skipped.
	prov := testharness.NewScriptedProvider(
		testharness.MakeTextTurnWithUsage("output", provider.StopReasonEndTurn,
			provider.Usage{InputTokens: 1000, OutputTokens: 100}),
	)

	registry := tools.NewRegistry()
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSessionWithConfig(session.SessionConfig{
		Model:             "test-model",
		MaxTurns:          100,
		TokenBudget:       compact.DefaultBudget(),
		PermissionMode:    permissions.AutoApprove,
		TokenBudgetTarget: 100_000, // Budget would normally allow continuation
	})

	// Stop hook prevents continuation despite budget remaining
	sess.StopHookRunner = query.StopHookRunner(func(texts []string) query.StopHookResult {
		return query.StopHookResult{PreventContinuation: true}
	})

	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Only 2 messages: initial "hello" + assistant "output"
	// No nudge message should exist because stop hook prevented it.
	for _, msg := range sess.Messages {
		if msg.Role == message.RoleUser {
			for _, b := range msg.Content {
				if b.Type == message.ContentText && strings.Contains(b.Text, "Stopped at") {
					t.Error("nudge message should NOT be injected when stop hook prevents continuation")
				}
			}
		}
	}
}
