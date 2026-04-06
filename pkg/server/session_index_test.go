package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// T93: SessionIndexStore persistence integration tests
// ---------------------------------------------------------------------------

func TestSessionIndexStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionIndexStore(dir)

	idx, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(idx) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx))
	}
}

func TestSessionIndexStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionIndexStore(dir)

	idx := SessionIndex{
		"key-1": {
			SessionID:           "sess-1",
			TranscriptSessionID: "transcript-1",
			CWD:                 "/home/user/project",
			PermissionMode:      "auto",
			CreatedAt:           1700000000000,
			LastActiveAt:        1700000001000,
		},
		"key-2": {
			SessionID:           "sess-2",
			TranscriptSessionID: "sess-2",
			CWD:                 "/tmp",
			CreatedAt:           1700000002000,
			LastActiveAt:        1700000003000,
		},
	}

	if err := store.Save(idx); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists at expected path.
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("session index file missing: %v", err)
	}

	// Load and verify round-trip.
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got["key-1"].SessionID != "sess-1" {
		t.Errorf("key-1 SessionID = %q, want %q", got["key-1"].SessionID, "sess-1")
	}
	if got["key-2"].CWD != "/tmp" {
		t.Errorf("key-2 CWD = %q, want %q", got["key-2"].CWD, "/tmp")
	}
}

func TestSessionIndexStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionIndexStore(dir)

	// Save an initial index.
	idx := SessionIndex{
		"k": {SessionID: "s1", CWD: "/a", CreatedAt: 1, LastActiveAt: 1},
	}
	if err := store.Save(idx); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Read raw file and verify it's valid JSON.
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var parsed SessionIndex
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("file is not valid JSON: %v", err)
	}
	if parsed["k"].SessionID != "s1" {
		t.Errorf("parsed SessionID = %q, want %q", parsed["k"].SessionID, "s1")
	}

	// Verify no temp files are left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != sessionIndexFile {
			t.Errorf("unexpected file in dir: %s", e.Name())
		}
	}
}

func TestSessionIndexStore_Put(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionIndexStore(dir)

	entry := SessionIndexEntry{
		SessionID:           "sess-new",
		TranscriptSessionID: "sess-new",
		CWD:                 "/work",
		CreatedAt:           5000,
		LastActiveAt:        6000,
	}

	if err := store.Put("new-key", entry); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	idx, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if idx["new-key"].SessionID != "sess-new" {
		t.Errorf("Put() entry not persisted correctly")
	}
}

func TestSessionIndexStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionIndexStore(dir)

	idx := SessionIndex{
		"a": {SessionID: "s-a", CWD: "/a", CreatedAt: 1, LastActiveAt: 1},
		"b": {SessionID: "s-b", CWD: "/b", CreatedAt: 2, LastActiveAt: 2},
	}
	if err := store.Save(idx); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	existed, err := store.Delete("a")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if !existed {
		t.Error("Delete() returned false for existing key")
	}

	got, _ := store.Load()
	if len(got) != 1 {
		t.Errorf("expected 1 entry after delete, got %d", len(got))
	}
	if _, ok := got["a"]; ok {
		t.Error("key 'a' should have been deleted")
	}

	// Delete non-existent key.
	existed, err = store.Delete("nonexistent")
	if err != nil {
		t.Fatalf("Delete(nonexistent) error: %v", err)
	}
	if existed {
		t.Error("Delete() returned true for non-existent key")
	}
}

func TestSessionIndexStore_CreatesDirIfNeeded(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep")
	store := NewSessionIndexStore(dir)

	idx := SessionIndex{"k": {SessionID: "s", CWD: "/", CreatedAt: 1, LastActiveAt: 1}}
	if err := store.Save(idx); err != nil {
		t.Fatalf("Save() should create nested dirs: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 entry, got %d", len(got))
	}
}

func TestSessionIndexStore_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionIndexStore(dir)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			entry := SessionIndexEntry{
				SessionID:    "sess",
				CWD:          "/",
				CreatedAt:    int64(n),
				LastActiveAt: int64(n),
			}
			if err := store.Put("key", entry); err != nil {
				t.Errorf("concurrent Put() error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	idx, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, ok := idx["key"]; !ok {
		t.Error("expected 'key' in index after concurrent writes")
	}
}

func TestSessionIndexStore_PathMatchesConvention(t *testing.T) {
	store := NewSessionIndexStore("/home/user/.claude")
	want := "/home/user/.claude/server-sessions.json"
	if store.Path() != want {
		t.Errorf("Path() = %q, want %q", store.Path(), want)
	}
}

func TestSessionIndexStore_CorruptFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionIndexStore(dir)

	// Write invalid JSON.
	os.WriteFile(store.Path(), []byte("{bad json"), 0o644)

	_, err := store.Load()
	if err == nil {
		t.Fatal("Load() should return error for corrupt JSON")
	}
}
