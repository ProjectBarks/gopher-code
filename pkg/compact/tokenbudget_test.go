package compact

import (
	"testing"
)

// Source: utils/tokenBudget.ts

func TestParseTokenBudget(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		// Source: utils/tokenBudget.ts:3 — shorthand at start
		{"shorthand_start_500k", "+500k", 500_000},
		{"shorthand_start_2M", "+2M", 2_000_000},
		{"shorthand_start_1b", "+1b", 1_000_000_000},
		{"shorthand_start_1.5k", "+1.5k", 1_500},
		{"shorthand_start_with_space", "  +500k", 500_000},

		// Source: utils/tokenBudget.ts:7 — shorthand at end
		{"shorthand_end", "write a lot +500k", 500_000},
		{"shorthand_end_period", "do stuff +2M.", 2_000_000},
		{"shorthand_end_exclaim", "go for it +1.5m!", 1_500_000},

		// Source: utils/tokenBudget.ts:8 — verbose
		{"verbose_use", "use 500k tokens", 500_000},
		{"verbose_spend", "spend 2M tokens", 2_000_000},
		{"verbose_use_token_singular", "use 1m token", 1_000_000},
		{"verbose_in_sentence", "please use 500k tokens to do this", 500_000},

		// Case insensitivity
		{"case_upper_K", "+500K", 500_000},
		{"case_upper_M", "+2M", 2_000_000},
		{"case_mixed", "Use 500K Tokens", 500_000},

		// No match
		{"no_match_plain_text", "hello world", 0},
		{"no_match_plus_no_suffix", "+500", 0},
		{"no_match_mid_sentence_shorthand", "I have +500k in my account", 0}, // not at start or end
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTokenBudget(tt.input)
			if got != tt.expected {
				t.Errorf("ParseTokenBudget(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetBudgetContinuationMessage(t *testing.T) {
	// Source: utils/tokenBudget.ts:66-73
	msg := GetBudgetContinuationMessage(45, 225_000, 500_000)
	expected := "Stopped at 45% of token target (225,000 / 500,000). Keep working \u2014 do not summarize."
	if msg != expected {
		t.Errorf("got %q, want %q", msg, expected)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{500000, "500,000"},
	}
	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.expected {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
