package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Source: services/autoDream/autoDream.ts

func TestAutoDreamConstants(t *testing.T) {
	// Source: autoDream.ts:56, 64-65
	if DefaultMinHours != 24 {
		t.Errorf("DefaultMinHours = %v, want 24", DefaultMinHours)
	}
	if DefaultMinSessions != 5 {
		t.Errorf("DefaultMinSessions = %d, want 5", DefaultMinSessions)
	}
	if SessionScanIntervalMs != 600_000 {
		t.Errorf("SessionScanIntervalMs = %d, want 600000", SessionScanIntervalMs)
	}
}

func TestShouldTriggerAutoDream(t *testing.T) {
	// Source: autoDream.ts:125-169

	t.Run("fires_when_thresholds_met", func(t *testing.T) {
		dir := t.TempDir()
		// Create 6 session files modified recently
		for i := 0; i < 6; i++ {
			os.WriteFile(filepath.Join(dir, "sess-"+string(rune('a'+i))+".jsonl"), []byte("{}"), 0644)
		}

		state := &AutoDreamState{
			LastConsolidatedAt: time.Now().Add(-25 * time.Hour), // >24h ago
		}
		cfg := DefaultAutoDreamConfig()

		if !ShouldTriggerAutoDream(state, cfg, dir, "current-session") {
			t.Error("should trigger: 25h > 24h minimum, 6 sessions >= 5 minimum")
		}
	})

	t.Run("skips_when_too_recent", func(t *testing.T) {
		// Source: autoDream.ts:141
		dir := t.TempDir()
		state := &AutoDreamState{
			LastConsolidatedAt: time.Now().Add(-1 * time.Hour), // Only 1h ago
		}
		cfg := DefaultAutoDreamConfig()

		if ShouldTriggerAutoDream(state, cfg, dir, "") {
			t.Error("should skip: 1h < 24h minimum")
		}
	})

	t.Run("skips_when_too_few_sessions", func(t *testing.T) {
		// Source: autoDream.ts:166-169
		dir := t.TempDir()
		// Create only 3 sessions
		for i := 0; i < 3; i++ {
			os.WriteFile(filepath.Join(dir, "sess-"+string(rune('a'+i))+".jsonl"), []byte("{}"), 0644)
		}

		state := &AutoDreamState{
			LastConsolidatedAt: time.Now().Add(-30 * time.Hour),
		}
		cfg := DefaultAutoDreamConfig()

		if ShouldTriggerAutoDream(state, cfg, dir, "") {
			t.Error("should skip: 3 sessions < 5 minimum")
		}
	})

	t.Run("excludes_current_session", func(t *testing.T) {
		// Source: autoDream.ts:164-165
		dir := t.TempDir()
		// Create exactly 5 sessions including current
		for i := 0; i < 5; i++ {
			os.WriteFile(filepath.Join(dir, "sess-"+string(rune('a'+i))+".jsonl"), []byte("{}"), 0644)
		}

		state := &AutoDreamState{
			LastConsolidatedAt: time.Now().Add(-30 * time.Hour),
		}
		cfg := DefaultAutoDreamConfig()

		// Current session is one of the 5 → only 4 non-current → should NOT fire
		if ShouldTriggerAutoDream(state, cfg, dir, "sess-a") {
			t.Error("should skip: current session excluded, only 4 sessions")
		}
	})

	t.Run("scan_throttle", func(t *testing.T) {
		// Source: autoDream.ts:144-150
		dir := t.TempDir()
		for i := 0; i < 6; i++ {
			os.WriteFile(filepath.Join(dir, "sess-"+string(rune('a'+i))+".jsonl"), []byte("{}"), 0644)
		}

		state := &AutoDreamState{
			LastConsolidatedAt: time.Now().Add(-30 * time.Hour),
			LastScanAt:         time.Now(), // Just scanned
		}
		cfg := DefaultAutoDreamConfig()

		if ShouldTriggerAutoDream(state, cfg, dir, "") {
			t.Error("should skip: scan throttle active")
		}
	})
}

func TestReadLastConsolidatedAt(t *testing.T) {
	t.Run("missing_file_returns_zero", func(t *testing.T) {
		result := ReadLastConsolidatedAt(t.TempDir())
		if !result.IsZero() {
			t.Error("expected zero time for missing file")
		}
	})

	t.Run("reads_written_timestamp", func(t *testing.T) {
		dir := t.TempDir()
		RecordConsolidation(dir)

		result := ReadLastConsolidatedAt(dir)
		if result.IsZero() {
			t.Fatal("expected non-zero time")
		}
		if time.Since(result) > 5*time.Second {
			t.Error("timestamp should be recent")
		}
	})
}
