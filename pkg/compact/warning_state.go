package compact

// Source: services/compact/compactWarningState.ts

import "sync/atomic"

// CompactWarningState tracks whether the "context left until autocompact"
// warning should be suppressed. Suppressed immediately after successful
// compaction since token counts are stale until the next API response.
// Source: compactWarningState.ts:1-18
type CompactWarningState struct {
	suppressed atomic.Bool
}

// NewCompactWarningState creates a new warning state (initially not suppressed).
func NewCompactWarningState() *CompactWarningState {
	return &CompactWarningState{}
}

// Suppress marks the warning as suppressed. Call after successful compaction.
// Source: compactWarningState.ts:11-13
func (s *CompactWarningState) Suppress() {
	s.suppressed.Store(true)
}

// ClearSuppression clears the suppression. Called at the start of a new
// compact attempt (the warning may be valid again).
// Source: compactWarningState.ts:16-18
func (s *CompactWarningState) ClearSuppression() {
	s.suppressed.Store(false)
}

// IsSuppressed returns whether the compact warning is currently suppressed.
func (s *CompactWarningState) IsSuppressed() bool {
	return s.suppressed.Load()
}
