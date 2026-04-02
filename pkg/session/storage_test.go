package session

import (
	"os"
	"testing"
	"time"
)

func setupTestHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	origFn := homeDirFn
	homeDirFn = func() (string, error) { return dir, nil }
	t.Cleanup(func() { homeDirFn = origFn })
	return dir
}

func TestSaveAndLoad(t *testing.T) {
	setupTestHome(t)

	s := New(DefaultConfig(), "/tmp/project")
	s.TurnCount = 5
	s.TotalInputTokens = 1000
	s.TotalOutputTokens = 500

	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(s.ID)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.ID != s.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, s.ID)
	}
	if loaded.CWD != s.CWD {
		t.Errorf("CWD = %q, want %q", loaded.CWD, s.CWD)
	}
	if loaded.TurnCount != s.TurnCount {
		t.Errorf("TurnCount = %d, want %d", loaded.TurnCount, s.TurnCount)
	}
	if loaded.TotalInputTokens != s.TotalInputTokens {
		t.Errorf("TotalInputTokens = %d, want %d", loaded.TotalInputTokens, s.TotalInputTokens)
	}
	if loaded.TotalOutputTokens != s.TotalOutputTokens {
		t.Errorf("TotalOutputTokens = %d, want %d", loaded.TotalOutputTokens, s.TotalOutputTokens)
	}
	if loaded.Config.Model != s.Config.Model {
		t.Errorf("Config.Model = %q, want %q", loaded.Config.Model, s.Config.Model)
	}
	if loaded.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestLoadLatest(t *testing.T) {
	setupTestHome(t)

	cwd := "/tmp/project"

	// Create older session
	older := New(DefaultConfig(), cwd)
	older.CreatedAt = time.Now().Add(-2 * time.Hour)
	older.TurnCount = 1
	if err := older.Save(); err != nil {
		t.Fatalf("Save older: %v", err)
	}

	// Small delay so UpdatedAt differs
	time.Sleep(10 * time.Millisecond)

	// Create newer session
	newer := New(DefaultConfig(), cwd)
	newer.TurnCount = 10
	if err := newer.Save(); err != nil {
		t.Fatalf("Save newer: %v", err)
	}

	loaded, err := LoadLatest(cwd)
	if err != nil {
		t.Fatalf("LoadLatest() error: %v", err)
	}

	if loaded.ID != newer.ID {
		t.Errorf("LoadLatest returned ID %q, want %q (newer)", loaded.ID, newer.ID)
	}
	if loaded.TurnCount != 10 {
		t.Errorf("TurnCount = %d, want 10", loaded.TurnCount)
	}
}

func TestLoadLatestFiltersByCWD(t *testing.T) {
	setupTestHome(t)

	// Session in /tmp/a
	sA := New(DefaultConfig(), "/tmp/a")
	sA.TurnCount = 1
	if err := sA.Save(); err != nil {
		t.Fatalf("Save sA: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Session in /tmp/b (more recent)
	sB := New(DefaultConfig(), "/tmp/b")
	sB.TurnCount = 2
	if err := sB.Save(); err != nil {
		t.Fatalf("Save sB: %v", err)
	}

	// Load latest for /tmp/a should return sA, not sB
	loaded, err := LoadLatest("/tmp/a")
	if err != nil {
		t.Fatalf("LoadLatest(/tmp/a) error: %v", err)
	}
	if loaded.ID != sA.ID {
		t.Errorf("LoadLatest(/tmp/a) returned ID %q, want %q", loaded.ID, sA.ID)
	}

	// Load latest for /tmp/b should return sB
	loaded, err = LoadLatest("/tmp/b")
	if err != nil {
		t.Fatalf("LoadLatest(/tmp/b) error: %v", err)
	}
	if loaded.ID != sB.ID {
		t.Errorf("LoadLatest(/tmp/b) returned ID %q, want %q", loaded.ID, sB.ID)
	}

	// Load latest with empty CWD should return sB (most recent overall)
	loaded, err = LoadLatest("")
	if err != nil {
		t.Fatalf("LoadLatest('') error: %v", err)
	}
	if loaded.ID != sB.ID {
		t.Errorf("LoadLatest('') returned ID %q, want %q", loaded.ID, sB.ID)
	}
}

func TestListSessions(t *testing.T) {
	setupTestHome(t)

	s1 := New(DefaultConfig(), "/tmp/a")
	if err := s1.Save(); err != nil {
		t.Fatalf("Save s1: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	s2 := New(DefaultConfig(), "/tmp/b")
	if err := s2.Save(); err != nil {
		t.Fatalf("Save s2: %v", err)
	}

	metas, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("ListSessions() returned %d items, want 2", len(metas))
	}

	// Should be sorted by UpdatedAt descending (s2 first)
	if metas[0].ID != s2.ID {
		t.Errorf("metas[0].ID = %q, want %q (most recent)", metas[0].ID, s2.ID)
	}
	if metas[1].ID != s1.ID {
		t.Errorf("metas[1].ID = %q, want %q (older)", metas[1].ID, s1.ID)
	}
}

func TestLoadNonexistent(t *testing.T) {
	setupTestHome(t)

	_, err := Load("nonexistent-id")
	if err == nil {
		t.Fatal("Load(nonexistent) should return error")
	}
}

func TestLoadLatestNoSessions(t *testing.T) {
	dir := t.TempDir()
	origFn := homeDirFn
	homeDirFn = func() (string, error) { return dir, nil }
	t.Cleanup(func() { homeDirFn = origFn })

	// Create the sessions directory but leave it empty
	if err := os.MkdirAll(dir+"/.claude/sessions", 0700); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLatest("/tmp/project")
	if err == nil {
		t.Fatal("LoadLatest should return error when no sessions exist")
	}
}
