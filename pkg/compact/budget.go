package compact

// TokenBudget manages the context window budget for the agent loop.
type TokenBudget struct {
	ContextWindow    int
	MaxOutputTokens  int
	CompactThreshold float64 // e.g. 0.8
}

// DefaultBudget returns a sensible default budget.
func DefaultBudget() TokenBudget {
	return TokenBudget{
		ContextWindow:    200000,
		MaxOutputTokens:  16000,
		CompactThreshold: 0.8,
	}
}

// InputBudget returns the max tokens available for input (context - output).
func (b TokenBudget) InputBudget() int {
	result := b.ContextWindow - b.MaxOutputTokens
	if result < 0 {
		return 0
	}
	return result
}

// ShouldCompact returns true if current token count exceeds the compact threshold.
func (b TokenBudget) ShouldCompact(currentTokens int) bool {
	return float64(currentTokens) > float64(b.InputBudget())*b.CompactThreshold
}
