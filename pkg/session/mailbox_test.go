package session

import (
	"testing"
)

// Source: utils/teammateMailbox.ts

func TestMailbox(t *testing.T) {

	t.Run("write_and_read", func(t *testing.T) {
		// Source: teammateMailbox.ts:84-108, 134-190
		dir := t.TempDir()
		mb := NewMailbox(dir)

		err := mb.WriteToMailbox("alice", "team1", "bob", "Hello Alice!", WithSummary("Greeting"))
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}

		messages, err := mb.ReadMailbox("alice", "team1")
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
		if messages[0].From != "bob" {
			t.Errorf("from = %q, want 'bob'", messages[0].From)
		}
		if messages[0].Text != "Hello Alice!" {
			t.Errorf("text = %q", messages[0].Text)
		}
		if messages[0].Read {
			t.Error("new messages should be unread")
		}
		if messages[0].Summary != "Greeting" {
			t.Errorf("summary = %q", messages[0].Summary)
		}
	})

	t.Run("read_unread_only", func(t *testing.T) {
		// Source: teammateMailbox.ts:115-125
		dir := t.TempDir()
		mb := NewMailbox(dir)

		mb.WriteToMailbox("alice", "team1", "bob", "msg1")
		mb.WriteToMailbox("alice", "team1", "carol", "msg2")
		mb.MarkAllRead("alice", "team1")
		mb.WriteToMailbox("alice", "team1", "dave", "msg3")

		unread, err := mb.ReadUnreadMessages("alice", "team1")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(unread) != 1 {
			t.Fatalf("expected 1 unread, got %d", len(unread))
		}
		if unread[0].From != "dave" {
			t.Errorf("unread from = %q, want 'dave'", unread[0].From)
		}
	})

	t.Run("empty_inbox", func(t *testing.T) {
		dir := t.TempDir()
		mb := NewMailbox(dir)

		messages, err := mb.ReadMailbox("nobody", "team1")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if messages != nil {
			t.Errorf("expected nil for empty inbox, got %v", messages)
		}
	})

	t.Run("multiple_messages_preserved", func(t *testing.T) {
		dir := t.TempDir()
		mb := NewMailbox(dir)

		mb.WriteToMailbox("alice", "team1", "bob", "first")
		mb.WriteToMailbox("alice", "team1", "carol", "second")
		mb.WriteToMailbox("alice", "team1", "dave", "third")

		messages, _ := mb.ReadMailbox("alice", "team1")
		if len(messages) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(messages))
		}
	})

	t.Run("different_teams_isolated", func(t *testing.T) {
		dir := t.TempDir()
		mb := NewMailbox(dir)

		mb.WriteToMailbox("alice", "team1", "bob", "for team1")
		mb.WriteToMailbox("alice", "team2", "bob", "for team2")

		msgs1, _ := mb.ReadMailbox("alice", "team1")
		msgs2, _ := mb.ReadMailbox("alice", "team2")

		if len(msgs1) != 1 || len(msgs2) != 1 {
			t.Errorf("teams should be isolated: team1=%d, team2=%d", len(msgs1), len(msgs2))
		}
	})

	t.Run("with_color_option", func(t *testing.T) {
		// Source: teammateMailbox.ts:48
		dir := t.TempDir()
		mb := NewMailbox(dir)

		mb.WriteToMailbox("alice", "team1", "bob", "hi", WithColor("blue"))
		messages, _ := mb.ReadMailbox("alice", "team1")
		if messages[0].Color != "blue" {
			t.Errorf("color = %q, want 'blue'", messages[0].Color)
		}
	})

	t.Run("default_team_name", func(t *testing.T) {
		// Source: teammateMailbox.ts:57 — defaults to 'default'
		dir := t.TempDir()
		mb := NewMailbox(dir)

		mb.WriteToMailbox("alice", "", "bob", "hi")
		messages, _ := mb.ReadMailbox("alice", "")
		if len(messages) != 1 {
			t.Error("should use 'default' team when empty")
		}
	})
}
