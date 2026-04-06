package provider

import (
	"sync"
	"time"
)

// RetryBudget tracks retry attempts and consecutive 529 errors across the
// lifecycle of a query. It enforces the TS retry policy:
//   - Max total retries (default 10)
//   - Max consecutive 529 errors (default 3) before triggering fallback
//   - Background query sources bail immediately on 529
//
// Source: services/api/withRetry.ts:170-365
type RetryBudget struct {
	mu sync.Mutex

	maxRetries         int
	max529Consecutive  int
	totalAttempts      int
	consecutive529     int
	querySource        QuerySource
	model              string
	fallbackModel      string
	exhausted          bool
	fallbackTriggered  bool
	lastRetryAt        time.Time
}

// RetryBudgetConfig holds the configuration for a RetryBudget.
type RetryBudgetConfig struct {
	MaxRetries        int         // Overall retry limit (default: DefaultMaxRetries)
	Max529Consecutive int         // Consecutive 529s before fallback (default: Max529Retries)
	QuerySource       QuerySource // Origin of the query for retry policy
	Model             string      // Primary model
	FallbackModel     string      // Model to fall back to on 529 exhaustion
	Initial529Count   int         // Pre-existing consecutive 529 errors (from prior attempt)
}

// NewRetryBudget creates a RetryBudget with the given configuration.
// Source: withRetry.ts:178-188
func NewRetryBudget(cfg RetryBudgetConfig) *RetryBudget {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}
	max529 := cfg.Max529Consecutive
	if max529 <= 0 {
		max529 = Max529Retries
	}
	return &RetryBudget{
		maxRetries:        maxRetries,
		max529Consecutive: max529,
		consecutive529:    cfg.Initial529Count,
		querySource:       cfg.QuerySource,
		model:             cfg.Model,
		fallbackModel:     cfg.FallbackModel,
	}
}

// RecordAttempt records a retry attempt. Returns the attempt number (1-based).
func (rb *RetryBudget) RecordAttempt() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.totalAttempts++
	rb.lastRetryAt = time.Now()
	return rb.totalAttempts
}

// Record529 records a consecutive 529 error. Returns true if the 529 budget
// is exhausted (consecutive count >= max), which should trigger fallback or
// abort.
// Source: withRetry.ts:327-365
func (rb *RetryBudget) Record529() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.consecutive529++
	if rb.consecutive529 >= rb.max529Consecutive {
		if rb.fallbackModel != "" {
			rb.fallbackTriggered = true
		} else {
			rb.exhausted = true
		}
		return true
	}
	return false
}

// Reset529Count resets the consecutive 529 counter (e.g., after a successful
// request).
func (rb *RetryBudget) Reset529Count() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.consecutive529 = 0
}

// CanRetry checks whether another retry is allowed given the current error.
// It enforces:
//   - Query source 529 bail-out for background queries
//   - Total attempt budget
//   - Error retryability
//
// Source: withRetry.ts:316-382
func (rb *RetryBudget) CanRetry(err error) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.exhausted || rb.fallbackTriggered {
		return false
	}

	// Background sources bail immediately on 529
	if Is529Error(err) && !ShouldRetry529(rb.querySource) {
		rb.exhausted = true
		return false
	}

	// Total attempt budget
	if rb.totalAttempts >= rb.maxRetries {
		rb.exhausted = true
		return false
	}

	return IsRetryableError(err)
}

// IsExhausted returns true if the retry budget is fully consumed.
func (rb *RetryBudget) IsExhausted() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.exhausted
}

// IsFallbackTriggered returns true if a model fallback was triggered by
// consecutive 529 errors.
func (rb *RetryBudget) IsFallbackTriggered() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.fallbackTriggered
}

// FallbackModel returns the configured fallback model, if any.
func (rb *RetryBudget) FallbackModel() string {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.fallbackModel
}

// TotalAttempts returns the total number of recorded attempts.
func (rb *RetryBudget) TotalAttempts() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.totalAttempts
}

// Consecutive529Count returns the current consecutive 529 error count.
func (rb *RetryBudget) Consecutive529Count() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.consecutive529
}
