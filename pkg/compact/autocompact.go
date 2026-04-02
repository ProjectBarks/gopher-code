package compact

import "time"

// Source: services/compact/autoCompact.ts

// AutoCompact constants matching TS source.
// Source: services/compact/autoCompact.ts:62-65
const (
	AutocompactBufferTokens    = 13_000
	WarningThresholdBuffer     = 20_000
	ErrorThresholdBuffer       = 20_000
	ManualCompactBufferTokens  = 3_000
)

// MaxConsecutiveAutocompactFailures is the circuit breaker threshold.
// Source: services/compact/autoCompact.ts:70
const MaxConsecutiveAutocompactFailures = 3

// AutoCompactTrackingState tracks compaction state across turns.
// Source: services/compact/autoCompact.ts:51-60
type AutoCompactTrackingState struct {
	Compacted           bool
	TurnCounter         int
	TurnID              string
	ConsecutiveFailures int
}

// ShouldSkipAutoCompact returns true when the circuit breaker has tripped.
// Source: services/compact/autoCompact.ts:343-345
func (s *AutoCompactTrackingState) ShouldSkipAutoCompact() bool {
	return s.ConsecutiveFailures >= MaxConsecutiveAutocompactFailures
}

// RecordSuccess resets the failure counter on successful compaction.
// Source: services/compact/autoCompact.ts:332
func (s *AutoCompactTrackingState) RecordSuccess() {
	s.ConsecutiveFailures = 0
	s.Compacted = true
}

// RecordFailure increments the consecutive failure counter.
// Source: services/compact/autoCompact.ts:338-349
func (s *AutoCompactTrackingState) RecordFailure() {
	s.ConsecutiveFailures++
}

// GetAutoCompactThreshold returns the token count at which auto-compact triggers.
// Source: services/compact/autoCompact.ts:72-91
func GetAutoCompactThreshold(contextWindow int) int {
	return contextWindow - AutocompactBufferTokens
}

// TokenWarningState describes the context window utilization.
// Source: services/compact/autoCompact.ts:93-101
type TokenWarningState struct {
	PercentLeft                 int
	IsAboveWarningThreshold     bool
	IsAboveErrorThreshold       bool
	IsAboveAutoCompactThreshold bool
	IsAtBlockingLimit           bool
}

// CalculateTokenWarningState computes context window utilization.
// Source: services/compact/autoCompact.ts:93-130
func CalculateTokenWarningState(tokenUsage, contextWindow int) TokenWarningState {
	threshold := contextWindow - AutocompactBufferTokens
	if threshold <= 0 {
		threshold = 1
	}

	percentLeft := int(float64(threshold-tokenUsage) / float64(threshold) * 100)
	if percentLeft < 0 {
		percentLeft = 0
	}

	warningThreshold := threshold - WarningThresholdBuffer
	errorThreshold := threshold - ErrorThresholdBuffer

	return TokenWarningState{
		PercentLeft:                 percentLeft,
		IsAboveWarningThreshold:     tokenUsage >= warningThreshold,
		IsAboveErrorThreshold:       tokenUsage >= errorThreshold,
		IsAboveAutoCompactThreshold: tokenUsage >= threshold,
		IsAtBlockingLimit:           tokenUsage >= contextWindow,
	}
}

// --- Token Budget Tracker ---
// Source: query/tokenBudget.ts

// Budget tracker constants.
// Source: query/tokenBudget.ts:3-4
const (
	CompletionThreshold   = 0.9
	DiminishingThreshold  = 500
)

// BudgetTracker tracks token output across continuations for budget enforcement.
// Source: query/tokenBudget.ts:6-11
type BudgetTracker struct {
	ContinuationCount    int
	LastDeltaTokens      int
	LastGlobalTurnTokens int
	StartedAt            time.Time
}

// NewBudgetTracker creates a fresh tracker.
// Source: query/tokenBudget.ts:13-19
func NewBudgetTracker() *BudgetTracker {
	return &BudgetTracker{
		StartedAt: time.Now(),
	}
}

// BudgetAction is the result of a budget check.
type BudgetAction int

const (
	BudgetContinue BudgetAction = iota
	BudgetStop
)

// BudgetDecision is the result of CheckTokenBudget.
type BudgetDecision struct {
	Action            BudgetAction
	NudgeMessage      string // only for Continue
	ContinuationCount int
	Pct               int
	TurnTokens        int
	Budget            int
	DiminishingReturns bool   // only for Stop
	DurationMs        int64  // only for Stop
}

// CheckTokenBudget decides whether to continue or stop based on token budget.
// Source: query/tokenBudget.ts:45-93
func (t *BudgetTracker) CheckTokenBudget(budget int, globalTurnTokens int) BudgetDecision {
	if budget <= 0 {
		return BudgetDecision{Action: BudgetStop}
	}

	turnTokens := globalTurnTokens
	pct := int(float64(turnTokens) / float64(budget) * 100)
	deltaSinceLastCheck := globalTurnTokens - t.LastGlobalTurnTokens

	// Source: query/tokenBudget.ts:59-62
	isDiminishing := t.ContinuationCount >= 3 &&
		deltaSinceLastCheck < DiminishingThreshold &&
		t.LastDeltaTokens < DiminishingThreshold

	// Source: query/tokenBudget.ts:64-76
	if !isDiminishing && turnTokens < int(float64(budget)*CompletionThreshold) {
		t.ContinuationCount++
		t.LastDeltaTokens = deltaSinceLastCheck
		t.LastGlobalTurnTokens = globalTurnTokens
		return BudgetDecision{
			Action:            BudgetContinue,
			ContinuationCount: t.ContinuationCount,
			Pct:               pct,
			TurnTokens:        turnTokens,
			Budget:            budget,
		}
	}

	// Source: query/tokenBudget.ts:78-93
	if isDiminishing || t.ContinuationCount > 0 {
		return BudgetDecision{
			Action:             BudgetStop,
			ContinuationCount:  t.ContinuationCount,
			Pct:                pct,
			TurnTokens:         turnTokens,
			Budget:             budget,
			DiminishingReturns: isDiminishing,
			DurationMs:         time.Since(t.StartedAt).Milliseconds(),
		}
	}

	return BudgetDecision{Action: BudgetStop}
}
