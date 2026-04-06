package session

// ---------------------------------------------------------------------------
// T165: Turn output token budget tracking
// Source: bootstrap/state.ts — getTurnOutputTokens, getCurrentTurnTokenBudget,
//   snapshotOutputTokensForTurn, incrementBudgetContinuationCount,
//   getBudgetContinuationCount
//
// Tracks how many output tokens the model has used this turn for budget
// decisions. The query loop calls SnapshotOutputTokensForTurn at turn start,
// then reads GetTurnOutputTokens to compute the delta. Budget continuation
// count tracks how many times the model has been auto-continued within a
// single turn.
// ---------------------------------------------------------------------------

// TurnBudgetState holds per-turn output token budget tracking state.
// These are module-scope in the TS (not in STATE), but we attach them to
// SessionState for cleaner Go architecture.
type TurnBudgetState struct {
	outputTokensAtTurnStart int
	currentTurnTokenBudget  *int
	budgetContinuationCount int
}

// GetTurnOutputTokens returns the number of output tokens used since the
// last SnapshotOutputTokensForTurn call.
// Source: bootstrap/state.ts — getTurnOutputTokens
func (s *SessionState) GetTurnOutputTokens() int {
	return s.TotalOutputTokens - s.turnBudget.outputTokensAtTurnStart
}

// GetCurrentTurnTokenBudget returns the token budget for the current turn,
// or nil if no budget is set.
// Source: bootstrap/state.ts — getCurrentTurnTokenBudget
func (s *SessionState) GetCurrentTurnTokenBudget() *int {
	return s.turnBudget.currentTurnTokenBudget
}

// SnapshotOutputTokensForTurn records the current total output tokens as the
// baseline for the new turn, sets the turn budget, and resets the continuation
// count.
// Source: bootstrap/state.ts — snapshotOutputTokensForTurn
func (s *SessionState) SnapshotOutputTokensForTurn(budget *int) {
	s.turnBudget.outputTokensAtTurnStart = s.TotalOutputTokens
	s.turnBudget.currentTurnTokenBudget = budget
	s.turnBudget.budgetContinuationCount = 0
}

// GetBudgetContinuationCount returns the number of budget continuations in
// the current turn.
// Source: bootstrap/state.ts — getBudgetContinuationCount
func (s *SessionState) GetBudgetContinuationCount() int {
	return s.turnBudget.budgetContinuationCount
}

// IncrementBudgetContinuationCount increments the budget continuation counter.
// Source: bootstrap/state.ts — incrementBudgetContinuationCount
func (s *SessionState) IncrementBudgetContinuationCount() {
	s.turnBudget.budgetContinuationCount++
}
