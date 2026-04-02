package compact

import (
	"testing"
)

// Source: services/compact/autoCompact.ts, query/tokenBudget.ts

func TestCircuitBreaker(t *testing.T) {
	// Source: services/compact/autoCompact.ts:70
	t.Run("trips_after_3_failures", func(t *testing.T) {
		state := &AutoCompactTrackingState{}

		if state.ShouldSkipAutoCompact() {
			t.Error("should not skip before any failures")
		}

		state.RecordFailure()
		if state.ShouldSkipAutoCompact() {
			t.Error("should not skip after 1 failure")
		}

		state.RecordFailure()
		if state.ShouldSkipAutoCompact() {
			t.Error("should not skip after 2 failures")
		}

		state.RecordFailure()
		if !state.ShouldSkipAutoCompact() {
			t.Error("should skip after 3 failures (circuit breaker tripped)")
		}
	})

	// Source: services/compact/autoCompact.ts:332
	t.Run("resets_on_success", func(t *testing.T) {
		state := &AutoCompactTrackingState{ConsecutiveFailures: 2}
		state.RecordSuccess()
		if state.ConsecutiveFailures != 0 {
			t.Errorf("expected 0 failures after success, got %d", state.ConsecutiveFailures)
		}
		if !state.Compacted {
			t.Error("expected Compacted=true after success")
		}
	})

	// Source: services/compact/autoCompact.ts:338-349
	t.Run("failure_count_increments", func(t *testing.T) {
		state := &AutoCompactTrackingState{}
		state.RecordFailure()
		if state.ConsecutiveFailures != 1 {
			t.Errorf("expected 1, got %d", state.ConsecutiveFailures)
		}
		state.RecordFailure()
		if state.ConsecutiveFailures != 2 {
			t.Errorf("expected 2, got %d", state.ConsecutiveFailures)
		}
	})
}

func TestAutocompactConstants(t *testing.T) {
	// Source: services/compact/autoCompact.ts:62-65
	if AutocompactBufferTokens != 13_000 {
		t.Errorf("expected 13000, got %d", AutocompactBufferTokens)
	}
	if WarningThresholdBuffer != 20_000 {
		t.Errorf("expected 20000, got %d", WarningThresholdBuffer)
	}
	if MaxConsecutiveAutocompactFailures != 3 {
		t.Errorf("expected 3, got %d", MaxConsecutiveAutocompactFailures)
	}
}

func TestGetAutoCompactThreshold(t *testing.T) {
	// Source: services/compact/autoCompact.ts:72-91
	threshold := GetAutoCompactThreshold(200_000)
	expected := 200_000 - 13_000
	if threshold != expected {
		t.Errorf("expected %d, got %d", expected, threshold)
	}
}

func TestCalculateTokenWarningState(t *testing.T) {
	// Source: services/compact/autoCompact.ts:93-130
	t.Run("low_usage", func(t *testing.T) {
		state := CalculateTokenWarningState(10_000, 200_000)
		if state.IsAboveWarningThreshold {
			t.Error("should not be above warning at low usage")
		}
		if state.IsAboveAutoCompactThreshold {
			t.Error("should not be above autocompact at low usage")
		}
		if state.PercentLeft <= 0 {
			t.Error("should have positive percent left")
		}
	})

	t.Run("above_threshold", func(t *testing.T) {
		// 200k - 13k buffer = 187k threshold
		state := CalculateTokenWarningState(190_000, 200_000)
		if !state.IsAboveAutoCompactThreshold {
			t.Error("should be above autocompact threshold at 190k/200k")
		}
	})
}

func TestBudgetTracker_DiminishingReturns(t *testing.T) {
	// Source: query/tokenBudget.ts:59-62

	t.Run("continues_when_under_threshold", func(t *testing.T) {
		tracker := NewBudgetTracker()
		decision := tracker.CheckTokenBudget(10_000, 1_000)
		if decision.Action != BudgetContinue {
			t.Error("should continue when well under budget")
		}
		if decision.ContinuationCount != 1 {
			t.Errorf("expected continuation count 1, got %d", decision.ContinuationCount)
		}
	})

	t.Run("stops_at_90_percent", func(t *testing.T) {
		// Source: query/tokenBudget.ts:3 — COMPLETION_THRESHOLD = 0.9
		tracker := NewBudgetTracker()
		tracker.ContinuationCount = 1 // need > 0 for stop
		decision := tracker.CheckTokenBudget(10_000, 9_500)
		if decision.Action != BudgetStop {
			t.Error("should stop at 95% of budget (above 90% threshold)")
		}
	})

	t.Run("detects_diminishing_after_3_continuations", func(t *testing.T) {
		// Source: query/tokenBudget.ts:59-62
		// Requires: continuationCount >= 3, last two deltas < 500
		tracker := NewBudgetTracker()
		tracker.ContinuationCount = 3
		tracker.LastDeltaTokens = 100     // < 500
		tracker.LastGlobalTurnTokens = 5000

		// delta = 5100 - 5000 = 100 < 500
		decision := tracker.CheckTokenBudget(100_000, 5100)
		if decision.Action != BudgetStop {
			t.Error("should stop on diminishing returns")
		}
		if !decision.DiminishingReturns {
			t.Error("should flag diminishing returns")
		}
	})

	t.Run("no_diminishing_before_3_continuations", func(t *testing.T) {
		// Source: query/tokenBudget.ts:60 — requires continuationCount >= 3
		tracker := NewBudgetTracker()
		tracker.ContinuationCount = 2  // < 3
		tracker.LastDeltaTokens = 100
		tracker.LastGlobalTurnTokens = 5000

		decision := tracker.CheckTokenBudget(100_000, 5100)
		if decision.Action != BudgetContinue {
			t.Error("should continue — not enough continuations for diminishing detection")
		}
	})

	t.Run("stops_with_zero_budget", func(t *testing.T) {
		// Source: query/tokenBudget.ts:51-53
		tracker := NewBudgetTracker()
		decision := tracker.CheckTokenBudget(0, 1000)
		if decision.Action != BudgetStop {
			t.Error("should stop with zero budget")
		}
	})
}
