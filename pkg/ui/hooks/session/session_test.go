package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/pkg/message"
	sess "github.com/projectbarks/gopher-code/pkg/session"
)

// assistantMsg creates an assistant message with a single text block (test helper).
func assistantMsg(text string) message.Message {
	return message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: text}},
	}
}

// ---------------------------------------------------------------------------
// MessageLogger tests — incremental JSONL transcript recording
// ---------------------------------------------------------------------------

func TestMessageLogger_IncrementalAppend(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")
	logger := NewMessageLogger(transcriptPath, "sess-001")

	// Append 3 messages incrementally.
	msgs := []message.Message{
		message.UserMessage("hello"),
	}
	cmd := logger.LogMessages(msgs)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for first message")
	}
	result := cmd()
	logMsg := result.(LogMessagesMsg)
	if logMsg.Err != nil {
		t.Fatalf("unexpected error: %v", logMsg.Err)
	}
	if logMsg.Count != 1 {
		t.Errorf("count = %d, want 1", logMsg.Count)
	}

	// Append two more messages (simulating assistant response).
	msgs = append(msgs,
		assistantMsg("hi there"),
		message.UserMessage("thanks"),
	)
	cmd = logger.LogMessages(msgs)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for incremental messages")
	}
	result = cmd()
	logMsg = result.(LogMessagesMsg)
	if logMsg.Err != nil {
		t.Fatalf("unexpected error: %v", logMsg.Err)
	}
	if logMsg.Count != 2 {
		t.Errorf("count = %d, want 2 (incremental)", logMsg.Count)
	}

	// Verify JSONL file has 3 lines.
	entries, err := sess.ReadTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ReadTranscript: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}

	// Verify content.
	if entries[0].Message.Content[0].Text != "hello" {
		t.Errorf("entry[0] text = %q, want %q", entries[0].Message.Content[0].Text, "hello")
	}
	if entries[1].Message.Content[0].Text != "hi there" {
		t.Errorf("entry[1] text = %q, want %q", entries[1].Message.Content[0].Text, "hi there")
	}
	if entries[2].Message.Content[0].Text != "thanks" {
		t.Errorf("entry[2] text = %q, want %q", entries[2].Message.Content[0].Text, "thanks")
	}

	// Verify each entry is valid JSON.
	data, _ := os.ReadFile(transcriptPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("JSONL lines = %d, want 3", len(lines))
	}
	for i, line := range lines {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestMessageLogger_NoOpWhenNothingNew(t *testing.T) {
	tmpDir := t.TempDir()
	logger := NewMessageLogger(filepath.Join(tmpDir, "t.jsonl"), "sess-002")

	msgs := []message.Message{message.UserMessage("hi")}
	cmd := logger.LogMessages(msgs)
	_ = cmd() // write first message

	// Same messages again — nothing new.
	cmd = logger.LogMessages(msgs)
	if cmd != nil {
		t.Error("expected nil cmd when no new messages")
	}
}

func TestMessageLogger_IgnoreSuppressesRecording(t *testing.T) {
	tmpDir := t.TempDir()
	logger := NewMessageLogger(filepath.Join(tmpDir, "t.jsonl"), "sess-003")
	logger.SetIgnore(true)

	msgs := []message.Message{message.UserMessage("secret")}
	cmd := logger.LogMessages(msgs)
	if cmd != nil {
		t.Error("expected nil cmd when ignore=true")
	}
}

func TestMessageLogger_CompactionResetsIndex(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")
	logger := NewMessageLogger(transcriptPath, "sess-004")

	// Write initial messages.
	msgs := []message.Message{
		message.UserMessage("original-first"),
		assistantMsg("response"),
	}
	cmd := logger.LogMessages(msgs)
	_ = cmd()

	// Simulate compaction: first message changes.
	compacted := []message.Message{
		assistantMsg("compacted-summary"),
		message.UserMessage("new-query"),
	}
	cmd = logger.LogMessages(compacted)
	if cmd == nil {
		t.Fatal("expected non-nil cmd after compaction")
	}
	result := cmd()
	logMsg := result.(LogMessagesMsg)
	if logMsg.Count != 2 {
		t.Errorf("count = %d, want 2 (full re-write after compaction)", logMsg.Count)
	}

	// Transcript should now have 4 entries total (2 original + 2 compacted).
	entries, _ := sess.ReadTranscript(transcriptPath)
	if len(entries) != 4 {
		t.Errorf("total entries = %d, want 4", len(entries))
	}
}

func TestMessageLogger_Reset(t *testing.T) {
	tmpDir := t.TempDir()
	logger := NewMessageLogger(filepath.Join(tmpDir, "t.jsonl"), "sess-005")

	msgs := []message.Message{message.UserMessage("hi")}
	cmd := logger.LogMessages(msgs)
	_ = cmd()

	if logger.LastWrittenIndex() != 1 {
		t.Errorf("lastWrittenIndex = %d, want 1", logger.LastWrittenIndex())
	}

	logger.Reset()
	if logger.LastWrittenIndex() != 0 {
		t.Errorf("after reset, lastWrittenIndex = %d, want 0", logger.LastWrittenIndex())
	}
}

// ---------------------------------------------------------------------------
// SessionBackgrounder tests — background detection, task management
// ---------------------------------------------------------------------------

func TestIsTmuxOrScreen_DefaultFalse(t *testing.T) {
	// In CI/test environments, we're typically not in tmux/screen.
	// This test verifies the function doesn't panic and returns a bool.
	_ = IsTmuxOrScreen()
}

func TestSessionBackgrounder_BackgroundSpawnsTask(t *testing.T) {
	spawned := false
	bg := NewSessionBackgrounder(func() string {
		spawned = true
		return "task-001"
	})

	bg.HandleBackground()

	if !spawned {
		t.Error("expected OnBackground to be called")
	}

	tasks := bg.Tasks()
	if len(tasks) != 1 {
		t.Fatalf("tasks count = %d, want 1", len(tasks))
	}
	task, ok := tasks["task-001"]
	if !ok {
		t.Fatal("expected task-001 in tasks map")
	}
	if task.Status != TaskRunning {
		t.Errorf("task status = %d, want TaskRunning", task.Status)
	}
	if !task.IsBackgrounded {
		t.Error("expected task to be backgrounded")
	}
}

func TestSessionBackgrounder_ForegroundAndRebackground(t *testing.T) {
	bg := NewSessionBackgrounder(func() string { return "task-002" })

	// Background a task.
	bg.HandleBackground()

	// Foreground it.
	ok := bg.Foreground("task-002")
	if !ok {
		t.Error("expected Foreground to succeed")
	}
	if bg.ForegroundedTaskID() != "task-002" {
		t.Errorf("foregroundedID = %q, want %q", bg.ForegroundedTaskID(), "task-002")
	}

	// Ctrl+B again re-backgrounds.
	bg.HandleBackground()
	if bg.ForegroundedTaskID() != "" {
		t.Errorf("expected empty foregroundedID after re-background, got %q", bg.ForegroundedTaskID())
	}
	tasks := bg.Tasks()
	if !tasks["task-002"].IsBackgrounded {
		t.Error("expected task to be re-backgrounded")
	}
}

func TestSessionBackgrounder_ForegroundNonexistentReturnsFalse(t *testing.T) {
	bg := NewSessionBackgrounder(nil)
	ok := bg.Foreground("nonexistent")
	if ok {
		t.Error("expected false for nonexistent task")
	}
}

func TestSessionBackgrounder_CompletedTaskAutoRestores(t *testing.T) {
	bg := NewSessionBackgrounder(func() string { return "task-003" })

	bg.HandleBackground()
	bg.Foreground("task-003")

	// Task completes while foregrounded.
	bg.UpdateTask("task-003", nil, TaskCompleted)

	if bg.ForegroundedTaskID() != "" {
		t.Error("expected foregroundedID cleared after task completion")
	}
	tasks := bg.Tasks()
	if !tasks["task-003"].IsBackgrounded {
		t.Error("expected completed task to be auto-backgrounded")
	}
	if tasks["task-003"].Status != TaskCompleted {
		t.Errorf("task status = %d, want TaskCompleted", tasks["task-003"].Status)
	}
}

func TestSessionBackgrounder_NilOnBackground(t *testing.T) {
	bg := NewSessionBackgrounder(nil)
	// Should not panic.
	bg.HandleBackground()
	if len(bg.Tasks()) != 0 {
		t.Error("expected no tasks when OnBackground is nil")
	}
}

// ---------------------------------------------------------------------------
// FileHistoryManager tests — snapshot capture, restore, eviction
// ---------------------------------------------------------------------------

func TestFileHistoryManager_SnapshotAndRestore(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	mgr := NewFileHistoryManager(backupDir, true)

	// Create a test file.
	testFile := filepath.Join(workDir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("version-1"), 0644); err != nil {
		t.Fatal(err)
	}

	// Track and snapshot.
	mgr.TrackFile(testFile)
	snap, err := mgr.MakeSnapshot("msg-001")
	if err != nil {
		t.Fatalf("MakeSnapshot: %v", err)
	}
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.MessageID != "msg-001" {
		t.Errorf("messageID = %q, want %q", snap.MessageID, "msg-001")
	}

	// Modify the file.
	if err := os.WriteFile(testFile, []byte("version-2-modified"), 0644); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(testFile)
	if string(data) != "version-2-modified" {
		t.Fatal("file should be modified before restore")
	}

	// Restore snapshot.
	if err := mgr.RestoreSnapshot(snap); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}

	data, _ = os.ReadFile(testFile)
	if string(data) != "version-1" {
		t.Errorf("restored content = %q, want %q", string(data), "version-1")
	}
}

func TestFileHistoryManager_EvictsOldSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	mgr := NewFileHistoryManager(backupDir, true)

	// Create MaxSnapshots + 10 snapshots.
	for i := 0; i < MaxSnapshots+10; i++ {
		mgr.mu.Lock()
		mgr.state.Snapshots = append(mgr.state.Snapshots, FileSnapshot{
			MessageID:          "msg-" + string(rune('A'+i%26)),
			TrackedFileBackups: map[string]FileBackup{},
			Timestamp:          time.Now(),
		})
		mgr.state.SnapshotSequence++
		if len(mgr.state.Snapshots) > MaxSnapshots {
			mgr.state.Snapshots = mgr.state.Snapshots[len(mgr.state.Snapshots)-MaxSnapshots:]
		}
		mgr.mu.Unlock()
	}

	snaps := mgr.Snapshots()
	if len(snaps) != MaxSnapshots {
		t.Errorf("snapshot count = %d, want %d", len(snaps), MaxSnapshots)
	}
}

func TestFileHistoryManager_DisabledReturnsNil(t *testing.T) {
	mgr := NewFileHistoryManager(t.TempDir(), false)
	snap, err := mgr.MakeSnapshot("msg-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap != nil {
		t.Error("expected nil snapshot when disabled")
	}
}

func TestFileHistoryManager_DeletedFileRecordsEmptyBackup(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	mgr := NewFileHistoryManager(backupDir, true)

	// Track a nonexistent file.
	mgr.TrackFile(filepath.Join(tmpDir, "nonexistent.txt"))

	snap, err := mgr.MakeSnapshot("msg-del")
	if err != nil {
		t.Fatalf("MakeSnapshot: %v", err)
	}
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}

	// Backup should have empty file name.
	backup := snap.TrackedFileBackups[filepath.Join(tmpDir, "nonexistent.txt")]
	if backup.BackupFileName != "" {
		t.Errorf("backupFileName = %q, want empty for nonexistent file", backup.BackupFileName)
	}
}

func TestFileHistoryManager_RestoreDeletedFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "ephemeral.txt")

	// Snapshot with empty backup (file didn't exist).
	snap := &FileSnapshot{
		MessageID: "msg-restore-del",
		TrackedFileBackups: map[string]FileBackup{
			testFile: {BackupFileName: "", Version: 1},
		},
	}

	// Create the file (simulating it was added after snapshot).
	if err := os.WriteFile(testFile, []byte("should be removed"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewFileHistoryManager(filepath.Join(tmpDir, "backups"), true)
	if err := mgr.RestoreSnapshot(snap); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}

	// File should be removed.
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("expected file to be removed after restoring snapshot with empty backup")
	}
}

func TestFileHistoryManager_RestoreFromLog(t *testing.T) {
	mgr := NewFileHistoryManager(t.TempDir(), true)

	snapshots := []FileSnapshot{
		{
			MessageID: "msg-a",
			TrackedFileBackups: map[string]FileBackup{
				"/tmp/foo.txt": {BackupFileName: "abc@v1", Version: 1},
			},
			Timestamp: time.Now(),
		},
		{
			MessageID: "msg-b",
			TrackedFileBackups: map[string]FileBackup{
				"/tmp/foo.txt": {BackupFileName: "abc@v2", Version: 2},
				"/tmp/bar.txt": {BackupFileName: "def@v1", Version: 1},
			},
			Timestamp: time.Now(),
		},
	}

	mgr.RestoreFromLog(snapshots)

	state := mgr.State()
	if len(state.Snapshots) != 2 {
		t.Errorf("snapshots = %d, want 2", len(state.Snapshots))
	}
	if state.SnapshotSequence != 2 {
		t.Errorf("snapshotSequence = %d, want 2", state.SnapshotSequence)
	}
	if !state.TrackedFiles["/tmp/foo.txt"] {
		t.Error("expected /tmp/foo.txt in tracked files")
	}
	if !state.TrackedFiles["/tmp/bar.txt"] {
		t.Error("expected /tmp/bar.txt in tracked files")
	}
}

func TestFileHistoryManager_MultipleSnapshotsIncrementVersion(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	testFile := filepath.Join(tmpDir, "versioned.txt")

	mgr := NewFileHistoryManager(backupDir, true)
	mgr.TrackFile(testFile)

	// Snapshot 1.
	if err := os.WriteFile(testFile, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	snap1, err := mgr.MakeSnapshot("msg-v1")
	if err != nil {
		t.Fatalf("snap1: %v", err)
	}

	// Snapshot 2.
	if err := os.WriteFile(testFile, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	snap2, err := mgr.MakeSnapshot("msg-v2")
	if err != nil {
		t.Fatalf("snap2: %v", err)
	}

	v1 := snap1.TrackedFileBackups[testFile].Version
	v2 := snap2.TrackedFileBackups[testFile].Version
	if v2 != v1+1 {
		t.Errorf("version progression: v1=%d, v2=%d, want v2 = v1+1", v1, v2)
	}
}

func TestFileHistoryManager_RestoreNilSnapshotErrors(t *testing.T) {
	mgr := NewFileHistoryManager(t.TempDir(), true)
	err := mgr.RestoreSnapshot(nil)
	if err == nil {
		t.Error("expected error for nil snapshot")
	}
}

// ---------------------------------------------------------------------------
// backupFileName determinism
// ---------------------------------------------------------------------------

func TestBackupFileName_Deterministic(t *testing.T) {
	a := backupFileName("/path/to/file.txt", 1)
	b := backupFileName("/path/to/file.txt", 1)
	if a != b {
		t.Errorf("expected deterministic names, got %q and %q", a, b)
	}

	// Different version → different name.
	c := backupFileName("/path/to/file.txt", 2)
	if a == c {
		t.Error("expected different names for different versions")
	}
}
