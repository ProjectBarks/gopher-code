package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// failingTool is a tool that always returns an error from Execute.
type failingTool struct{}

func (f failingTool) Name() string                { return "failing_tool" }
func (f failingTool) Description() string          { return "always fails" }
func (f failingTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (f failingTool) IsReadOnly() bool             { return false }
func (f failingTool) Execute(_ context.Context, _ *tools.ToolContext, _ json.RawMessage) (*tools.ToolOutput, error) {
	return nil, fmt.Errorf("something went wrong")
}

func makeToolContext(mode permissions.PermissionMode) *tools.ToolContext {
	return &tools.ToolContext{
		CWD:         "/tmp",
		Permissions: permissions.NewRuleBasedPolicy(mode),
		SessionID:   "test-session",
	}
}

func TestOrchestrator(t *testing.T) {

	// 1. unknown_tool_returns_error
	t.Run("unknown_tool_returns_error", func(t *testing.T) {
		registry := tools.NewRegistry()
		orch := tools.NewOrchestrator(registry)
		tc := makeToolContext(permissions.AutoApprove)

		calls := []tools.ToolCall{
			{ID: "c1", Name: "nonexistent", Input: json.RawMessage(`{}`)},
		}
		results := orch.ExecuteBatch(context.Background(), calls, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if !results[0].Output.IsError {
			t.Error("expected output to be an error")
		}
		if !strings.Contains(results[0].Output.Content, "unknown tool") {
			t.Errorf("expected error content to contain 'unknown tool', got: %q", results[0].Output.Content)
		}
	})

	// 2. read_only_tool_skips_permission_check
	t.Run("read_only_tool_skips_permission_check", func(t *testing.T) {
		spy := testharness.NewSpyTool("read_tool", true)
		registry := tools.NewRegistry()
		registry.Register(spy)
		orch := tools.NewOrchestrator(registry)
		tc := makeToolContext(permissions.Deny)

		calls := []tools.ToolCall{
			{ID: "c1", Name: "read_tool", Input: json.RawMessage(`{}`)},
		}
		results := orch.ExecuteBatch(context.Background(), calls, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Output.IsError {
			t.Errorf("expected read-only tool to succeed even in Deny mode, got error: %q", results[0].Output.Content)
		}
	})

	// 3. mutating_tool_denied_in_deny_mode
	t.Run("mutating_tool_denied_in_deny_mode", func(t *testing.T) {
		spy := testharness.NewSpyTool("write_tool", false)
		registry := tools.NewRegistry()
		registry.Register(spy)
		orch := tools.NewOrchestrator(registry)
		tc := makeToolContext(permissions.Deny)

		calls := []tools.ToolCall{
			{ID: "c1", Name: "write_tool", Input: json.RawMessage(`{}`)},
		}
		results := orch.ExecuteBatch(context.Background(), calls, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if !results[0].Output.IsError {
			t.Error("expected mutating tool to be denied in Deny mode")
		}
		if !strings.Contains(strings.ToLower(results[0].Output.Content), "permission denied") {
			t.Errorf("expected error content to contain 'permission denied', got: %q", results[0].Output.Content)
		}
	})

	// 4. mutating_tool_allowed_in_auto_approve
	t.Run("mutating_tool_allowed_in_auto_approve", func(t *testing.T) {
		spy := testharness.NewSpyTool("write_tool", false)
		registry := tools.NewRegistry()
		registry.Register(spy)
		orch := tools.NewOrchestrator(registry)
		tc := makeToolContext(permissions.AutoApprove)

		calls := []tools.ToolCall{
			{ID: "c1", Name: "write_tool", Input: json.RawMessage(`{}`)},
		}
		results := orch.ExecuteBatch(context.Background(), calls, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Output.IsError {
			t.Errorf("expected mutating tool to succeed in AutoApprove mode, got error: %q", results[0].Output.Content)
		}
	})

	// 5. failing_tool_wraps_error
	t.Run("failing_tool_wraps_error", func(t *testing.T) {
		registry := tools.NewRegistry()
		registry.Register(failingTool{})
		orch := tools.NewOrchestrator(registry)
		tc := makeToolContext(permissions.AutoApprove)

		calls := []tools.ToolCall{
			{ID: "c1", Name: "failing_tool", Input: json.RawMessage(`{}`)},
		}
		results := orch.ExecuteBatch(context.Background(), calls, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if !results[0].Output.IsError {
			t.Error("expected failing tool to produce an error output")
		}
		if !strings.Contains(results[0].Output.Content, "tool execution failed") {
			t.Errorf("expected error content to contain 'tool execution failed', got: %q", results[0].Output.Content)
		}
	})

	// 6. execute_batch_runs_all_tools
	t.Run("execute_batch_runs_all_tools", func(t *testing.T) {
		readTool := testharness.NewSpyTool("read_tool", true)
		writeTool := testharness.NewSpyTool("write_tool", false)
		registry := tools.NewRegistry()
		registry.Register(readTool)
		registry.Register(writeTool)
		orch := tools.NewOrchestrator(registry)
		tc := makeToolContext(permissions.AutoApprove)

		calls := []tools.ToolCall{
			{ID: "c1", Name: "read_tool", Input: json.RawMessage(`{}`)},
			{ID: "c2", Name: "write_tool", Input: json.RawMessage(`{}`)},
		}
		results := orch.ExecuteBatch(context.Background(), calls, tc)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		for i, r := range results {
			if r.Output.IsError {
				t.Errorf("result[%d]: expected no error, got: %q", i, r.Output.Content)
			}
		}
	})
}
