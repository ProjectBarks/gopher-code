package provider

import (
	"os"
	"strconv"
	"strings"
)

// Source: utils/effort.ts

// EffortLevel represents a named effort level.
type EffortLevel string

const (
	EffortLow    EffortLevel = "low"
	EffortMedium EffortLevel = "medium"
	EffortHigh   EffortLevel = "high"
	EffortMax    EffortLevel = "max"
)

// EffortLevels is the ordered list of valid effort levels.
// Source: effort.ts:13-18
var EffortLevels = []EffortLevel{EffortLow, EffortMedium, EffortHigh, EffortMax}

// IsEffortLevel checks if a string is a valid effort level.
// Source: effort.ts:68-70
func IsEffortLevel(value string) bool {
	switch EffortLevel(value) {
	case EffortLow, EffortMedium, EffortHigh, EffortMax:
		return true
	}
	return false
}

// ParseEffortValue parses a string or number into an EffortLevel.
// Source: effort.ts:71-87
func ParseEffortValue(value string) (EffortLevel, bool) {
	if value == "" {
		return "", false
	}
	v := strings.ToLower(value)
	if IsEffortLevel(v) {
		return EffortLevel(v), true
	}
	// Try numeric
	if n, err := strconv.Atoi(v); err == nil {
		return ConvertNumericEffortToLevel(n), true
	}
	return "", false
}

// ConvertNumericEffortToLevel maps a numeric value to an effort level.
// Source: effort.ts:202-216
func ConvertNumericEffortToLevel(value int) EffortLevel {
	if value <= 50 {
		return EffortLow
	}
	if value <= 85 {
		return EffortMedium
	}
	if value <= 100 {
		return EffortHigh
	}
	return EffortMax
}

// ModelSupportsEffort checks if a model supports the effort parameter.
// Source: effort.ts:23-49
func ModelSupportsEffort(model string) bool {
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_ALWAYS_ENABLE_EFFORT")) {
		return true
	}
	m := strings.ToLower(model)
	// Opus 4.6 and Sonnet 4.6 support effort
	if strings.Contains(m, "opus-4-6") || strings.Contains(m, "sonnet-4-6") {
		return true
	}
	// Exclude known legacy models
	if strings.Contains(m, "haiku") || strings.Contains(m, "sonnet") || strings.Contains(m, "opus") {
		return false
	}
	// Default true for unknown models on 1P
	return GetAPIProvider() == ProviderFirstParty
}

// ModelSupportsMaxEffort checks if a model supports 'max' effort level.
// Source: effort.ts:53-65
func ModelSupportsMaxEffort(model string) bool {
	if strings.Contains(strings.ToLower(model), "opus-4-6") {
		return true
	}
	return false
}

// ResolveAppliedEffort resolves the effort value for API calls.
// Precedence: env → appState → model default.
// Source: effort.ts:152-167
func ResolveAppliedEffort(model string, appStateEffort EffortLevel) EffortLevel {
	// Env override
	envOverride := os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
	if strings.EqualFold(envOverride, "unset") || strings.EqualFold(envOverride, "auto") {
		return ""
	}
	if envOverride != "" {
		if parsed, ok := ParseEffortValue(envOverride); ok {
			// API rejects 'max' on non-Opus-4.6
			if parsed == EffortMax && !ModelSupportsMaxEffort(model) {
				return EffortHigh
			}
			return parsed
		}
	}

	// appState
	if appStateEffort != "" {
		if appStateEffort == EffortMax && !ModelSupportsMaxEffort(model) {
			return EffortHigh
		}
		return appStateEffort
	}

	// Model default — for simplicity, return empty (API defaults to high)
	return ""
}

// GetDisplayedEffortLevel returns the effort level to show the user.
// Source: effort.ts:174-180
func GetDisplayedEffortLevel(model string, appStateEffort EffortLevel) EffortLevel {
	resolved := ResolveAppliedEffort(model, appStateEffort)
	if resolved == "" {
		return EffortHigh
	}
	return resolved
}

// GetEffortLevelDescription returns a human-readable description.
// Source: effort.ts:224-235
func GetEffortLevelDescription(level EffortLevel) string {
	switch level {
	case EffortLow:
		return "Quick, straightforward implementation with minimal overhead"
	case EffortMedium:
		return "Balanced approach with standard implementation and testing"
	case EffortHigh:
		return "Comprehensive implementation with extensive testing and documentation"
	case EffortMax:
		return "Maximum capability with deepest reasoning (Opus 4.6 only)"
	default:
		return ""
	}
}

// Source: utils/thinking.ts

// ThinkingConfigType represents the thinking mode.
type ThinkingConfigType string

const (
	ThinkingAdaptive ThinkingConfigType = "adaptive"
	ThinkingEnabled  ThinkingConfigType = "enabled"
	ThinkingDisabled ThinkingConfigType = "disabled"
)

// ModelSupportsThinking checks if a model supports extended thinking.
// Source: thinking.ts:90-110
func ModelSupportsThinking(model string) bool {
	canonical := GetCanonicalName(model)
	p := GetAPIProvider()
	// 1P and Foundry: all Claude 4+ (including Haiku 4.5)
	if p == ProviderFoundry || p == ProviderFirstParty {
		return !strings.Contains(canonical, "claude-3-")
	}
	// 3P: only Opus 4+ and Sonnet 4+
	return strings.Contains(canonical, "sonnet-4") || strings.Contains(canonical, "opus-4")
}

// ModelSupportsAdaptiveThinking checks if a model supports adaptive thinking.
// Source: thinking.ts:113-144
func ModelSupportsAdaptiveThinking(model string) bool {
	canonical := GetCanonicalName(model)
	// Supported by 4.6 models
	if strings.Contains(canonical, "opus-4-6") || strings.Contains(canonical, "sonnet-4-6") {
		return true
	}
	// Exclude known legacy models
	if strings.Contains(canonical, "opus") || strings.Contains(canonical, "sonnet") || strings.Contains(canonical, "haiku") {
		return false
	}
	// Default true for 1P/Foundry unknown models
	p := GetAPIProvider()
	return p == ProviderFirstParty || p == ProviderFoundry
}
