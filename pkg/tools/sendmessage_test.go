package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// setupTeamDir creates a team directory with a team.json file for testing.
func setupTeamDir(t *testing.T, teamsDir, teamName string, members []session.TeamMember) {
	t.Helper()
	teamDir := filepath.Join(teamsDir, teamName)
	if err := os.MkdirAll(filepath.Join(teamDir, "inboxes"), 0755); err != nil {
		t.Fatal(err)
	}
	tf := session.TeamFile{
		Name:        teamName,
		LeadAgentID: "lead-001",
		Members:     members,
	}
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(teamDir, "team.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSendMessageTool(t *testing.T) {
	// Source: tools/SendMessageTool/SendMessageTool.ts

	tool := &tools.SendMessageTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "SendMessage" {
			t.Errorf("expected 'SendMessage', got %q", tool.Name())
		}
	})

	// Source: prompt.ts — DESCRIPTION = 'Send a message to another agent'
	t.Run("description_matches_ts", func(t *testing.T) {
		want := "Send a message to another agent"
		if tool.Description() != want {
			t.Errorf("Description() = %q, want %q", tool.Description(), want)
		}
	})

	// Source: SendMessageTool.ts:539 — isReadOnly is false (writes to mailbox)
	t.Run("is_not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("SendMessageTool should NOT be read-only (it writes to the mailbox)")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		// Source: SendMessageTool.ts:60-80
		for _, field := range []string{"to", "message", "summary"} {
			if _, ok := props[field]; !ok {
				t.Errorf("schema missing %q property", field)
			}
		}
	})

	t.Run("no_mailbox_returns_team_error", func(t *testing.T) {
		// Source: SendMessageTool.ts — not in team context
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"to": "alice", "message": "hello"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error when no mailbox configured")
		}
		if !strings.Contains(out.Content, "not in a team context") {
			t.Errorf("expected team context error, got %q", out.Content)
		}
	})

	t.Run("direct_message_to_known_agent", func(t *testing.T) {
		// Source: SendMessageTool.ts:140-190
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		toolWithMb := &tools.SendMessageTool{
			Mailbox:    mb,
			TeamName:   "test-team",
			SenderName: "bob",
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "alice", "message": "hello alice!", "summary": "greeting"}`)
		out, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Source: SendMessageTool.ts:179 — "Message sent to {name}'s inbox"
		if !strings.Contains(out.Content, "alice's inbox") {
			t.Errorf("expected 'alice's inbox' in result, got %q", out.Content)
		}

		// Verify message was written to mailbox
		messages, _ := mb.ReadMailbox("alice", "test-team")
		if len(messages) != 1 {
			t.Fatalf("expected 1 message in alice's inbox, got %d", len(messages))
		}
		if messages[0].From != "bob" {
			t.Errorf("from = %q, want 'bob'", messages[0].From)
		}
	})

	// Source: SendMessageTool.ts:605-609 — empty to validation
	t.Run("empty_to_returns_error", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"to": "", "message": "hello"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty to")
		}
		if !strings.Contains(out.Content, "to must not be empty") {
			t.Errorf("expected 'to must not be empty', got %q", out.Content)
		}
	})

	// Source: SendMessageTool.ts:605 — whitespace-only to
	t.Run("whitespace_to_returns_error", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"to": "   ", "message": "hello"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for whitespace-only to")
		}
		if !strings.Contains(out.Content, "to must not be empty") {
			t.Errorf("expected 'to must not be empty', got %q", out.Content)
		}
	})

	// Source: SendMessageTool.ts:623-628 — @ in to field
	t.Run("at_sign_in_to_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		toolWithMb := &tools.SendMessageTool{
			Mailbox:    mb,
			TeamName:   "test-team",
			SenderName: "bob",
		}
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "alice@team1", "message": "hello"}`)
		out, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for @ in to field")
		}
		if !strings.Contains(out.Content, "bare teammate name") {
			t.Errorf("expected bare-teammate-name error, got %q", out.Content)
		}
	})

	// Source: SendMessageTool.ts — message required
	t.Run("empty_message_returns_error", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"to": "alice", "message": ""}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty message")
		}
		if !strings.Contains(out.Content, "message is required") {
			t.Errorf("expected 'message is required', got %q", out.Content)
		}
	})

	// Source: SendMessageTool.ts — sender defaults to "agent" when no name set
	t.Run("default_sender_name_is_agent", func(t *testing.T) {
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		toolWithMb := &tools.SendMessageTool{
			Mailbox:  mb,
			TeamName: "test-team",
			// SenderName deliberately unset
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "alice", "message": "hi", "summary": "test"}`)
		out, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		messages, _ := mb.ReadMailbox("alice", "test-team")
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
		if messages[0].From != "agent" {
			t.Errorf("from = %q, want 'agent' (default)", messages[0].From)
		}
	})

	// Source: SendMessageTool.ts — sender color propagation
	t.Run("sender_color_propagated", func(t *testing.T) {
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		toolWithMb := &tools.SendMessageTool{
			Mailbox:     mb,
			TeamName:    "test-team",
			SenderName:  "bob",
			SenderColor: "#ff0000",
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "alice", "message": "hi", "summary": "test"}`)
		_, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		messages, _ := mb.ReadMailbox("alice", "test-team")
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
		if messages[0].Color != "#ff0000" {
			t.Errorf("color = %q, want '#ff0000'", messages[0].Color)
		}
	})

	// Source: SendMessageTool.ts:195-264 — broadcast fan-out
	t.Run("broadcast_sends_to_all_except_sender", func(t *testing.T) {
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		setupTeamDir(t, dir, "alpha", []session.TeamMember{
			{Name: "team-lead", AgentID: "lead-001"},
			{Name: "researcher", AgentID: "agent-002"},
			{Name: "tester", AgentID: "agent-003"},
		})
		toolWithMb := &tools.SendMessageTool{
			Mailbox:    mb,
			TeamName:   "alpha",
			SenderName: "team-lead",
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "*", "message": "attention everyone", "summary": "broadcast"}`)
		out, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Source: SendMessageTool.ts:253
		if !strings.Contains(out.Content, "2 teammate(s)") {
			t.Errorf("expected '2 teammate(s)' in result, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "researcher") || !strings.Contains(out.Content, "tester") {
			t.Errorf("expected recipient names in result, got %q", out.Content)
		}

		// Verify messages landed in inboxes
		for _, name := range []string{"researcher", "tester"} {
			msgs, _ := mb.ReadMailbox(name, "alpha")
			if len(msgs) != 1 {
				t.Errorf("%s inbox: expected 1 message, got %d", name, len(msgs))
			}
		}
		// Sender should NOT have a message
		senderMsgs, _ := mb.ReadMailbox("team-lead", "alpha")
		if len(senderMsgs) != 0 {
			t.Errorf("sender should not receive broadcast, got %d messages", len(senderMsgs))
		}
	})

	// Source: SendMessageTool.ts:228-231 — sole member broadcast
	t.Run("broadcast_sole_member_returns_no_recipients", func(t *testing.T) {
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		setupTeamDir(t, dir, "solo", []session.TeamMember{
			{Name: "team-lead", AgentID: "lead-001"},
		})
		toolWithMb := &tools.SendMessageTool{
			Mailbox:    mb,
			TeamName:   "solo",
			SenderName: "team-lead",
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "*", "message": "hello?", "summary": "lonely"}`)
		out, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "only team member") {
			t.Errorf("expected sole-member message, got %q", out.Content)
		}
	})

	// Source: SendMessageTool.ts:199-203 — broadcast without team name
	t.Run("broadcast_no_team_name_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		toolWithMb := &tools.SendMessageTool{
			Mailbox:    mb,
			TeamName:   "", // no team
			SenderName: "bob",
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "*", "message": "hello"}`)
		out, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for broadcast without team name")
		}
		if !strings.Contains(out.Content, "Not in a team context") {
			t.Errorf("expected team context error, got %q", out.Content)
		}
	})

	// Source: SendMessageTool.ts:205-206 — broadcast with nonexistent team
	t.Run("broadcast_nonexistent_team_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		mb := session.NewMailbox(dir)
		toolWithMb := &tools.SendMessageTool{
			Mailbox:    mb,
			TeamName:   "nonexistent",
			SenderName: "bob",
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"to": "*", "message": "hello"}`)
		out, err := toolWithMb.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent team")
		}
		if !strings.Contains(out.Content, "does not exist") {
			t.Errorf("expected 'does not exist' error, got %q", out.Content)
		}
	})

	// Source: SendMessageTool.ts:524 — searchHint
	t.Run("search_hint", func(t *testing.T) {
		hint := tool.SearchHint()
		if !strings.Contains(hint, "swarm") {
			t.Errorf("SearchHint() = %q, want mention of swarm", hint)
		}
	})

	// Source: SendMessageTool.ts:524 — maxResultSizeChars
	t.Run("max_result_size_chars", func(t *testing.T) {
		if tool.MaxResultSizeChars() != 100_000 {
			t.Errorf("MaxResultSizeChars() = %d, want 100000", tool.MaxResultSizeChars())
		}
	})
}

// TestSendMessageToolPrompt verifies the prompt text contains key guidance
// verbatim from tools/SendMessageTool/prompt.ts.
func TestSendMessageToolPrompt(t *testing.T) {
	tool := &tools.SendMessageTool{}
	prompt := tool.Prompt()

	if prompt == "" {
		t.Fatal("Prompt() should not be empty")
	}

	// Source: prompt.ts — verbatim strings that must appear
	requiredStrings := []string{
		"# SendMessage",
		"Send a message to another agent.",
		// Example JSON
		`"to": "researcher"`,
		`"summary": "assign task 1"`,
		`"message": "start on task #1"`,
		// To table entries
		"Teammate by name",
		"Broadcast to all teammates",
		"expensive (linear in team size)",
		// Key communication contract
		"Your plain text output is NOT visible to other agents",
		"you MUST call this tool",
		"Messages from teammates are delivered automatically",
		"you don't check an inbox",
		"Refer to teammates by name, never by UUID",
		"don't quote the original",
		// Protocol responses section
		"## Protocol responses (legacy)",
		"shutdown_request",
		"plan_approval_request",
		"_response",
		"request_id",
		"approve",
		// Protocol response examples
		`"type": "shutdown_response"`,
		`"type": "plan_approval_response"`,
		`"feedback": "add error handling"`,
		// Behavioral guidance
		"Approving shutdown terminates your process",
		"Rejecting plan sends the teammate back to revise",
		"Don't originate `shutdown_request` unless asked",
		// TaskUpdate reference
		"use TaskUpdate",
	}

	for _, want := range requiredStrings {
		if !strings.Contains(prompt, want) {
			t.Errorf("Prompt() missing required string: %q", want)
		}
	}
}
