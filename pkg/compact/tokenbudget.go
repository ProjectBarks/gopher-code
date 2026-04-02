package compact

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Source: utils/tokenBudget.ts

// Token budget parsing regexes.
// Source: utils/tokenBudget.ts:1-9
var (
	// Shorthand at start: +500k, +2M, +1.5b
	// Source: utils/tokenBudget.ts:3
	shorthandStartRE = regexp.MustCompile(`(?i)^\s*\+(\d+(?:\.\d+)?)\s*(k|m|b)\b`)

	// Shorthand at end: "do something +500k"
	// Source: utils/tokenBudget.ts:7
	shorthandEndRE = regexp.MustCompile(`(?i)\s\+(\d+(?:\.\d+)?)\s*(k|m|b)\s*[.!?]?\s*$`)

	// Verbose: "use 2M tokens", "spend 500k tokens"
	// Source: utils/tokenBudget.ts:8
	verboseRE = regexp.MustCompile(`(?i)\b(?:use|spend)\s+(\d+(?:\.\d+)?)\s*(k|m|b)\s*tokens?\b`)
)

// Multipliers for k/m/b suffixes.
// Source: utils/tokenBudget.ts:11-15
var multipliers = map[string]float64{
	"k": 1_000,
	"m": 1_000_000,
	"b": 1_000_000_000,
}

// parseBudgetMatch converts a number+suffix match to token count.
// Source: utils/tokenBudget.ts:17-19
func parseBudgetMatch(value, suffix string) int {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	mult := multipliers[strings.ToLower(suffix)]
	return int(math.Round(f * mult))
}

// ParseTokenBudget extracts a token budget from user input text.
// Supports shorthand (+500k, +2M) and verbose (use 500k tokens, spend 2M tokens).
// Returns 0 if no budget is found.
// Source: utils/tokenBudget.ts:21-29
func ParseTokenBudget(text string) int {
	// 1. Check shorthand at start
	if m := shorthandStartRE.FindStringSubmatch(text); m != nil {
		return parseBudgetMatch(m[1], m[2])
	}
	// 2. Check shorthand at end
	if m := shorthandEndRE.FindStringSubmatch(text); m != nil {
		return parseBudgetMatch(m[1], m[2])
	}
	// 3. Check verbose
	if m := verboseRE.FindStringSubmatch(text); m != nil {
		return parseBudgetMatch(m[1], m[2])
	}
	return 0
}

// GetBudgetContinuationMessage returns the nudge message for token budget continuations.
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
