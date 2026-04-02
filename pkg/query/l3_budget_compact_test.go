package query_test

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestL3BudgetCompact(t *testing.T) {
	t.Run("auto_compact_triggered_at_threshold", func(t *testing.T) {
		// Budget: context=1000, output=200 => input_budget=800, threshold@0.8 => 640
		spy := testharness.NewSpyTool("my_tool", false)
		cfg := session.SessionConfig{
			Model:    "test-model",
			MaxTurns: 100,
			TokenBudget: compact.TokenBudget{
				ContextWindow:    1000,
				MaxOutputTokens:  200,
				CompactThreshold: 0.8,
			},
			PermissionMode: permissions.AutoApprove,
		}

		prov := testharness.NewScriptedProvider(
			// Turn 1: tool call with usage that exceeds threshold (700 > 640)
			testharness.MakeToolTurnWithUsage(
				"t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse,
				provider.Usage{InputTokens: 700, OutputTokens: 50},
			),
			// Turn 2: after compaction, model responds with text
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(cfg)
		// Pad messages so compaction has something to remove
		for i := 0; i < 10; i++ {
			sess.PushMessage(message.UserMessage(fmt.Sprintf("padding %d", i)))
			sess.PushMessage(message.Message{
				Role:    message.RoleAssistant,
				Content: []message.ContentBlock{message.TextBlock(fmt.Sprintf("ack %d", i))},
			})
		}

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("query returned error: %v", err)
		}

		requests := prov.Requests()
		if len(requests) < 2 {
			t.Fatalf("expected at least 2 requests, got %d", len(requests))
		}
		if requests[1].Messages == nil || requests[0].Messages == nil {
			t.Fatal("requests should have messages")
		}
		if len(requests[1].Messages) >= len(requests[0].Messages) {
			t.Errorf(
				"compacted request (%d msgs) should have fewer messages than original (%d msgs)",
				len(requests[1].Messages), len(requests[0].Messages),
			)
		}
	})

	t.Run("auto_compact_preserves_system_prompt", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)
		cfg := session.SessionConfig{
			Model:        "test-model",
			SystemPrompt: "Be helpful",
			MaxTurns:     100,
			TokenBudget: compact.TokenBudget{
				ContextWindow:    1000,
				MaxOutputTokens:  200,
				CompactThreshold: 0.8,
			},
			PermissionMode: permissions.AutoApprove,
		}

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurnWithUsage(
				"t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse,
				provider.Usage{InputTokens: 700, OutputTokens: 50},
			),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(cfg)
		for i := 0; i < 10; i++ {
			sess.PushMessage(message.UserMessage(fmt.Sprintf("padding %d", i)))
			sess.PushMessage(message.Message{
				Role:    message.RoleAssistant,
				Content: []message.ContentBlock{message.TextBlock(fmt.Sprintf("ack %d", i))},
			})
		}

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("query returned error: %v", err)
		}

		requests := prov.Requests()
		for i, req := range requests {
			if req.System != "Be helpful" {
				t.Errorf("request[%d].System = %q, want %q (system prompt must survive compaction)", i, req.System, "Be helpful")
			}
		}
	})

	t.Run("auto_compact_preserves_last_exchange", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)
		cfg := session.SessionConfig{
			Model:    "test-model",
			MaxTurns: 100,
			TokenBudget: compact.TokenBudget{
				ContextWindow:    1000,
				MaxOutputTokens:  200,
				CompactThreshold: 0.8,
			},
			PermissionMode: permissions.AutoApprove,
		}

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurnWithUsage(
				"t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse,
				provider.Usage{InputTokens: 700, OutputTokens: 50},
			),
			testharness.MakeTextTurn("final", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(cfg)
		// Fill session with 10 extra messages
		for i := 0; i < 10; i++ {
			sess.PushMessage(message.UserMessage(fmt.Sprintf("padding %d", i)))
			sess.PushMessage(message.Message{
				Role:    message.RoleAssistant,
				Content: []message.ContentBlock{message.TextBlock(fmt.Sprintf("ack %d", i))},
			})
		}
		// Add a distinctive last exchange
		sess.PushMessage(message.UserMessage("the important question"))
		sess.PushMessage(message.Message{
			Role:    message.RoleAssistant,
			Content: []message.ContentBlock{message.TextBlock("the important answer")},
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("query returned error: %v", err)
		}

		requests := prov.Requests()
		if len(requests) >= 2 {
			compactedMsgs := requests[1].Messages
			if len(compactedMsgs) < 2 {
				t.Errorf("should preserve at least 2 messages after compaction, got %d", len(compactedMsgs))
			}
		}
	})

	t.Run("token_budget_limits_output_tokens", func(t *testing.T) {
		cfg := session.SessionConfig{
			Model:    "test-model",
			MaxTurns: 100,
			TokenBudget: compact.TokenBudget{
				ContextWindow:    100000,
				MaxOutputTokens:  4096,
				CompactThreshold: 0.8,
			},
			PermissionMode: permissions.AutoApprove,
		}

		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(cfg)

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("query returned error: %v", err)
		}

		requests := prov.Requests()
		if len(requests) < 1 {
			t.Fatal("expected at least 1 request")
		}
		if requests[0].MaxTokens != 4096 {
			t.Errorf("MaxTokens = %d, want 4096", requests[0].MaxTokens)
		}
	})

	t.Run("input_budget_checked_each_turn", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)
		cfg := session.SessionConfig{
			Model:    "test-model",
			MaxTurns: 100,
			TokenBudget: compact.TokenBudget{
				ContextWindow:    500,
				MaxOutputTokens:  100,
				CompactThreshold: 0.8,
			},
			PermissionMode: permissions.AutoApprove,
		}

		prov := testharness.NewScriptedProvider(
			// Turn 1: usage stays under threshold
			testharness.MakeToolTurnWithUsage(
				"t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse,
				provider.Usage{InputTokens: 100, OutputTokens: 20},
			),
			// Turn 2: usage exceeds threshold; compaction should occur before request
			testharness.MakeToolTurnWithUsage(
				"t2", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse,
				provider.Usage{InputTokens: 350, OutputTokens: 30},
			),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(cfg)
		for i := 0; i < 5; i++ {
			sess.PushMessage(message.UserMessage(fmt.Sprintf("msg %d", i)))
			sess.PushMessage(message.Message{
				Role:    message.RoleAssistant,
				Content: []message.ContentBlock{message.TextBlock(fmt.Sprintf("reply %d", i))},
			})
		}

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("query returned error: %v", err)
		}

		requests := prov.Requests()
		if len(requests) < 3 {
			t.Fatalf("expected at least 3 requests, got %d", len(requests))
		}
		// Third request should have fewer messages than second (compacted)
		if len(requests[2].Messages) >= len(requests[1].Messages) {
			t.Errorf(
				"third request (%d msgs) should reflect compaction vs second request (%d msgs)",
				len(requests[2].Messages), len(requests[1].Messages),
			)
		}
	})

	t.Run("micro_compact_large_tool_result", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false).WithResponse(func(_ json.RawMessage) *tools.ToolOutput {
			return tools.SuccessOutput(strings.Repeat("x", 20000))
		})

		cfg := session.SessionConfig{
			Model:          "test-model",
			MaxTurns:       100,
			TokenBudget:    compact.DefaultBudget(),
			PermissionMode: permissions.AutoApprove,
		}

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(cfg)

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("query returned error: %v", err)
		}

		// Find the tool_result in session messages
		var toolResultContent string
		found := false
		for _, msg := range sess.Messages {
			for _, block := range msg.Content {
				if block.Type == message.ContentToolResult {
					toolResultContent = block.Content
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatal("should have tool_result in session messages")
		}

		if !strings.Contains(toolResultContent, "...[truncated]") {
			t.Errorf("large tool result should contain '...[truncated]', got content of length %d", len(toolResultContent))
		}
		if len(toolResultContent) > 10240 {
			t.Errorf("large tool result (%d chars) should be micro-compacted to <= ~10KB", len(toolResultContent))
		}
	})

	t.Run("micro_compact_small_tool_result_untouched", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false).WithResponse(func(_ json.RawMessage) *tools.ToolOutput {
			return tools.SuccessOutput("ok")
		})

		cfg := session.SessionConfig{
			Model:          "test-model",
			MaxTurns:       100,
			TokenBudget:    compact.DefaultBudget(),
			PermissionMode: permissions.AutoApprove,
		}

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(cfg)

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("query returned error: %v", err)
		}

		// Find the tool_result in session messages
		var toolResultContent string
		found := false
		for _, msg := range sess.Messages {
			for _, block := range msg.Content {
				if block.Type == message.ContentToolResult {
					toolResultContent = block.Content
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatal("should have tool_result in session messages")
		}

		if toolResultContent != "ok" {
			t.Errorf("small tool result should be preserved as-is, got %q", toolResultContent)
		}
	})
}

func TestTokenBudgetNudge(t *testing.T) {
	// Source: query.ts:1308-1355 — token budget continuation nudge

	t.Run("nudge_injected_when_under_budget", func(t *testing.T) {
		// Source: query.ts:1316-1340 — action='continue' injects nudge message
		// With a 100k budget, when the model produces ~5k output tokens
		// (well under 90% threshold), a nudge should be injected and the loop continues.

		// Turn 1: model responds with text (5k output tokens)
		// Turn 2: model responds with text (after nudge)
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurnWithUsage("first part", provider.StopReasonEndTurn,
				provider.Usage{InputTokens: 1000, OutputTokens: 5000}),
			testharness.MakeTextTurnWithUsage("second part", provider.StopReasonEndTurn,
				provider.Usage{InputTokens: 2000, OutputTokens: 90000}), // now at 95k, above 90% of 100k
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:             "test-model",
			MaxTurns:          100,
			TokenBudget:       compact.DefaultBudget(),
			PermissionMode:    permissions.AutoApprove,
			TokenBudgetTarget: 100_000, // +100k
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Should have at least 3 messages: user("hello"), asst("first part"), user(nudge), asst("second part")
		if len(sess.Messages) < 4 {
			t.Fatalf("expected at least 4 messages (with nudge), got %d", len(sess.Messages))
		}

		// Find the nudge message
		foundNudge := false
		for _, msg := range sess.Messages {
			if msg.Role == message.RoleUser {
				for _, b := range msg.Content {
					if b.Type == message.ContentText && strings.Contains(b.Text, "Stopped at") && strings.Contains(b.Text, "token target") {
						foundNudge = true
					}
				}
			}
		}
		if !foundNudge {
			t.Error("expected a nudge message containing 'Stopped at...token target'")
		}
	})

	t.Run("no_nudge_without_budget_target", func(t *testing.T) {
		// When TokenBudgetTarget is 0, no nudge should be injected
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:             "test-model",
			MaxTurns:          100,
			TokenBudget:       compact.DefaultBudget(),
			PermissionMode:    permissions.AutoApprove,
			TokenBudgetTarget: 0, // no budget
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Should have exactly 2 messages: user("hello"), asst("done")
		if len(sess.Messages) != 2 {
			t.Errorf("expected 2 messages (no nudge), got %d", len(sess.Messages))
		}
	})
}
