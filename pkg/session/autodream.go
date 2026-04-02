package session

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Source: services/autoDream/autoDream.ts

// AutoDream trigger constants.
// Source: services/autoDream/autoDream.ts:56-66
const (
	// SessionScanIntervalMs is the throttle between session directory scans.
	// Source: autoDream.ts:56
	SessionScanIntervalMs = 10 * 60 * 1000 // 10 minutes

	// DefaultMinHours is the minimum hours since last consolidation.
	// Source: autoDream.ts:64
	DefaultMinHours = 24

	// DefaultMinSessions is the minimum number of sessions since last consolidation.
	// Source: autoDream.ts:65
	DefaultMinSessions = 5
)

// AutoDreamConfig holds the trigger thresholds.
// Source: services/autoDream/autoDream.ts:58-66
type AutoDreamConfig struct {
	MinHours    float64 // Hours since last consolidation (default 24)
	MinSessions int     // Sessions since last consolidation (default 5)
}

// DefaultAutoDreamConfig returns the default config.
// Source: autoDream.ts:63-66
func DefaultAutoDreamConfig() AutoDreamConfig {
	return AutoDreamConfig{
		MinHours:    DefaultMinHours,
		MinSessions: DefaultMinSessions,
	}
}

// AutoDreamState tracks when the last consolidation ran.
type AutoDreamState struct {
	LastConsolidatedAt time.Time
	LastScanAt         time.Time
}

// ShouldTriggerAutoDream checks if auto-dream should fire based on time and session count.
// Returns true when:
// 1. Hours since last consolidation >= minHours (default 24)
// 2. Number of sessions since last consolidation >= minSessions (default 5)
// Source: services/autoDream/autoDream.ts:125-169
func ShouldTriggerAutoDream(state *AutoDreamState, cfg AutoDreamConfig, sessionDir string, currentSessionID string) bool {
	// Time gate: hours since last consolidation
	// Source: autoDream.ts:140-141
	hoursSince := time.Since(state.LastConsolidatedAt).Hours()
	if hoursSince < cfg.MinHours {
		return false
	}

	// Scan throttle: don't scan more than once per 10 minutes
	// Source: autoDream.ts:144-150
	if time.Since(state.LastScanAt) < time.Duration(SessionScanIntervalMs)*time.Millisecond {
		return false
	}
	state.LastScanAt = time.Now()

	// Session gate: count sessions touched since last consolidation
	// Source: autoDream.ts:153-169
	sessionCount := countSessionsSince(sessionDir, state.LastConsolidatedAt, currentSessionID)
	return sessionCount >= cfg.MinSessions
}

// countSessionsSince counts session JSONL files modified after the given time,
// excluding the current session.
// Source: autoDream.ts:154-169
func countSessionsSince(sessionDir string, since time.Time, currentSessionID string) int {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return 0
	}

	count := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		// Exclude current session
		// Source: autoDream.ts:164-165
		sessionID := strings.TrimSuffix(e.Name(), ".jsonl")
		if sessionID == currentSessionID {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(since) {
			count++
		}
	}
	return count
}

// ReadLastConsolidatedAt reads the timestamp of the last auto-dream consolidation.
// Returns zero time if no consolidation has occurred.
func ReadLastConsolidatedAt(memDir string) time.Time {
	path := filepath.Join(memDir, ".last-consolidation")
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	ms, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// RecordConsolidation writes the current timestamp as the last consolidation time.
func RecordConsolidation(memDir string) error {
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(memDir, ".last-consolidation")
	data := []byte(strconv.FormatInt(time.Now().UnixMilli(), 10))
	return os.WriteFile(path, data, 0644)
}
