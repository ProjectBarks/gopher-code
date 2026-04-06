package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: utils/sessionStorage.ts + utils/listSessionsImpl.ts

func TestGetSessionFilesWithMtime(t *testing.T) {
	// Source: sessionStorage.ts:4526-4569

	t.Run("returns_uuid_named_jsonl_files", func(t *testing.T) {
		dir := t.TempDir()

		// Create valid session files
		uuid1 := "550e8400-e29b-41d4-a716-446655440000"
		uuid2 := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
		os.WriteFile(filepath.Join(dir, uuid1+".jsonl"), []byte(`{"type":"message"}`+"\n"), 0600)
		os.WriteFile(filepath.Join(dir, uuid2+".jsonl"), []byte(`{"type":"message"}`+"\n"), 0600)

		// Create non-UUID files (should be skipped)
		os.WriteFile(filepath.Join(dir, "not-a-uuid.jsonl"), []byte("{}"), 0600)
		os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0600)

		// Create a subdirectory (should be skipped)
		os.MkdirAll(filepath.Join(dir, "subdir"), 0700)

		result, err := GetSessionFilesWithMtime(dir)
		if err != nil {
			t.Fatalf("GetSessionFilesWithMtime() error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 session files, got %d", len(result))
		}

		info1, ok := result[uuid1]
		if !ok {
			t.Fatalf("missing session %s", uuid1)
		}
		if info1.Path != filepath.Join(dir, uuid1+".jsonl") {
			t.Errorf("path = %q, want %q", info1.Path, filepath.Join(dir, uuid1+".jsonl"))
		}
		if info1.Size == 0 {
			t.Error("size should be > 0")
		}
		if info1.ModTime.IsZero() {
			t.Error("mtime should not be zero")
		}

		if _, ok := result[uuid2]; !ok {
			t.Fatalf("missing session %s", uuid2)
		}
	})

	t.Run("nonexistent_directory_returns_empty", func(t *testing.T) {
		result, err := GetSessionFilesWithMtime("/nonexistent/path/to/sessions")
		if err != nil {
			t.Fatalf("should not return error for nonexistent dir, got: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("empty_directory_returns_empty", func(t *testing.T) {
		dir := t.TempDir()

		result, err := GetSessionFilesWithMtime(dir)
		if err != nil {
			t.Fatalf("GetSessionFilesWithMtime() error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("skips_non_jsonl_files", func(t *testing.T) {
		dir := t.TempDir()
		uuid := "550e8400-e29b-41d4-a716-446655440000"
		os.WriteFile(filepath.Join(dir, uuid+".json"), []byte("{}"), 0600) // .json not .jsonl
		os.WriteFile(filepath.Join(dir, uuid+".txt"), []byte("hi"), 0600)

		result, err := GetSessionFilesWithMtime(dir)
		if err != nil {
			t.Fatalf("GetSessionFilesWithMtime() error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 (no .jsonl files), got %d", len(result))
		}
	})
}

func TestGetSessionIDFromLog(t *testing.T) {
	// Source: sessionStorage.ts:2927-2934

	t.Run("from_session_id_field", func(t *testing.T) {
		log := LogOption{SessionID: "abc-123"}
		got := GetSessionIDFromLog(log)
		if got != "abc-123" {
			t.Errorf("GetSessionIDFromLog() = %q, want %q", got, "abc-123")
		}
	})

	t.Run("from_first_message_entry", func(t *testing.T) {
		log := LogOption{
			Messages: []TranscriptEntry{
				{Type: "message", SessionID: "from-msg-1"},
				{Type: "message", SessionID: "from-msg-2"},
			},
		}
		got := GetSessionIDFromLog(log)
		if got != "from-msg-1" {
			t.Errorf("GetSessionIDFromLog() = %q, want %q", got, "from-msg-1")
		}
	})

	t.Run("empty_when_nothing_available", func(t *testing.T) {
		log := LogOption{}
		got := GetSessionIDFromLog(log)
		if got != "" {
			t.Errorf("GetSessionIDFromLog() = %q, want empty", got)
		}
	})

	t.Run("session_id_takes_priority_over_messages", func(t *testing.T) {
		log := LogOption{
			SessionID: "direct-id",
			Messages: []TranscriptEntry{
				{Type: "message", SessionID: "msg-id"},
			},
		}
		got := GetSessionIDFromLog(log)
		if got != "direct-id" {
			t.Errorf("GetSessionIDFromLog() = %q, want %q (direct ID preferred)", got, "direct-id")
		}
	})
}

func TestLoadAllLogsFromSessionFile(t *testing.T) {
	// Source: sessionStorage.ts:4598-4640

	t.Run("extracts_messages_and_metadata", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")
		w := NewTranscriptWriter(path)

		w.AppendMessage("sess-1", message.UserMessage("hello world"))
		w.AppendMessage("sess-1", message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{message.TextBlock("hi there")},
		})
		w.AppendTitle("sess-1", "My Session")
		w.Append(TranscriptEntry{Type: "cwd", SessionID: "sess-1", CWD: "/home/user/project"})
		w.Append(TranscriptEntry{Type: "tag", SessionID: "sess-1", Tag: "v1.0"})

		log, err := LoadAllLogsFromSessionFile(path)
		if err != nil {
			t.Fatalf("LoadAllLogsFromSessionFile() error: %v", err)
		}

		if log.SessionID != "sess-1" {
			t.Errorf("SessionID = %q, want %q", log.SessionID, "sess-1")
		}
		if len(log.Messages) != 2 {
			t.Errorf("expected 2 message entries, got %d", len(log.Messages))
		}
		if log.CustomTitle != "My Session" {
			t.Errorf("CustomTitle = %q, want %q", log.CustomTitle, "My Session")
		}
		if log.CWD != "/home/user/project" {
			t.Errorf("CWD = %q, want %q", log.CWD, "/home/user/project")
		}
		if log.Tag != "v1.0" {
			t.Errorf("Tag = %q, want %q", log.Tag, "v1.0")
		}
		if log.FirstPrompt != "hello world" {
			t.Errorf("FirstPrompt = %q, want %q", log.FirstPrompt, "hello world")
		}
	})

	t.Run("first_prompt_from_first_user_message", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")
		w := NewTranscriptWriter(path)

		// Assistant message first (no user prompt yet)
		w.AppendMessage("s1", message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{message.TextBlock("greeting")},
		})
		// Then user message
		w.AppendMessage("s1", message.UserMessage("first question"))
		// Another user message
		w.AppendMessage("s1", message.UserMessage("second question"))

		log, err := LoadAllLogsFromSessionFile(path)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if log.FirstPrompt != "first question" {
			t.Errorf("FirstPrompt = %q, want %q", log.FirstPrompt, "first question")
		}
	})

	t.Run("custom_title_uses_last_value", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")
		w := NewTranscriptWriter(path)

		w.AppendTitle("s1", "First Title")
		w.AppendTitle("s1", "Renamed Title")

		log, err := LoadAllLogsFromSessionFile(path)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if log.CustomTitle != "Renamed Title" {
			t.Errorf("CustomTitle = %q, want %q (last value wins)", log.CustomTitle, "Renamed Title")
		}
	})

	t.Run("missing_file_returns_error", func(t *testing.T) {
		_, err := LoadAllLogsFromSessionFile("/nonexistent/session.jsonl")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("empty_file_returns_empty_log", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		os.WriteFile(path, []byte{}, 0600)

		log, err := LoadAllLogsFromSessionFile(path)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if log.SessionID != "" {
			t.Errorf("SessionID should be empty, got %q", log.SessionID)
		}
		if len(log.Messages) != 0 {
			t.Errorf("Messages should be empty, got %d", len(log.Messages))
		}
	})
}

func TestSaveCustomTitle(t *testing.T) {
	// Source: sessionStorage.ts:2617-2638

	t.Run("appends_title_to_transcript", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")

		err := SaveCustomTitle("sess-1", "My Custom Title", path)
		if err != nil {
			t.Fatalf("SaveCustomTitle() error: %v", err)
		}

		// Read back and verify
		entries, err := ReadTranscript(path)
		if err != nil {
			t.Fatalf("ReadTranscript() error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Type != "custom-title" {
			t.Errorf("type = %q, want %q", entries[0].Type, "custom-title")
		}
		if entries[0].CustomTitle != "My Custom Title" {
			t.Errorf("customTitle = %q, want %q", entries[0].CustomTitle, "My Custom Title")
		}
		if entries[0].SessionID != "sess-1" {
			t.Errorf("sessionId = %q, want %q", entries[0].SessionID, "sess-1")
		}
	})

	t.Run("no_op_when_path_empty", func(t *testing.T) {
		err := SaveCustomTitle("sess-1", "title", "")
		if err != nil {
			t.Fatalf("SaveCustomTitle with empty path should be no-op, got: %v", err)
		}
	})
}
