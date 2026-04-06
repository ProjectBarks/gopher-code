package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/session"
)

func setupPointerTest(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	session.SetHomeDirForTest(tmp)
	return tmp
}

func TestWriteAndReadPointer(t *testing.T) {
	setupPointerTest(t)

	dir := "/projects/my-app"
	ptr := BridgePointer{
		SessionID:     "sess-123",
		EnvironmentID: "env-456",
		Source:        PointerSourceStandalone,
	}

	if err := WriteBridgePointer(dir, ptr, nil); err != nil {
		t.Fatalf("WriteBridgePointer: %v", err)
	}

	got := ReadBridgePointer(dir, nil)
	if got == nil {
		t.Fatal("ReadBridgePointer returned nil")
	}
	if got.SessionID != ptr.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, ptr.SessionID)
	}
	if got.EnvironmentID != ptr.EnvironmentID {
		t.Errorf("EnvironmentID = %q, want %q", got.EnvironmentID, ptr.EnvironmentID)
	}
	if got.Source != ptr.Source {
		t.Errorf("Source = %q, want %q", got.Source, ptr.Source)
	}
	if got.AgeMs < 0 {
		t.Errorf("AgeMs = %d, want >= 0", got.AgeMs)
	}
}

func TestClearPointer(t *testing.T) {
	setupPointerTest(t)

	dir := "/projects/my-app"
	ptr := BridgePointer{
		SessionID:     "sess-123",
		EnvironmentID: "env-456",
		Source:        PointerSourceREPL,
	}

	if err := WriteBridgePointer(dir, ptr, nil); err != nil {
		t.Fatalf("WriteBridgePointer: %v", err)
	}

	// Verify it exists.
	if got := ReadBridgePointer(dir, nil); got == nil {
		t.Fatal("pointer should exist before clear")
	}

	ClearBridgePointer(dir, nil)

	// Verify it's gone.
	if got := ReadBridgePointer(dir, nil); got != nil {
		t.Error("pointer should be nil after clear")
	}
}

func TestClearPointerIdempotent(t *testing.T) {
	setupPointerTest(t)

	// Clearing a non-existent pointer should not panic or error.
	ClearBridgePointer("/nonexistent/path", nil)
}

func TestPointerFileFormat(t *testing.T) {
	setupPointerTest(t)

	dir := "/projects/my-app"
	ptr := BridgePointer{
		SessionID:     "sess-abc",
		EnvironmentID: "env-def",
		Source:        PointerSourceStandalone,
	}

	if err := WriteBridgePointer(dir, ptr, nil); err != nil {
		t.Fatalf("WriteBridgePointer: %v", err)
	}

	path := GetBridgePointerPath(dir)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Verify it's valid JSON with the expected keys.
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if m["sessionId"] != "sess-abc" {
		t.Errorf("sessionId = %v, want %q", m["sessionId"], "sess-abc")
	}
	if m["environmentId"] != "env-def" {
		t.Errorf("environmentId = %v, want %q", m["environmentId"], "env-def")
	}
	if m["source"] != "standalone" {
		t.Errorf("source = %v, want %q", m["source"], "standalone")
	}

	// Must have exactly 3 keys (no extra fields).
	if len(m) != 3 {
		t.Errorf("expected 3 JSON keys, got %d: %v", len(m), m)
	}
}

func TestReadPointerMissingFile(t *testing.T) {
	setupPointerTest(t)

	got := ReadBridgePointer("/no/such/dir", nil)
	if got != nil {
		t.Error("expected nil for missing pointer file")
	}
}

func TestReadPointerInvalidJSON(t *testing.T) {
	setupPointerTest(t)

	dir := "/projects/corrupt"
	path := GetBridgePointerPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json}"), 0600); err != nil {
		t.Fatal(err)
	}

	got := ReadBridgePointer(dir, nil)
	if got != nil {
		t.Error("expected nil for corrupt JSON")
	}

	// File should have been cleared.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("corrupt pointer file should have been deleted")
	}
}

func TestReadPointerInvalidSource(t *testing.T) {
	setupPointerTest(t)

	dir := "/projects/badsource"
	path := GetBridgePointerPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := `{"sessionId":"s","environmentId":"e","source":"bogus"}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	got := ReadBridgePointer(dir, nil)
	if got != nil {
		t.Error("expected nil for invalid source")
	}
}

func TestReadPointerStale(t *testing.T) {
	setupPointerTest(t)

	dir := "/projects/stale"
	ptr := BridgePointer{
		SessionID:     "sess-old",
		EnvironmentID: "env-old",
		Source:        PointerSourceStandalone,
	}

	if err := WriteBridgePointer(dir, ptr, nil); err != nil {
		t.Fatalf("WriteBridgePointer: %v", err)
	}

	// Backdate the file mtime to 5 hours ago.
	path := GetBridgePointerPath(dir)
	old := time.Now().Add(-5 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	got := ReadBridgePointer(dir, nil)
	if got != nil {
		t.Error("expected nil for stale pointer")
	}

	// File should have been cleared.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("stale pointer file should have been deleted")
	}
}

func TestWritePointerValidation(t *testing.T) {
	setupPointerTest(t)

	tests := []struct {
		name string
		ptr  BridgePointer
	}{
		{"empty sessionId", BridgePointer{SessionID: "", EnvironmentID: "e", Source: PointerSourceStandalone}},
		{"empty environmentId", BridgePointer{SessionID: "s", EnvironmentID: "", Source: PointerSourceStandalone}},
		{"invalid source", BridgePointer{SessionID: "s", EnvironmentID: "e", Source: "bad"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteBridgePointer("/tmp/test", tt.ptr, nil)
			if err == nil {
				t.Error("expected error for invalid pointer")
			}
		})
	}
}

func TestGetBridgePointerPath(t *testing.T) {
	setupPointerTest(t)

	path := GetBridgePointerPath("/Users/foo/my-project")
	if filepath.Base(path) != "bridge-pointer.json" {
		t.Errorf("expected bridge-pointer.json, got %s", filepath.Base(path))
	}
	// Should be under the projects dir.
	projectsDir := session.GetProjectsDir()
	rel, err := filepath.Rel(projectsDir, path)
	if err != nil || rel == path {
		t.Errorf("path %q should be under projects dir %q", path, projectsDir)
	}
}

func TestPointerSourceConstants(t *testing.T) {
	if PointerSourceStandalone != "standalone" {
		t.Errorf("PointerSourceStandalone = %q, want %q", PointerSourceStandalone, "standalone")
	}
	if PointerSourceREPL != "repl" {
		t.Errorf("PointerSourceREPL = %q, want %q", PointerSourceREPL, "repl")
	}
}

func TestBridgePointerTTL(t *testing.T) {
	if BridgePointerTTL != 4*time.Hour {
		t.Errorf("BridgePointerTTL = %v, want 4h", BridgePointerTTL)
	}
}
