package provider

import (
	"math"
	"testing"
)

// Source: utils/modelCost.ts

func TestModelCostsTiers(t *testing.T) {
	// Source: modelCost.ts — pricing tier constants
	t.Run("sonnet_3_15", func(t *testing.T) {
		if CostTier3_15.InputTokens != 3 {
			t.Errorf("input = %v, want 3", CostTier3_15.InputTokens)
		}
		if CostTier3_15.OutputTokens != 15 {
			t.Errorf("output = %v, want 15", CostTier3_15.OutputTokens)
		}
		if CostTier3_15.PromptCacheWriteTokens != 3.75 {
			t.Errorf("cache write = %v, want 3.75", CostTier3_15.PromptCacheWriteTokens)
		}
		if CostTier3_15.PromptCacheReadTokens != 0.3 {
			t.Errorf("cache read = %v, want 0.3", CostTier3_15.PromptCacheReadTokens)
		}
	})

	t.Run("opus_4_15_75", func(t *testing.T) {
		if CostTier15_75.InputTokens != 15 {
			t.Errorf("input = %v, want 15", CostTier15_75.InputTokens)
		}
		if CostTier15_75.OutputTokens != 75 {
			t.Errorf("output = %v, want 75", CostTier15_75.OutputTokens)
		}
	})

	t.Run("opus_45_46_5_25", func(t *testing.T) {
		if CostTier5_25.InputTokens != 5 {
			t.Errorf("input = %v, want 5", CostTier5_25.InputTokens)
		}
		if CostTier5_25.OutputTokens != 25 {
			t.Errorf("output = %v, want 25", CostTier5_25.OutputTokens)
		}
	})

	t.Run("fast_mode_30_150", func(t *testing.T) {
		if CostTier30_150.InputTokens != 30 {
			t.Errorf("input = %v, want 30", CostTier30_150.InputTokens)
		}
		if CostTier30_150.OutputTokens != 150 {
			t.Errorf("output = %v, want 150", CostTier30_150.OutputTokens)
		}
	})

	t.Run("haiku_35", func(t *testing.T) {
		if CostHaiku35.InputTokens != 0.8 {
			t.Errorf("input = %v, want 0.8", CostHaiku35.InputTokens)
		}
		if CostHaiku35.OutputTokens != 4 {
			t.Errorf("output = %v, want 4", CostHaiku35.OutputTokens)
		}
	})

	t.Run("haiku_45", func(t *testing.T) {
		if CostHaiku45.InputTokens != 1 {
			t.Errorf("input = %v, want 1", CostHaiku45.InputTokens)
		}
		if CostHaiku45.OutputTokens != 5 {
			t.Errorf("output = %v, want 5", CostHaiku45.OutputTokens)
		}
	})
}

func TestModelCostsMap(t *testing.T) {
	// Source: modelCost.ts — MODEL_COSTS
	if len(ModelCostsMap) < 11 {
		t.Errorf("expected at least 11 model entries, got %d", len(ModelCostsMap))
	}

	// Verify specific mappings
	tests := []struct {
		model  string
		input  float64
		output float64
	}{
		{"claude-sonnet-4-6", 3, 15},
		{"claude-sonnet-4-5", 3, 15},
		{"claude-opus-4-6", 5, 25},
		{"claude-opus-4-5", 5, 25},
		{"claude-opus-4", 15, 75},
		{"claude-opus-4-1", 15, 75},
		{"claude-haiku-4-5", 1, 5},
		{"claude-3-5-haiku", 0.8, 4},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			costs, ok := ModelCostsMap[tc.model]
			if !ok {
				t.Fatalf("missing %s", tc.model)
			}
			if costs.InputTokens != tc.input {
				t.Errorf("input = %v, want %v", costs.InputTokens, tc.input)
			}
			if costs.OutputTokens != tc.output {
				t.Errorf("output = %v, want %v", costs.OutputTokens, tc.output)
			}
		})
	}
}

func TestGetModelCosts(t *testing.T) {
	// Source: modelCost.ts — getModelCosts

	t.Run("exact_match", func(t *testing.T) {
		costs := GetModelCosts("claude-sonnet-4-6")
		if costs.InputTokens != 3 {
			t.Errorf("input = %v, want 3", costs.InputTokens)
		}
	})

	t.Run("canonical_resolution", func(t *testing.T) {
		// Full model ID with date should resolve via FirstPartyNameToCanonical
		costs := GetModelCosts("claude-sonnet-4-5-20250929")
		if costs.InputTokens != 3 {
			t.Errorf("input = %v, want 3 (sonnet pricing)", costs.InputTokens)
		}
	})

	t.Run("opus_46_family", func(t *testing.T) {
		costs := GetModelCosts("claude-opus-4-6")
		if costs.InputTokens != 5 {
			t.Errorf("opus 4.6 input = %v, want 5", costs.InputTokens)
		}
	})

	t.Run("unknown_defaults_to_sonnet", func(t *testing.T) {
		costs := GetModelCosts("totally-unknown-model")
		if costs.InputTokens != 3 {
			t.Errorf("unknown model should default to sonnet pricing, got input=%v", costs.InputTokens)
		}
	})

	t.Run("haiku_family_detection", func(t *testing.T) {
		costs := GetModelCosts("some-haiku-model")
		if costs.OutputTokens != 4 {
			t.Errorf("haiku family should match haiku 3.5, got output=%v", costs.OutputTokens)
		}
	})

	t.Run("opus_family_detection", func(t *testing.T) {
		costs := GetModelCosts("us.anthropic.claude-opus-4-6-v1")
		if costs.InputTokens != 5 {
			t.Errorf("bedrock opus should match 5/25, got input=%v", costs.InputTokens)
		}
	})
}

func TestCalculateUSDCost(t *testing.T) {
	// Source: modelCost.ts — calculateUSDCost

	t.Run("basic_sonnet", func(t *testing.T) {
		// 10k input tokens + 1k output tokens at Sonnet pricing ($3/$15 per Mtok)
		cost := CalculateUSDCost("claude-sonnet-4-6", TokenUsage{
			InputTokens:  10000,
			OutputTokens: 1000,
		})
		expected := 10000.0/1e6*3 + 1000.0/1e6*15 // 0.03 + 0.015 = 0.045
		if math.Abs(cost-expected) > 0.0001 {
			t.Errorf("cost = %f, want %f", cost, expected)
		}
	})

	t.Run("with_cache", func(t *testing.T) {
		// Including cache tokens
		cost := CalculateUSDCost("claude-sonnet-4-6", TokenUsage{
			InputTokens:              100000,
			OutputTokens:             5000,
			CacheReadInputTokens:     50000,
			CacheCreationInputTokens: 20000,
		})
		expected := 100000.0/1e6*3 + 5000.0/1e6*15 + 50000.0/1e6*0.3 + 20000.0/1e6*3.75
		if math.Abs(cost-expected) > 0.0001 {
			t.Errorf("cost = %f, want %f", cost, expected)
		}
	})

	t.Run("zero_tokens", func(t *testing.T) {
		cost := CalculateUSDCost("claude-sonnet-4-6", TokenUsage{})
		if cost != 0 {
			t.Errorf("zero tokens should cost $0, got %f", cost)
		}
	})

	t.Run("opus_is_more_expensive", func(t *testing.T) {
		usage := TokenUsage{InputTokens: 10000, OutputTokens: 1000}
		sonnetCost := CalculateUSDCost("claude-sonnet-4-6", usage)
		opusCost := CalculateUSDCost("claude-opus-4-6", usage)
		if opusCost <= sonnetCost {
			t.Errorf("opus should be more expensive: opus=$%f sonnet=$%f", opusCost, sonnetCost)
		}
	})
}

func TestFormatCost(t *testing.T) {
	// Source: modelCost.ts — formatCost
	t.Run("small_cost", func(t *testing.T) {
		s := FormatCost(0.0045)
		if s != "$0.0045" {
			t.Errorf("got %q", s)
		}
	})

	t.Run("large_cost", func(t *testing.T) {
		s := FormatCost(1.234)
		if s != "$1.23" {
			t.Errorf("got %q", s)
		}
	})

	t.Run("zero", func(t *testing.T) {
		s := FormatCost(0)
		if s != "$0.0000" {
			t.Errorf("got %q", s)
		}
	})
}

func TestFormatModelPricing(t *testing.T) {
	// Source: modelCost.ts — formatModelPricing
	t.Run("sonnet", func(t *testing.T) {
		s := FormatModelPricing(CostTier3_15)
		if s != "$3/$15 per Mtok" {
			t.Errorf("got %q", s)
		}
	})

	t.Run("haiku_35", func(t *testing.T) {
		s := FormatModelPricing(CostHaiku35)
		if s != "$0.80/$4 per Mtok" {
			t.Errorf("got %q", s)
		}
	})

	t.Run("opus_46", func(t *testing.T) {
		s := FormatModelPricing(CostTier5_25)
		if s != "$5/$25 per Mtok" {
			t.Errorf("got %q", s)
		}
	})
}

func TestCacheTokenPricing(t *testing.T) {
	// Source: modelCost.ts — cache pricing is 25% write / 10% read of input

	t.Run("sonnet_cache_write_25pct", func(t *testing.T) {
		// Cache write should be 25% more than input: $3 * 1.25 = $3.75
		if CostTier3_15.PromptCacheWriteTokens != 3.75 {
			t.Errorf("cache write = %v, want 3.75 (125%% of $3)", CostTier3_15.PromptCacheWriteTokens)
		}
	})

	t.Run("sonnet_cache_read_10pct", func(t *testing.T) {
		// Cache read should be 10% of input: $3 * 0.10 = $0.30
		if CostTier3_15.PromptCacheReadTokens != 0.3 {
			t.Errorf("cache read = %v, want 0.3 (10%% of $3)", CostTier3_15.PromptCacheReadTokens)
		}
	})

	t.Run("haiku_45_cache_ratios", func(t *testing.T) {
		// Haiku 4.5: $1 input, cache write = $1.25 (125%), cache read = $0.10 (10%)
		if CostHaiku45.PromptCacheWriteTokens != 1.25 {
			t.Errorf("haiku cache write = %v, want 1.25", CostHaiku45.PromptCacheWriteTokens)
		}
		if CostHaiku45.PromptCacheReadTokens != 0.1 {
			t.Errorf("haiku cache read = %v, want 0.1", CostHaiku45.PromptCacheReadTokens)
		}
	})
}
