package compact

import (
	"testing"
)

// Source: services/compact/autoCompact.ts

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
