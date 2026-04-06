package provider

import "testing"

func TestRetryBudget_ExhaustionAfterMaxRetries(t *testing.T) {
	// A budget with maxRetries=3 should allow 3 attempts then refuse more.
	// Source: withRetry.ts:189, 370
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:  3,
		QuerySource: QuerySourceREPLMainThread,
		Model:       "claude-sonnet-4-20250514",
	})

	retryableErr := &APIError{StatusCode: 429, Type: ErrRateLimit, Retryable: true}

	// First 3 attempts should be allowed
	for i := 0; i < 3; i++ {
		budget.RecordAttempt()
	}

	// After 3 attempts, CanRetry should return false
	if budget.CanRetry(retryableErr) {
		t.Error("CanRetry should return false after maxRetries exhausted")
	}

	if !budget.IsExhausted() {
		t.Error("IsExhausted should return true after maxRetries exhausted")
	}
}

func TestRetryBudget_CanRetryWithBudgetRemaining(t *testing.T) {
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:  10,
		QuerySource: QuerySourceREPLMainThread,
		Model:       "claude-sonnet-4-20250514",
	})

	retryableErr := &APIError{StatusCode: 429, Type: ErrRateLimit, Retryable: true}

	// After 1 attempt, should still be retryable
	budget.RecordAttempt()
	if !budget.CanRetry(retryableErr) {
		t.Error("CanRetry should return true with budget remaining")
	}
}

func TestRetryBudget_NonRetryableErrorRefused(t *testing.T) {
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:  10,
		QuerySource: QuerySourceREPLMainThread,
		Model:       "claude-sonnet-4-20250514",
	})

	nonRetryableErr := &APIError{StatusCode: 400, Type: ErrClientError, Retryable: false}

	budget.RecordAttempt()
	if budget.CanRetry(nonRetryableErr) {
		t.Error("CanRetry should return false for non-retryable errors")
	}
}

func TestRetryBudget_Consecutive529Fallback(t *testing.T) {
	// After Max529Retries consecutive 529s with a fallback model configured,
	// the budget should trigger fallback.
	// Source: withRetry.ts:334-351
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:        10,
		Max529Consecutive: 3,
		QuerySource:       QuerySourceREPLMainThread,
		Model:             "claude-opus-4-20250514",
		FallbackModel:     "claude-sonnet-4-20250514",
	})

	// Record 3 consecutive 529 errors
	for i := 0; i < 2; i++ {
		exhausted := budget.Record529()
		if exhausted {
			t.Fatalf("Record529 should not exhaust budget after %d errors", i+1)
		}
	}

	// Third 529 should trigger fallback
	exhausted := budget.Record529()
	if !exhausted {
		t.Error("Record529 should return true after max consecutive 529s")
	}

	if !budget.IsFallbackTriggered() {
		t.Error("IsFallbackTriggered should return true")
	}

	if budget.FallbackModel() != "claude-sonnet-4-20250514" {
		t.Errorf("FallbackModel = %q, want claude-sonnet-4-20250514", budget.FallbackModel())
	}

	// CanRetry should now return false
	retryableErr := &APIError{StatusCode: 529, Type: ErrServerOverload, Retryable: true}
	if budget.CanRetry(retryableErr) {
		t.Error("CanRetry should return false after fallback triggered")
	}
}

func TestRetryBudget_Consecutive529ExhaustionWithoutFallback(t *testing.T) {
	// Without a fallback model, consecutive 529s exhaust the budget.
	// Source: withRetry.ts:352-364
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:        10,
		Max529Consecutive: 3,
		QuerySource:       QuerySourceREPLMainThread,
		Model:             "claude-opus-4-20250514",
		// No FallbackModel
	})

	for i := 0; i < 3; i++ {
		budget.Record529()
	}

	if !budget.IsExhausted() {
		t.Error("IsExhausted should return true after max 529s without fallback")
	}

	if budget.IsFallbackTriggered() {
		t.Error("IsFallbackTriggered should be false without a fallback model")
	}
}

func TestRetryBudget_BackgroundSource529BailOut(t *testing.T) {
	// Non-foreground query sources bail immediately on 529.
	// Source: withRetry.ts:316-324
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:  10,
		QuerySource: QuerySourceCompact, // background source
		Model:       "claude-sonnet-4-20250514",
	})

	budget.RecordAttempt()

	err529 := &APIError{StatusCode: 529, Type: ErrServerOverload, Retryable: true}
	if budget.CanRetry(err529) {
		t.Error("CanRetry should return false for 529 on background source")
	}

	if !budget.IsExhausted() {
		t.Error("IsExhausted should be true after background 529 bail-out")
	}
}

func TestRetryBudget_ForegroundSource529AllowsRetry(t *testing.T) {
	// Foreground query sources should retry 529s within budget.
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:  10,
		QuerySource: QuerySourceREPLMainThread,
		Model:       "claude-sonnet-4-20250514",
	})

	budget.RecordAttempt()

	err529 := &APIError{StatusCode: 529, Type: ErrServerOverload, Retryable: true}
	if !budget.CanRetry(err529) {
		t.Error("CanRetry should return true for 529 on foreground source within budget")
	}
}

func TestRetryBudget_Reset529Count(t *testing.T) {
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:        10,
		Max529Consecutive: 3,
		QuerySource:       QuerySourceREPLMainThread,
		Model:             "claude-sonnet-4-20250514",
	})

	// Record 2 consecutive 529s, then reset
	budget.Record529()
	budget.Record529()
	budget.Reset529Count()

	if budget.Consecutive529Count() != 0 {
		t.Errorf("Consecutive529Count = %d, want 0 after reset", budget.Consecutive529Count())
	}

	// Should need 3 more 529s to exhaust
	budget.Record529()
	budget.Record529()
	exhausted := budget.Record529()
	if !exhausted {
		t.Error("Should exhaust after 3 consecutive 529s post-reset")
	}
}

func TestRetryBudget_Initial529Count(t *testing.T) {
	// Pre-seeding consecutive 529 count from a prior attempt.
	// Source: withRetry.ts:186
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:        10,
		Max529Consecutive: 3,
		QuerySource:       QuerySourceREPLMainThread,
		Model:             "claude-opus-4-20250514",
		FallbackModel:     "claude-sonnet-4-20250514",
		Initial529Count:   2,
	})

	// Already have 2 consecutive 529s; one more should trigger fallback
	exhausted := budget.Record529()
	if !exhausted {
		t.Error("Record529 should trigger fallback with Initial529Count=2")
	}

	if !budget.IsFallbackTriggered() {
		t.Error("IsFallbackTriggered should be true")
	}
}

func TestRetryBudget_DefaultConfig(t *testing.T) {
	// Zero-value config should use defaults.
	budget := NewRetryBudget(RetryBudgetConfig{
		QuerySource: QuerySourceREPLMainThread,
		Model:       "claude-sonnet-4-20250514",
	})

	retryableErr := &APIError{StatusCode: 429, Type: ErrRateLimit, Retryable: true}

	// Should allow up to DefaultMaxRetries attempts
	for i := 0; i < DefaultMaxRetries; i++ {
		budget.RecordAttempt()
	}

	if budget.CanRetry(retryableErr) {
		t.Errorf("CanRetry should return false after %d attempts (default max)", DefaultMaxRetries)
	}
}

func TestRetryBudget_TotalAttempts(t *testing.T) {
	budget := NewRetryBudget(RetryBudgetConfig{
		MaxRetries:  10,
		QuerySource: QuerySourceREPLMainThread,
		Model:       "claude-sonnet-4-20250514",
	})

	if budget.TotalAttempts() != 0 {
		t.Errorf("TotalAttempts = %d, want 0 initially", budget.TotalAttempts())
	}

	budget.RecordAttempt()
	budget.RecordAttempt()
	budget.RecordAttempt()

	if budget.TotalAttempts() != 3 {
		t.Errorf("TotalAttempts = %d, want 3", budget.TotalAttempts())
	}
}
