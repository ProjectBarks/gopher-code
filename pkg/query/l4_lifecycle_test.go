package query_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

func intPtr(n int) *int { return &n }

func TestL4Lifecycle(t *testing.T) {

	// 1. memory_prefetch_loads_claude_md
	t.Run("memory_prefetch_loads_claude_md", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gopher-test-claude-md-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
		if err := os.WriteFile(claudeMDPath, []byte("SECRET_CONTEXT_42"), 0644); err != nil {
			t.Fatalf("failed to write CLAUDE.md: %v", err)
		}

		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithCWD(tmpDir)
		err = query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(prov.CapturedRequests) < 1 {
			t.Fatal("expected at least 1 captured request")
		}
		system := prov.CapturedRequests[0].System
		if !strings.Contains(system, "SECRET_CONTEXT_42") {
			t.Errorf("system prompt should contain CLAUDE.md content, got: %q", system)
		}
	})

	// 2. memory_prefetch_missing_file
	t.Run("memory_prefetch_missing_file", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gopher-test-no-claude-md-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:          "test-model",
			SystemPrompt:   "base prompt",
			MaxTurns:       100,
			TokenBudget:    compact.DefaultBudget(),
			PermissionMode: permissions.AutoApprove,
		})
		// Override CWD to the empty temp dir (no CLAUDE.md)
		sess.CWD = tmpDir

		err = query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(prov.CapturedRequests) < 1 {
			t.Fatal("expected at least 1 captured request")
		}
		system := prov.CapturedRequests[0].System
		if system != "base prompt" {
			t.Errorf("system prompt should be unchanged when CLAUDE.md is missing, got: %q", system)
		}
	})

	// 3. usage_includes_cache_tokens
	t.Run("usage_includes_cache_tokens", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurnWithUsage("response", provider.StopReasonEndTurn, provider.Usage{
				InputTokens:              100,
				OutputTokens:             50,
				CacheCreationInputTokens: intPtr(500),
				CacheReadInputTokens:     intPtr(1200),
			}),
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if sess.TotalCacheCreationTokens != 500 {
			t.Errorf("expected TotalCacheCreationTokens=500, got %d", sess.TotalCacheCreationTokens)
		}
		if sess.TotalCacheReadTokens != 1200 {
			t.Errorf("expected TotalCacheReadTokens=1200, got %d", sess.TotalCacheReadTokens)
		}

		usageEvents := eventLog.UsageEvents()
		if len(usageEvents) != 1 {
			t.Fatalf("expected 1 usage event, got %d", len(usageEvents))
		}
		ue := usageEvents[0]
		if ue.CacheCreation == nil || *ue.CacheCreation != 500 {
			t.Errorf("expected CacheCreation=500, got %v", ue.CacheCreation)
		}
		if ue.CacheRead == nil || *ue.CacheRead != 1200 {
			t.Errorf("expected CacheRead=1200, got %v", ue.CacheRead)
		}
	})

	// 4. permission_denied_surfaces_as_tool_error
	t.Run("permission_denied_surfaces_as_tool_error", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:          "test-model",
			MaxTurns:       100,
			TokenBudget:    compact.DefaultBudget(),
			PermissionMode: permissions.Deny,
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Look for a tool_result message with IsError=true and "permission denied" in content
		found := false
		for _, msg := range sess.Messages {
			for _, block := range msg.Content {
				if block.Type == message.ContentToolResult && block.IsError {
					if strings.Contains(strings.ToLower(block.Content), "permission denied") {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Error("expected a ToolResult message with IsError=true containing 'permission denied'")
		}
	})

	// 5. session_save_load_roundtrip
	t.Run("session_save_load_roundtrip", func(t *testing.T) {
		sess := testharness.MakeSession()
		sess.PushMessage(message.Message{
			Role: message.RoleAssistant,
			Content: []message.ContentBlock{
				message.TextBlock("Hello!"),
			},
		})
		sess.PushMessage(message.Message{
			Role: message.RoleUser,
			Content: []message.ContentBlock{
				message.TextBlock("How are you?"),
			},
		})

		data, err := json.Marshal(sess)
		if err != nil {
			t.Fatalf("failed to marshal session: %v", err)
		}

		var loaded session.SessionState
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("failed to unmarshal session: %v", err)
		}

		if len(loaded.Messages) != len(sess.Messages) {
			t.Fatalf("expected %d messages, got %d", len(sess.Messages), len(loaded.Messages))
		}
		for i, msg := range sess.Messages {
			if loaded.Messages[i].Role != msg.Role {
				t.Errorf("message[%d]: expected role %s, got %s", i, msg.Role, loaded.Messages[i].Role)
			}
			if len(loaded.Messages[i].Content) != len(msg.Content) {
				t.Errorf("message[%d]: expected %d content blocks, got %d",
					i, len(msg.Content), len(loaded.Messages[i].Content))
				continue
			}
			for j, block := range msg.Content {
				loadedBlock := loaded.Messages[i].Content[j]
				if loadedBlock.Type != block.Type {
					t.Errorf("message[%d].content[%d]: expected type %s, got %s",
						i, j, block.Type, loadedBlock.Type)
				}
				if loadedBlock.Text != block.Text {
					t.Errorf("message[%d].content[%d]: expected text %q, got %q",
						i, j, block.Text, loadedBlock.Text)
				}
			}
		}
	})

	// 6. ctrl_c_aborts_gracefully
	t.Run("ctrl_c_aborts_gracefully", func(t *testing.T) {
		t.Skip("interrupt/resume not yet implemented")

		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("partial text before interrupt", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		sess := testharness.MakeSession()
		err := query.Query(ctx, sess, prov, registry, orchestrator, nil)

		if err == nil {
			t.Fatal("expected an error from cancelled context, got nil")
		}

		// Accept either context.Canceled or AgentError
		var agentErr *query.AgentError
		if !errors.Is(err, context.Canceled) && !errors.As(err, &agentErr) {
			t.Fatalf("expected context.Canceled or AgentError, got %T: %v", err, err)
		}
	})
}
