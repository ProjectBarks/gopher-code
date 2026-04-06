package compact

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
