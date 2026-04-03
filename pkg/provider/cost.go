package provider

import (
	"fmt"
	"math"
	"strings"
)

// Source: utils/modelCost.ts

// ModelCosts holds per-million-token pricing for a model.
// Source: utils/modelCost.ts — ModelCosts type
type ModelCosts struct {
	InputTokens           float64 // Cost per million input tokens
	OutputTokens          float64 // Cost per million output tokens
	PromptCacheWriteTokens float64 // Cache creation cost per Mtok
	PromptCacheReadTokens  float64 // Cache read cost per Mtok
	WebSearchRequests     float64 // Cost per web search request
}

// Standard pricing tiers.
// Source: utils/modelCost.ts

// CostTier3_15 is Sonnet pricing: $3 input / $15 output per Mtok.
// Source: modelCost.ts — COST_TIER_3_15
var CostTier3_15 = ModelCosts{
	InputTokens:           3,
	OutputTokens:          15,
	PromptCacheWriteTokens: 3.75,
	PromptCacheReadTokens:  0.3,
	WebSearchRequests:     0.01,
}

// CostTier15_75 is Opus 4/4.1 pricing: $15 input / $75 output per Mtok.
// Source: modelCost.ts — COST_TIER_15_75
var CostTier15_75 = ModelCosts{
	InputTokens:           15,
	OutputTokens:          75,
	PromptCacheWriteTokens: 18.75,
	PromptCacheReadTokens:  1.5,
	WebSearchRequests:     0.01,
}

// CostTier5_25 is Opus 4.5/4.6 pricing: $5 input / $25 output per Mtok.
// Source: modelCost.ts — COST_TIER_5_25
var CostTier5_25 = ModelCosts{
	InputTokens:           5,
	OutputTokens:          25,
	PromptCacheWriteTokens: 6.25,
	PromptCacheReadTokens:  0.5,
	WebSearchRequests:     0.01,
}

// CostTier30_150 is fast mode Opus 4.6 pricing: $30 input / $150 output per Mtok.
// Source: modelCost.ts — COST_TIER_30_150
var CostTier30_150 = ModelCosts{
	InputTokens:           30,
	OutputTokens:          150,
	PromptCacheWriteTokens: 37.5,
	PromptCacheReadTokens:  3,
	WebSearchRequests:     0.01,
}

// CostHaiku35 is Haiku 3.5 pricing: $0.80 input / $4 output per Mtok.
// Source: modelCost.ts — COST_HAIKU_35
var CostHaiku35 = ModelCosts{
	InputTokens:           0.8,
	OutputTokens:          4,
	PromptCacheWriteTokens: 1,
	PromptCacheReadTokens:  0.08,
	WebSearchRequests:     0.01,
}

// CostHaiku45 is Haiku 4.5 pricing: $1 input / $5 output per Mtok.
// Source: modelCost.ts — COST_HAIKU_45
var CostHaiku45 = ModelCosts{
	InputTokens:           1,
	OutputTokens:          5,
	PromptCacheWriteTokens: 1.25,
	PromptCacheReadTokens:  0.1,
	WebSearchRequests:     0.01,
}

// ModelCostsMap maps canonical model names to their pricing.
// Source: utils/modelCost.ts — MODEL_COSTS
var ModelCostsMap = map[string]ModelCosts{
	"claude-3-5-haiku":  CostHaiku35,
	"claude-haiku-4-5":  CostHaiku45,
	"claude-3-5-sonnet": CostTier3_15,
	"claude-3-7-sonnet": CostTier3_15,
	"claude-sonnet-4":   CostTier3_15,
	"claude-sonnet-4-5": CostTier3_15,
	"claude-sonnet-4-6": CostTier3_15,
	"claude-opus-4":     CostTier15_75,
	"claude-opus-4-1":   CostTier15_75,
	"claude-opus-4-5":   CostTier5_25,
	"claude-opus-4-6":   CostTier5_25,
}

// GetModelCosts returns the pricing for a model, using canonical name resolution.
// Falls back to Sonnet pricing for unknown models.
// Source: utils/modelCost.ts — getModelCosts
func GetModelCosts(model string) ModelCosts {
	// Try exact match first
	if costs, ok := ModelCostsMap[model]; ok {
		return costs
	}

	// Try canonical name resolution
	canonical := FirstPartyNameToCanonical(model)
	if costs, ok := ModelCostsMap[canonical]; ok {
		return costs
	}

	// Fallback: detect family from name
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "haiku-4-5") || strings.Contains(lower, "haiku-4.5"):
		return CostHaiku45
	case strings.Contains(lower, "haiku"):
		return CostHaiku35
	case strings.Contains(lower, "opus-4-6") || strings.Contains(lower, "opus-4-5"):
		return CostTier5_25
	case strings.Contains(lower, "opus"):
		return CostTier15_75
	default:
		return CostTier3_15 // Default to Sonnet pricing
	}
}

// TokenUsage holds token counts for cost calculation.
type TokenUsage struct {
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// CalculateUSDCost computes the total cost for a model + usage.
// Source: utils/modelCost.ts — calculateUSDCost
func CalculateUSDCost(model string, usage TokenUsage) float64 {
	costs := GetModelCosts(model)

	return float64(usage.InputTokens)/1_000_000*costs.InputTokens +
		float64(usage.OutputTokens)/1_000_000*costs.OutputTokens +
		float64(usage.CacheReadInputTokens)/1_000_000*costs.PromptCacheReadTokens +
		float64(usage.CacheCreationInputTokens)/1_000_000*costs.PromptCacheWriteTokens
}

// FormatCost formats a cost value for display.
// Source: utils/modelCost.ts — formatCost
func FormatCost(cost float64) string {
	if cost > 0.5 {
		return fmt.Sprintf("$%.2f", math.Round(cost*100)/100)
	}
	return fmt.Sprintf("$%.4f", cost)
}

// FormatModelPricing returns a human-readable pricing string.
// Source: utils/modelCost.ts — formatModelPricing
func FormatModelPricing(costs ModelCosts) string {
	return fmt.Sprintf("%s/%s per Mtok", formatPrice(costs.InputTokens), formatPrice(costs.OutputTokens))
}

func formatPrice(price float64) string {
	if price == float64(int(price)) {
		return fmt.Sprintf("$%d", int(price))
	}
	return fmt.Sprintf("$%.2f", price)
}
