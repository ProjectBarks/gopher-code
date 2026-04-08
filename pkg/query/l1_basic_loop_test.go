package query_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestL1BasicLoop(t *testing.T) {

	// 1. single_turn_text_only
	t.Run("single_turn_text_only", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("Hello back!", provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// user("hello") + assistant("Hello back!")
		if len(sess.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(sess.Messages))
		}
		if sess.Messages[0].Role != message.RoleUser {
			t.Errorf("expected message[0] role User, got %s", sess.Messages[0].Role)
		}
		if sess.Messages[1].Role != message.RoleAssistant {
			t.Errorf("expected message[1] role Assistant, got %s", sess.Messages[1].Role)
		}
		if sess.TurnCount != 1 {
			t.Errorf("expected turn_count=1, got %d", sess.TurnCount)
		}

		if len(sess.Messages[1].Content) == 0 {
			t.Fatal("assistant message has no content blocks")
		}
		block := sess.Messages[1].Content[0]
		if block.Type != message.ContentText {
			t.Fatalf("expected Text block, got %s", block.Type)
		}
		if block.Text != "Hello back!" {
			t.Errorf("expected text 'Hello back!', got %q", block.Text)
		}
	})

	// 2. multi_turn_with_tools
	t.Run("multi_turn_with_tools", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{"x":1}`), provider.StopReasonToolUse),
			testharness.MakeToolTurn("t2", "my_tool", json.RawMessage(`{"x":2}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("all done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if sess.TurnCount != 3 {
			t.Errorf("expected turn_count=3, got %d", sess.TurnCount)
		}

		// Messages: user, assistant(tool_use), user(tool_result),
		//           assistant(tool_use), user(tool_result),
		//           assistant(text)
		if len(sess.Messages) != 6 {
			t.Fatalf("expected 6 messages, got %d", len(sess.Messages))
		}
		expectedRoles := []message.Role{
			message.RoleUser, message.RoleAssistant, message.RoleUser,
			message.RoleAssistant, message.RoleUser, message.RoleAssistant,
		}
		for i, role := range expectedRoles {
			if sess.Messages[i].Role != role {
				t.Errorf("message[%d]: expected role %s, got %s", i, role, sess.Messages[i].Role)
			}
		}
	})

	// 3. streaming_text_deltas
	t.Run("streaming_text_deltas", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeChunkedTextTurn([]string{"He", "llo", " ", "wo", "rld"}, provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		deltas := eventLog.TextDeltas()
		if len(deltas) != 5 {
			t.Fatalf("expected 5 text deltas, got %d", len(deltas))
		}
		assembled := strings.Join(deltas, "")
		if assembled != "Hello world" {
			t.Errorf("expected assembled text 'Hello world', got %q", assembled)
		}
	})

	// 4. streaming_tool_json_assembly
	t.Run("streaming_tool_json_assembly", func(t *testing.T) {
		fullInput := json.RawMessage(`{"command":"ls -la","cwd":"/tmp"}`)
		serialized := string(fullInput)
		mid1 := len(serialized) / 3
		mid2 := mid1 * 2
		chunks := []string{serialized[:mid1], serialized[mid1:mid2], serialized[mid2:]}

		spy := testharness.NewSpyTool("bash", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurnWithJSONChunks("t1", "bash", chunks, fullInput, provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// The assistant message at index 1 should contain a ToolUse block
		if len(sess.Messages) < 2 {
			t.Fatal("expected at least 2 messages")
		}
		assistantMsg := sess.Messages[1]
		var foundInput json.RawMessage
		for _, block := range assistantMsg.Content {
			if block.Type == message.ContentToolUse {
				foundInput = block.Input
				break
			}
		}
		if foundInput == nil {
			t.Fatal("assistant message should contain a ToolUse block")
		}

		// Compare as parsed JSON to avoid formatting differences
		var expected, actual interface{}
		if err := json.Unmarshal(fullInput, &expected); err != nil {
			t.Fatalf("failed to unmarshal expected: %v", err)
		}
		if err := json.Unmarshal(foundInput, &actual); err != nil {
			t.Fatalf("failed to unmarshal actual: %v", err)
		}
		expectedBytes, _ := json.Marshal(expected)
		actualBytes, _ := json.Marshal(actual)
		if string(expectedBytes) != string(actualBytes) {
			t.Errorf("expected input %s, got %s", string(expectedBytes), string(actualBytes))
		}
	})

	// 5. max_turns_exceeded
	t.Run("max_turns_exceeded", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeToolTurn("t2", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:          "test-model",
			MaxTurns:       2,
			PermissionMode: permissions.AutoApprove,
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var agentErr *query.AgentError
		if !errors.As(err, &agentErr) {
			t.Fatalf("expected AgentError, got %T: %v", err, err)
		}
		if agentErr.Kind != query.ErrMaxTurnsExceeded {
			t.Errorf("expected ErrMaxTurnsExceeded, got %d", agentErr.Kind)
		}
	})

	// 6. empty_tool_calls_terminates
	t.Run("empty_tool_calls_terminates", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("final answer", provider.StopReasonEndTurn),
		)
		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		events := eventLog.Events()
		turnCompleteFound := false
		for _, e := range events {
			if e.Type == query.QEventTurnComplete && e.StopReason == provider.StopReasonEndTurn {
				turnCompleteFound = true
				break
			}
		}
		if !turnCompleteFound {
			t.Error("should emit TurnComplete with EndTurn stop reason")
		}
	})

	// 7. multiple_concurrent_tool_calls
	t.Run("multiple_concurrent_tool_calls", func(t *testing.T) {
		readTool := testharness.NewSpyTool("read_tool", true)
		writeTool := testharness.NewSpyTool("write_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeMultiToolTurn([]testharness.ToolSpec{
				{ID: "t1", Name: "read_tool", Input: json.RawMessage(`{"path":"/etc/hosts"}`)},
				{ID: "t2", Name: "write_tool", Input: json.RawMessage(`{"path":"/tmp/out"}`)},
			}, provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(readTool)
		registry.Register(writeTool)
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		toolResults := eventLog.ToolResults()
		if len(toolResults) != 2 {
			t.Errorf("expected 2 ToolResult events, got %d", len(toolResults))
		}

		// The user message after tools: user(idx=0), assistant(idx=1, 2 tools), user(idx=2, 2 results)
		if len(sess.Messages) < 3 {
			t.Fatal("expected at least 3 messages")
		}
		toolResultMsg := sess.Messages[2]
		if toolResultMsg.Role != message.RoleUser {
			t.Errorf("expected message[2] role User, got %s", toolResultMsg.Role)
		}
		if len(toolResultMsg.Content) != 2 {
			t.Fatalf("expected 2 content blocks in tool result message, got %d", len(toolResultMsg.Content))
		}
		for i, block := range toolResultMsg.Content {
			if block.Type != message.ContentToolResult {
				t.Errorf("block[%d]: expected ToolResult type, got %s", i, block.Type)
			}
		}
	})

	// 8. usage_tokens_accumulated
	t.Run("usage_tokens_accumulated", func(t *testing.T) {
		spy := testharness.NewSpyTool("my_tool", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurnWithUsage("t1", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse,
				provider.Usage{InputTokens: 100, OutputTokens: 50}),
			testharness.MakeToolTurnWithUsage("t2", "my_tool", json.RawMessage(`{}`), provider.StopReasonToolUse,
				provider.Usage{InputTokens: 200, OutputTokens: 80}),
			testharness.MakeTextTurnWithUsage("done", provider.StopReasonEndTurn,
				provider.Usage{InputTokens: 300, OutputTokens: 120}),
		)

		registry := tools.NewRegistry()
		registry.Register(spy)
		orchestrator := tools.NewOrchestrator(registry)
		callback, eventLog := testharness.NewEventCollector()

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if sess.TotalInputTokens != 600 {
			t.Errorf("expected TotalInputTokens=600, got %d", sess.TotalInputTokens)
		}
		if sess.TotalOutputTokens != 250 {
			t.Errorf("expected TotalOutputTokens=250, got %d", sess.TotalOutputTokens)
		}
		if sess.TurnCount != 3 {
			t.Errorf("expected turn_count=3, got %d", sess.TurnCount)
		}

		usageEvents := eventLog.UsageEvents()
		if len(usageEvents) != 3 {
			t.Fatalf("expected 3 usage events, got %d", len(usageEvents))
		}
		expectedUsage := []struct{ input, output int }{
			{100, 50}, {200, 80}, {300, 120},
		}
		for i, eu := range expectedUsage {
			if usageEvents[i].InputTokens != eu.input {
				t.Errorf("usage[%d]: expected InputTokens=%d, got %d", i, eu.input, usageEvents[i].InputTokens)
			}
			if usageEvents[i].OutputTokens != eu.output {
				t.Errorf("usage[%d]: expected OutputTokens=%d, got %d", i, eu.output, usageEvents[i].OutputTokens)
			}
		}
	})

	// 9. system_prompt_forwarded
	t.Run("system_prompt_forwarded", func(t *testing.T) {
		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSessionWithConfig(session.SessionConfig{
			Model:          "test-model",
			SystemPrompt:   "You are a helpful assistant.",
			MaxTurns:       100,
			PermissionMode: permissions.AutoApprove,
		})

		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		requests := prov.CapturedRequests
		if len(requests) < 1 {
			t.Fatal("expected at least 1 captured request")
		}
		if requests[0].System != "You are a helpful assistant." {
			t.Errorf("expected system prompt 'You are a helpful assistant.', got %q", requests[0].System)
		}
	})

	// 10. tool_definitions_included
	t.Run("tool_definitions_included", func(t *testing.T) {
		toolA := testharness.NewSpyTool("tool_a", true)
		toolB := testharness.NewSpyTool("tool_b", false)

		prov := testharness.NewScriptedProvider(
			testharness.MakeTextTurn("ok", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(toolA)
		registry.Register(toolB)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		requests := prov.CapturedRequests
		if len(requests) < 1 {
			t.Fatal("expected at least 1 captured request")
		}
		toolDefs := requests[0].Tools
		if len(toolDefs) != 2 {
			t.Fatalf("expected 2 tool definitions, got %d", len(toolDefs))
		}
		names := make(map[string]bool)
		for _, td := range toolDefs {
			names[td.Name] = true
		}
		if !names["tool_a"] {
			t.Error("expected tool_a in tool definitions")
		}
		if !names["tool_b"] {
			t.Error("expected tool_b in tool definitions")
		}
	})

	// 11. empty_tool_result_gets_no_content_message
	// Source: utils/messages.ts:506 — content: content || NO_CONTENT_MESSAGE
	t.Run("empty_tool_result_gets_no_content_message", func(t *testing.T) {
		// A tool that returns empty content should have its result replaced
		// with the NoContentMessage constant to avoid sending empty strings
		// to the API.
		emptyTool := testharness.NewSpyTool("empty_tool", false).
			WithResponse(func(_ json.RawMessage) *tools.ToolOutput {
				return tools.SuccessOutput("") // empty content
			})

		prov := testharness.NewScriptedProvider(
			testharness.MakeToolTurn("t1", "empty_tool", json.RawMessage(`{}`), provider.StopReasonToolUse),
			testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
		)

		registry := tools.NewRegistry()
		registry.Register(emptyTool)
		orchestrator := tools.NewOrchestrator(registry)

		sess := testharness.MakeSession()
		err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Messages: user, assistant(tool_use), user(tool_result), assistant(text)
		if len(sess.Messages) < 3 {
			t.Fatalf("expected at least 3 messages, got %d", len(sess.Messages))
		}

		toolResultMsg := sess.Messages[2]
		if toolResultMsg.Role != message.RoleUser {
			t.Fatalf("expected message[2] role User, got %s", toolResultMsg.Role)
		}

		found := false
		for _, block := range toolResultMsg.Content {
			if block.Type == message.ContentToolResult {
				if block.Content != message.NoContentMessage {
					t.Errorf("expected tool result content %q, got %q", message.NoContentMessage, block.Content)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("expected a tool_result block in message[2]")
		}
	})
}
