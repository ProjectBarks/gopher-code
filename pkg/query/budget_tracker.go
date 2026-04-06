package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Token budget continuation tracker.
// Source: src/query/tokenBudget.ts
//
// Tracks output token usage across continuations for the +500k feature.
// When the user specifies a token budget (e.g. "+500k"), the query loop
// auto-continues after each assistant turn until the budget is nearly
// exhausted or diminishing returns are detected.

// Budget tracker constants.
// Source: query/tokenBudget.ts:3-4
const (
	// CompletionThreshold is the fraction of budget at which to stop.
	// T56: 90% budget used -> stop
	CompletionThreshold = 0.9

	// DiminishingThreshold is the minimum delta tokens per continuation.
	// T57: If delta < 500 for 2 consecutive checks after 3+ continuations, stop.
	DiminishingThreshold = 500
)

// BudgetAction is the result of a budget check.
type BudgetAction int

const (
	BudgetContinue BudgetAction = iota
	BudgetStop
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

// BudgetDecision is the result of CheckTokenBudget.
// Source: query/tokenBudget.ts:22-41
// T58: TokenBudgetDecision union + completion event shape
type BudgetDecision struct {
	Action            BudgetAction
	NudgeMessage      string // only for Continue
	ContinuationCount int
	Pct               int
	TurnTokens        int
	Budget            int
	DiminishingReturns bool  // only for Stop
	DurationMs        int64 // only for Stop
}

// CheckTokenBudget decides whether to continue or stop based on token budget.
// T55: continue/stop decision
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

// GetBudgetContinuationMessage returns the nudge message for token budget continuations.
// T59: nudge text generator
// Source: utils/tokenBudget.ts:66-73
func GetBudgetContinuationMessage(pct, turnTokens, budget int) string {
	return fmt.Sprintf(
		"Stopped at %d%% of token target (%s / %s). Keep working \u2014 do not summarize.",
		pct, formatNumber(turnTokens), formatNumber(budget),
	)
}

// formatNumber formats an integer with comma separators.
func formatNumber(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		result.WriteString(s[:remainder])
		if len(s) > remainder {
			result.WriteByte(',')
		}
	}
	for i := remainder; i < len(s); i += 3 {
		result.WriteString(s[i : i+3])
		if i+3 < len(s) {
			result.WriteByte(',')
		}
	}
	return result.String()
}
