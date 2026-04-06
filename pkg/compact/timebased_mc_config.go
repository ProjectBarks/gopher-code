package compact

// Source: services/compact/timeBasedMCConfig.ts

// TimeBasedMCConfig controls time-based microcompact behavior.
// When enabled, clears old tool results if the gap since the last
// assistant message exceeds the threshold — the server-side prompt
// cache has likely expired, so the full prefix will be rewritten anyway.
//
// Runs BEFORE the API call so the shrunk prompt is what gets sent.
// Main-thread only — subagents have short lifetimes.
// Source: timeBasedMCConfig.ts:18-28
type TimeBasedMCConfig struct {
	// Enabled is the master switch. When false, time-based MC is a no-op.
	Enabled bool
	// GapThresholdMinutes triggers when (now - last assistant timestamp)
	// exceeds this many minutes. 60 is the safe choice (1h server cache TTL).
	GapThresholdMinutes int
	// KeepRecent preserves this many most-recent compactable tool results.
	KeepRecent int
}

// DefaultTimeBasedMCConfig is the production default.
// Source: timeBasedMCConfig.ts:30-34
var DefaultTimeBasedMCConfig = TimeBasedMCConfig{
	Enabled:             false,
	GapThresholdMinutes: 60,
	KeepRecent:          5,
}

// TimeBasedMCConfigProvider returns the current time-based MC config.
// In TS this reads from GrowthBook; in Go callers inject a provider or
// use the default.
type TimeBasedMCConfigProvider func() TimeBasedMCConfig

// GetDefaultTimeBasedMCConfig returns the default config (feature disabled).
// Source: timeBasedMCConfig.ts:36-43
func GetDefaultTimeBasedMCConfig() TimeBasedMCConfig {
	return DefaultTimeBasedMCConfig
}
