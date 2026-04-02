package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: utils/sessionStorage.ts

func TestTranscriptWriter(t *testing.T) {

	t.Run("append_and_read_jsonl", func(t *testing.T) {
		// Source: utils/sessionStorage.ts:2572-2584
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")
		w := NewTranscriptWriter(path)

		// Write a message entry
		msg := message.UserMessage("hello world")
		err := w.AppendMessage("sess-1", msg)
		if err != nil {
			t.Fatalf("failed to append message: %v", err)
		}

		// Write a usage entry
		err = w.AppendUsage("sess-1", 100, 50, 1)
		if err != nil {
			t.Fatalf("failed to append usage: %v", err)
		}

		// Write a title entry
		err = w.AppendTitle("sess-1", "Test Session")
		if err != nil {
			t.Fatalf("failed to append title: %v", err)
		}

		// Read back
		entries, err := ReadTranscript(path)
		if err != nil {
			t.Fatalf("failed to read transcript: %v", err)
		}
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}

		// Verify message entry
		if entries[0].Type != "message" {
			t.Errorf("entry[0] type = %q, want 'message'", entries[0].Type)
		}
		if entries[0].SessionID != "sess-1" {
			t.Errorf("entry[0] sessionId = %q, want 'sess-1'", entries[0].SessionID)
		}
		if entries[0].Message == nil {
			t.Fatal("entry[0] message is nil")
		}
		if entries[0].Message.Role != message.RoleUser {
			t.Errorf("message role = %q, want 'user'", entries[0].Message.Role)
		}

		// Verify usage entry
		if entries[1].Type != "usage" {
			t.Errorf("entry[1] type = %q, want 'usage'", entries[1].Type)
		}
		if entries[1].InputTokens != 100 {
			t.Errorf("inputTokens = %d, want 100", entries[1].InputTokens)
		}
		if entries[1].OutputTokens != 50 {
			t.Errorf("outputTokens = %d, want 50", entries[1].OutputTokens)
		}

		// Verify title entry
		// Source: utils/sessionStorage.ts:777-781
		if entries[2].Type != "custom-title" {
			t.Errorf("entry[2] type = %q, want 'custom-title'", entries[2].Type)
		}
		if entries[2].CustomTitle != "Test Session" {
			t.Errorf("customTitle = %q, want 'Test Session'", entries[2].CustomTitle)
		}
	})

	t.Run("creates_parent_directory", func(t *testing.T) {
		// Source: utils/sessionStorage.ts:2580-2582
		dir := t.TempDir()
		path := filepath.Join(dir, "subdir", "nested", "session.jsonl")
		w := NewTranscriptWriter(path)

		err := w.AppendMessage("sess-1", message.UserMessage("test"))
		if err != nil {
			t.Fatalf("failed to append with nested dirs: %v", err)
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("file should have been created")
		}
	})

	t.Run("file_permissions_0600", func(t *testing.T) {
		// Source: utils/sessionStorage.ts:2579
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")
		w := NewTranscriptWriter(path)

		w.AppendMessage("sess-1", message.UserMessage("test"))

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("expected 0600 permissions, got %o", perm)
		}
	})

	t.Run("jsonl_format_one_json_per_line", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")
		w := NewTranscriptWriter(path)

		w.AppendMessage("s1", message.UserMessage("first"))
		w.AppendMessage("s1", message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{message.TextBlock("response")},
		})

		data, _ := os.ReadFile(path)
		lines := 0
		for _, b := range data {
			if b == '\n' {
				lines++
			}
		}
		if lines != 2 {
			t.Errorf("expected 2 lines (one per entry), got %d", lines)
		}
	})

	t.Run("skip_malformed_lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")

		// Write valid + invalid + valid
		os.WriteFile(path, []byte(
			`{"type":"message","role":"user"}`+"\n"+
				`not json at all`+"\n"+
				`{"type":"usage","inputTokens":5}`+"\n",
		), 0600)

		entries, err := ReadTranscript(path)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 valid entries (skip malformed), got %d", len(entries))
		}
	})
}

func TestRestoreMessages(t *testing.T) {
	entries := []TranscriptEntry{
		{Type: "message", Message: &message.Message{Role: message.RoleUser, Content: []message.ContentBlock{message.TextBlock("hello")}}},
		{Type: "usage", InputTokens: 100},
		{Type: "message", Message: &message.Message{Role: message.RoleAssistant, Content: []message.ContentBlock{message.TextBlock("hi")}}},
		{Type: "custom-title", CustomTitle: "test"},
	}

	msgs := RestoreMessages(entries)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != message.RoleUser {
		t.Errorf("msg[0] role = %q, want 'user'", msgs[0].Role)
	}
	if msgs[1].Role != message.RoleAssistant {
		t.Errorf("msg[1] role = %q, want 'assistant'", msgs[1].Role)
	}
}

func TestTranscriptPathForSession(t *testing.T) {
	// Source: utils/sessionStorage.ts:204
	path := TranscriptPathForSession("/home/user/.claude/projects/myapp", "abc-123")
	expected := filepath.Join("/home/user/.claude/projects/myapp", "abc-123.jsonl")
	if path != expected {
		t.Errorf("got %q, want %q", path, expected)
	}
}
