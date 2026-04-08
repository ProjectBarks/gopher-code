package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/session"
	sessionhooks "github.com/projectbarks/gopher-code/pkg/ui/hooks/session"
)

// TestSessionHooks_MessageLoggerIntegration exercises the MessageLogger through
// the same code path wired in main(): create a logger with a real transcript
// path, log messages, and verify the JSONL transcript on disk.
func TestSessionHooks_MessageLoggerIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	transcriptPath := session.GetTranscriptPath(projectDir, "test-sess-001")
	logger := sessionhooks.NewMessageLogger(transcriptPath, "test-sess-001")

	// Log a user message, then an assistant reply (incremental).
	msgs := []message.Message{message.UserMessage("hello")}
	cmd := logger.LogMessages(msgs)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for first message")
	}
	result := cmd()
	logMsg, ok := result.(sessionhooks.LogMessagesMsg)
	if !ok {
		t.Fatalf("expected LogMessagesMsg, got %T", result)
	}
	if logMsg.Err != nil {
		t.Fatalf("unexpected error: %v", logMsg.Err)
	}
	if logMsg.Count != 1 {
		t.Errorf("count = %d, want 1", logMsg.Count)
	}

	// Incremental: add assistant response.
	msgs = append(msgs, message.Message{
		Role:    message.RoleAssistant,
		Content: []message.ContentBlock{{Type: message.ContentText, Text: "hi there"}},
	})
	cmd = logger.LogMessages(msgs)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for incremental message")
	}
	result = cmd()
	logMsg = result.(sessionhooks.LogMessagesMsg)
	if logMsg.Count != 1 {
		t.Errorf("count = %d, want 1 (incremental)", logMsg.Count)
	}

	// Verify transcript has 2 entries via the real session.ReadTranscript path.
	entries, err := session.ReadTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ReadTranscript: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Message.Content[0].Text != "hello" {
		t.Errorf("entry[0] text = %q, want %q", entries[0].Message.Content[0].Text, "hello")
	}
	if entries[1].Message.Content[0].Text != "hi there" {
		t.Errorf("entry[1] text = %q, want %q", entries[1].Message.Content[0].Text, "hi there")
	}

	// Ignore mode suppresses recording (same as --print or --no-session-persistence).
	logger.SetIgnore(true)
	cmd = logger.LogMessages(append(msgs, message.UserMessage("secret")))
	if cmd != nil {
		t.Error("expected nil cmd when ignore=true")
	}
}

// TestSessionHooks_FileHistoryIntegration exercises the FileHistoryManager
// through the same code path wired in main(): track files, snapshot, modify,
// and restore.
func TestSessionHooks_FileHistoryIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "sess_file_history")
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	mgr := sessionhooks.NewFileHistoryManager(backupDir, true)

	testFile := filepath.Join(workDir, "code.go")
	if err := os.WriteFile(testFile, []byte("package main // v1"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr.TrackFile(testFile)
	snap, err := mgr.MakeSnapshot("msg-turn-1")
	if err != nil {
		t.Fatalf("MakeSnapshot: %v", err)
	}
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}

	// Modify the file.
	if err := os.WriteFile(testFile, []byte("package main // v2 modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Restore to v1.
	if err := mgr.RestoreSnapshot(snap); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}
	data, _ := os.ReadFile(testFile)
	if string(data) != "package main // v1" {
		t.Errorf("restored content = %q, want %q", string(data), "package main // v1")
	}

	// Disabled manager returns nil snapshot (same as --no-session-persistence).
	disabled := sessionhooks.NewFileHistoryManager(backupDir, false)
	snap2, err := disabled.MakeSnapshot("msg-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap2 != nil {
		t.Error("expected nil snapshot when disabled")
	}
}

// TestSessionHooks_BackgrounderIntegration exercises SessionBackgrounder
// through the same code path wired in main().
func TestSessionHooks_BackgrounderIntegration(t *testing.T) {
	spawned := false
	bgr := sessionhooks.NewSessionBackgrounder(func() string {
		spawned = true
		return "bg-task-001"
	})

	bgr.HandleBackground()
	if !spawned {
		t.Error("expected OnBackground callback to be invoked")
	}

	tasks := bgr.Tasks()
	if len(tasks) != 1 {
		t.Fatalf("tasks count = %d, want 1", len(tasks))
	}
	task := tasks["bg-task-001"]
	if task == nil {
		t.Fatal("expected bg-task-001 in tasks")
	}
	if task.Status != sessionhooks.TaskRunning {
		t.Errorf("task status = %d, want TaskRunning", task.Status)
	}

	// Foreground then re-background via HandleBackground.
	bgr.Foreground("bg-task-001")
	if bgr.ForegroundedTaskID() != "bg-task-001" {
		t.Errorf("foregroundedID = %q, want %q", bgr.ForegroundedTaskID(), "bg-task-001")
	}
	bgr.HandleBackground()
	if bgr.ForegroundedTaskID() != "" {
		t.Error("expected empty foregroundedID after re-background")
	}
}
