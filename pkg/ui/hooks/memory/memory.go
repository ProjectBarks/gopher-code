// Package memory provides bubbletea hooks for memory/skills monitoring:
// process memory usage tracking, skill-directory change detection, and
// skill-improvement suggestion tracking.
//
// Source: src/hooks/useMemoryUsage.ts, src/hooks/useSkillsChange.ts,
//
//	src/hooks/useSkillImprovementSurvey.ts,
//	src/utils/hooks/skillImprovement.ts,
//	src/utils/skills/skillChangeDetector.ts
package memory

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// MemoryUsage — process memory monitor
// Source: src/hooks/useMemoryUsage.ts (39 LOC)
// ---------------------------------------------------------------------------

// MemoryStatus classifies the current heap usage level.
type MemoryStatus int

const (
	// MemoryNormal indicates heap usage is within safe bounds.
	MemoryNormal MemoryStatus = iota
	// MemoryHigh indicates heap usage exceeds 1.5 GB.
	MemoryHigh
	// MemoryCritical indicates heap usage exceeds 2.5 GB.
	MemoryCritical
)

func (s MemoryStatus) String() string {
	switch s {
	case MemoryHigh:
		return "high"
	case MemoryCritical:
		return "critical"
	default:
		return "normal"
	}
}

const (
	// HighMemoryThreshold is the heap usage level that triggers a "high" warning.
	// Matches TS HIGH_MEMORY_THRESHOLD = 1.5 * 1024^3.
	HighMemoryThreshold uint64 = 1.5 * 1024 * 1024 * 1024 // 1.5 GB

	// CriticalMemoryThreshold is the heap usage level that triggers a "critical" warning.
	// Matches TS CRITICAL_MEMORY_THRESHOLD = 2.5 * 1024^3.
	CriticalMemoryThreshold uint64 = 2.5 * 1024 * 1024 * 1024 // 2.5 GB

	// DefaultPollInterval is the default polling interval for memory checks.
	// Matches TS useInterval(fn, 10_000).
	DefaultPollInterval = 10 * time.Second
)

// MemoryUsageMsg is dispatched by the memory monitor tick.
// A nil pointer means status is normal (no action needed) — matches the TS
// optimization of returning null to avoid re-rendering Notifications.
type MemoryUsageMsg struct {
	HeapUsed uint64
	Status   MemoryStatus
}

// MemoryUsage tracks process heap usage and surfaces warnings when approaching
// context limits. In bubbletea this becomes a repeating tick Cmd.
type MemoryUsage struct {
	// PollInterval overrides the default 10s poll interval (for testing).
	PollInterval time.Duration
	// readMem is an injectable memory reader (defaults to runtime.ReadMemStats).
	readMem func() uint64
}

// Classify returns the MemoryStatus for a given heap size in bytes.
func Classify(heapUsed uint64) MemoryStatus {
	switch {
	case heapUsed >= CriticalMemoryThreshold:
		return MemoryCritical
	case heapUsed >= HighMemoryThreshold:
		return MemoryHigh
	default:
		return MemoryNormal
	}
}

// Suggestion returns a user-facing bloat suggestion for the given status.
// Returns empty string for normal status.
func Suggestion(status MemoryStatus) string {
	switch status {
	case MemoryCritical:
		return "Memory usage is critical (>2.5 GB). Consider starting a new session."
	case MemoryHigh:
		return "Memory usage is high (>1.5 GB). Long sessions may slow down."
	default:
		return ""
	}
}

func (m *MemoryUsage) interval() time.Duration {
	if m.PollInterval > 0 {
		return m.PollInterval
	}
	return DefaultPollInterval
}

func (m *MemoryUsage) heapAlloc() uint64 {
	if m.readMem != nil {
		return m.readMem()
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

// Tick returns a tea.Cmd that polls memory and dispatches MemoryUsageMsg.
// Returns nil when status is normal (matching the TS null-when-normal optimization).
func (m *MemoryUsage) Tick() tea.Cmd {
	return tea.Tick(m.interval(), func(time.Time) tea.Msg {
		heap := m.heapAlloc()
		status := Classify(heap)
		if status == MemoryNormal {
			return nil
		}
		return MemoryUsageMsg{HeapUsed: heap, Status: status}
	})
}

// ---------------------------------------------------------------------------
// SkillsWatcher — skill directory change detection
// Source: src/utils/skills/skillChangeDetector.ts,
//
//	src/hooks/useSkillsChange.ts
//
// ---------------------------------------------------------------------------

// SkillsChangedMsg is dispatched when skill files on disk have changed.
type SkillsChangedMsg struct {
	// ChangedPaths lists the paths that were detected as changed.
	ChangedPaths []string
}

// SkillsWatcher monitors .claude/skills/ directories for changes and
// dispatches SkillsChangedMsg when files are added, modified, or removed.
// In the TS source this is chokidar-based; in Go we use stat-polling for
// simplicity and portability (fsnotify can be added later if needed).
type SkillsWatcher struct {
	// Dirs is the list of skill directories to watch.
	Dirs []string
	// PollInterval overrides the default poll interval (for testing).
	PollInterval time.Duration
	// ReloadDebounce overrides the 300ms debounce window (for testing).
	ReloadDebounce time.Duration

	mu        sync.Mutex
	snapshots map[string]time.Time // path -> last mod time
	onChange  func(changed []string)
}

const (
	// DefaultSkillsPollInterval matches TS POLLING_INTERVAL_MS = 2000.
	DefaultSkillsPollInterval = 2 * time.Second
	// DefaultReloadDebounce matches TS RELOAD_DEBOUNCE_MS = 300.
	DefaultReloadDebounce = 300 * time.Millisecond
)

func (w *SkillsWatcher) pollInterval() time.Duration {
	if w.PollInterval > 0 {
		return w.PollInterval
	}
	return DefaultSkillsPollInterval
}

// Init takes an initial snapshot of all watched directories.
func (w *SkillsWatcher) Init() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.snapshots = w.scan()
}

// scan walks all configured directories and returns path -> modtime.
func (w *SkillsWatcher) scan() map[string]time.Time {
	snap := make(map[string]time.Time)
	for _, dir := range w.Dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			info, err := e.Info()
			if err != nil {
				continue
			}
			snap[p] = info.ModTime()
			// Recurse one level (skills use skill-name/SKILL.md format, depth: 2)
			if e.IsDir() {
				subEntries, err := os.ReadDir(p)
				if err != nil {
					continue
				}
				for _, se := range subEntries {
					sp := filepath.Join(p, se.Name())
					si, err := se.Info()
					if err != nil {
						continue
					}
					snap[sp] = si.ModTime()
				}
			}
		}
	}
	return snap
}

// Poll checks for changes since the last snapshot and returns any changed paths.
func (w *SkillsWatcher) Poll() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	current := w.scan()
	var changed []string

	// Detect new or modified files.
	for p, modTime := range current {
		if prev, ok := w.snapshots[p]; !ok || !modTime.Equal(prev) {
			changed = append(changed, p)
		}
	}

	// Detect deleted files.
	for p := range w.snapshots {
		if _, ok := current[p]; !ok {
			changed = append(changed, p)
		}
	}

	w.snapshots = current
	return changed
}

// Tick returns a tea.Cmd that polls skill directories for changes.
func (w *SkillsWatcher) Tick() tea.Cmd {
	return tea.Tick(w.pollInterval(), func(time.Time) tea.Msg {
		changed := w.Poll()
		if len(changed) == 0 {
			return nil
		}
		return SkillsChangedMsg{ChangedPaths: changed}
	})
}

// ---------------------------------------------------------------------------
// SkillImprovementTracker — tracks skill usage and surfaces suggestions
// Source: src/utils/hooks/skillImprovement.ts,
//
//	src/hooks/useSkillImprovementSurvey.ts
//
// ---------------------------------------------------------------------------

// SkillUpdate describes a single proposed change to a skill definition.
// Matches the TS SkillUpdate type.
type SkillUpdate struct {
	Section string `json:"section"`
	Change  string `json:"change"`
	Reason  string `json:"reason"`
}

// SkillImprovementSuggestion is a batch of proposed updates for a skill.
type SkillImprovementSuggestion struct {
	SkillName string
	Updates   []SkillUpdate
}

// SurveyResponse represents the user's response to a skill-improvement prompt.
type SurveyResponse int

const (
	// SurveyApplied means the user accepted the improvement.
	SurveyApplied SurveyResponse = iota
	// SurveyDismissed means the user declined.
	SurveyDismissed
)

func (r SurveyResponse) String() string {
	if r == SurveyApplied {
		return "applied"
	}
	return "dismissed"
}

// SkillImprovementMsg is dispatched when a skill improvement suggestion is ready.
type SkillImprovementMsg struct {
	Suggestion *SkillImprovementSuggestion
}

// SkillImprovementTracker monitors skill usage across conversation turns and
// surfaces improvement suggestions after a configurable batch of user messages.
//
// In the TS source this is a post-sampling hook that runs every TURN_BATCH_SIZE
// (5) user messages, queries a small/fast model for suggested improvements, and
// sets app state when updates are found.
type SkillImprovementTracker struct {
	// TurnBatchSize is the number of user turns between improvement checks.
	// Matches TS TURN_BATCH_SIZE = 5.
	TurnBatchSize int

	mu                sync.Mutex
	userTurnCount     int
	lastAnalyzedCount int
	skillUseCounts    map[string]int // skill name -> invocation count
	suggestion        *SkillImprovementSuggestion
	loggedAppearance  bool
}

const (
	// DefaultTurnBatchSize matches TS TURN_BATCH_SIZE = 5.
	DefaultTurnBatchSize = 5
)

func (t *SkillImprovementTracker) batchSize() int {
	if t.TurnBatchSize > 0 {
		return t.TurnBatchSize
	}
	return DefaultTurnBatchSize
}

// RecordTurn records a user turn. Returns true if the batch threshold has
// been reached and an improvement check should run.
func (t *SkillImprovementTracker) RecordTurn() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.userTurnCount++
	if t.userTurnCount-t.lastAnalyzedCount >= t.batchSize() {
		t.lastAnalyzedCount = t.userTurnCount
		return true
	}
	return false
}

// RecordSkillUse increments the invocation count for a named skill.
func (t *SkillImprovementTracker) RecordSkillUse(skillName string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.skillUseCounts == nil {
		t.skillUseCounts = make(map[string]int)
	}
	t.skillUseCounts[skillName]++
	return t.skillUseCounts[skillName]
}

// SkillUseCount returns the current invocation count for a skill.
func (t *SkillImprovementTracker) SkillUseCount(skillName string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.skillUseCounts[skillName]
}

// SetSuggestion stores a pending improvement suggestion.
func (t *SkillImprovementTracker) SetSuggestion(s *SkillImprovementSuggestion) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.suggestion = s
	t.loggedAppearance = false
}

// PendingSuggestion returns the current pending suggestion, or nil.
func (t *SkillImprovementTracker) PendingSuggestion() *SkillImprovementSuggestion {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.suggestion
}

// HandleResponse processes the user's survey response. Returns the suggestion
// that was acted on (nil if none was pending).
func (t *SkillImprovementTracker) HandleResponse(resp SurveyResponse) *SkillImprovementSuggestion {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.suggestion
	if s == nil {
		return nil
	}
	t.suggestion = nil
	t.loggedAppearance = false
	return s
}

// MarkAppearanceLogged records that the survey-appeared event was logged.
// Returns true if this is the first call since the last suggestion was set.
func (t *SkillImprovementTracker) MarkAppearanceLogged() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.loggedAppearance {
		return false
	}
	t.loggedAppearance = true
	return true
}

// ResultMessage returns the user-facing system message after applying an
// improvement. Matches TS: `Skill "${name}" updated with improvements.`
func ResultMessage(skillName string) string {
	return `Skill "` + skillName + `" updated with improvements.`
}
