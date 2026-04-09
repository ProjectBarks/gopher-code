// Package autodream implements background memory consolidation.
// Source: services/autoDream/autoDream.ts
//
// Fires a consolidation prompt as a background task when:
//  1. Time: hours since last consolidation >= minHours (default 24)
//  2. Sessions: number of sessions since last consolidation >= minSessions (default 5)
//  3. Lock: no other process is mid-consolidation
package autodream

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config controls when auto-dream fires.
// Source: autoDream.ts:58-66
type Config struct {
	MinHours    int // hours since last consolidation (default 24)
	MinSessions int // sessions since last consolidation (default 5)
}

// DefaultConfig returns the default auto-dream thresholds.
var DefaultConfig = Config{MinHours: 24, MinSessions: 5}

// State tracks auto-dream gate state within a session.
type State struct {
	mu              sync.Mutex
	enabled         bool
	lastScanAt      time.Time
	scanInterval    time.Duration
	memoryDir       string // ~/.claude/projects/{project}/memory/
	config          Config
}

// New creates an auto-dream state tracker.
func New(memoryDir string, cfg Config) *State {
	if cfg.MinHours <= 0 {
		cfg.MinHours = DefaultConfig.MinHours
	}
	if cfg.MinSessions <= 0 {
		cfg.MinSessions = DefaultConfig.MinSessions
	}
	return &State{
		enabled:      true,
		memoryDir:    memoryDir,
		config:       cfg,
		scanInterval: 10 * time.Minute,
	}
}

// SetEnabled enables or disables auto-dream.
func (s *State) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// ShouldRun checks all gates. Returns true if a dream should fire.
// Source: autoDream.ts:125-175
func (s *State) ShouldRun(currentSessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return false
	}

	// Time gate: check hours since last consolidation
	lastAt, err := ReadLastConsolidatedAt(s.memoryDir)
	if err != nil {
		return false
	}
	hoursSince := time.Since(lastAt).Hours()
	if hoursSince < float64(s.config.MinHours) {
		return false
	}

	// Scan throttle: don't scan sessions too frequently
	if time.Since(s.lastScanAt) < s.scanInterval {
		return false
	}
	s.lastScanAt = time.Now()

	// Session gate: count sessions since last consolidation
	sessions, err := ListSessionsSince(s.memoryDir, lastAt)
	if err != nil {
		return false
	}
	// Exclude current session
	var filtered []string
	for _, id := range sessions {
		if id != currentSessionID {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) < s.config.MinSessions {
		return false
	}

	// Lock gate
	if !TryAcquireLock(s.memoryDir) {
		return false
	}

	return true
}

// MarkComplete records successful consolidation.
func (s *State) MarkComplete() {
	writeLastConsolidatedAt(s.memoryDir, time.Now())
	ReleaseLock(s.memoryDir)
}

// MarkFailed releases the lock without updating the timestamp.
func (s *State) MarkFailed() {
	ReleaseLock(s.memoryDir)
}

// --- Lock file operations ---
// Source: services/autoDream/consolidationLock.ts

func lockPath(memDir string) string {
	return filepath.Join(memDir, ".consolidation-lock")
}

func timestampPath(memDir string) string {
	return filepath.Join(memDir, ".last-consolidated-at")
}

// ReadLastConsolidatedAt returns when consolidation last ran.
// Returns zero time if never run.
func ReadLastConsolidatedAt(memDir string) (time.Time, error) {
	data, err := os.ReadFile(timestampPath(memDir))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil // never consolidated
		}
		return time.Time{}, err
	}
	ms, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms), nil
}

func writeLastConsolidatedAt(memDir string, t time.Time) {
	os.MkdirAll(memDir, 0755)
	os.WriteFile(timestampPath(memDir), []byte(fmt.Sprintf("%d", t.UnixMilli())), 0644)
}

// ListSessionsSince returns session IDs with transcripts modified after t.
func ListSessionsSince(memDir string, since time.Time) ([]string, error) {
	transcriptDir := filepath.Join(filepath.Dir(memDir), "transcripts")
	entries, err := os.ReadDir(transcriptDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(since) {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".jsonl"))
		}
	}
	return ids, nil
}

// TryAcquireLock attempts to create the lock file. Returns false if locked.
func TryAcquireLock(memDir string) bool {
	os.MkdirAll(memDir, 0755)
	lp := lockPath(memDir)
	// Check if lock exists and is recent (< 1 hour = not stale)
	if info, err := os.Stat(lp); err == nil {
		if time.Since(info.ModTime()) < time.Hour {
			return false // active lock
		}
		// Stale lock — remove it
	}
	return os.WriteFile(lp, []byte(fmt.Sprintf("%d", time.Now().UnixMilli())), 0644) == nil
}

// ReleaseLock removes the consolidation lock file.
func ReleaseLock(memDir string) {
	os.Remove(lockPath(memDir))
}
