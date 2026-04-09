package autodream

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldRun_NotEnoughTime(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0755)

	// Write recent consolidation timestamp
	writeLastConsolidatedAt(memDir, time.Now())

	s := New(memDir, Config{MinHours: 24, MinSessions: 5})
	if s.ShouldRun("current") {
		t.Error("should not run — consolidated recently")
	}
}

func TestShouldRun_NotEnoughSessions(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0755)

	// Write old consolidation timestamp (48h ago)
	writeLastConsolidatedAt(memDir, time.Now().Add(-48*time.Hour))

	// Create transcript dir with only 2 sessions
	transcriptDir := filepath.Join(dir, "transcripts")
	os.MkdirAll(transcriptDir, 0755)
	os.WriteFile(filepath.Join(transcriptDir, "sess1.jsonl"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(transcriptDir, "sess2.jsonl"), []byte("{}"), 0644)

	s := New(memDir, Config{MinHours: 24, MinSessions: 5})
	if s.ShouldRun("other") {
		t.Error("should not run — not enough sessions")
	}
}

func TestShouldRun_AllGatesPass(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0755)

	// Old consolidation
	writeLastConsolidatedAt(memDir, time.Now().Add(-48*time.Hour))

	// Create enough sessions
	transcriptDir := filepath.Join(dir, "transcripts")
	os.MkdirAll(transcriptDir, 0755)
	for i := 0; i < 6; i++ {
		os.WriteFile(filepath.Join(transcriptDir, "sess"+string(rune('a'+i))+".jsonl"), []byte("{}"), 0644)
	}

	s := New(memDir, Config{MinHours: 24, MinSessions: 5})
	if !s.ShouldRun("other") {
		t.Error("should run — all gates passed")
	}
}

func TestShouldRun_ExcludesCurrentSession(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	os.MkdirAll(memDir, 0755)
	writeLastConsolidatedAt(memDir, time.Now().Add(-48*time.Hour))

	// Create sessions, all named "current"
	transcriptDir := filepath.Join(dir, "transcripts")
	os.MkdirAll(transcriptDir, 0755)
	for i := 0; i < 6; i++ {
		os.WriteFile(filepath.Join(transcriptDir, "current.jsonl"), []byte("{}"), 0644)
	}

	s := New(memDir, Config{MinHours: 24, MinSessions: 5})
	if s.ShouldRun("current") {
		t.Error("should not run — all sessions are current session")
	}
}

func TestShouldRun_Disabled(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	s := New(memDir, DefaultConfig)
	s.SetEnabled(false)
	if s.ShouldRun("x") {
		t.Error("should not run when disabled")
	}
}

func TestLock(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")

	// First acquire should succeed
	if !TryAcquireLock(memDir) {
		t.Error("first lock should succeed")
	}

	// Second acquire should fail (lock held)
	if TryAcquireLock(memDir) {
		t.Error("second lock should fail")
	}

	// Release
	ReleaseLock(memDir)

	// Now should succeed again
	if !TryAcquireLock(memDir) {
		t.Error("lock after release should succeed")
	}
	ReleaseLock(memDir)
}

func TestReadLastConsolidatedAt_Missing(t *testing.T) {
	ts, err := ReadLastConsolidatedAt(t.TempDir())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !ts.IsZero() {
		t.Error("should be zero time when never consolidated")
	}
}

func TestMarkComplete(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")
	TryAcquireLock(memDir)

	s := New(memDir, DefaultConfig)
	s.MarkComplete()

	// Timestamp should be recent
	ts, _ := ReadLastConsolidatedAt(memDir)
	if time.Since(ts) > time.Second {
		t.Error("timestamp should be recent after MarkComplete")
	}

	// Lock should be released
	if !TryAcquireLock(memDir) {
		t.Error("lock should be released after MarkComplete")
	}
	ReleaseLock(memDir)
}
