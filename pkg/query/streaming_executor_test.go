package query_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// Source: services/tools/StreamingToolExecutor.ts

func TestStreamingToolExecutor(t *testing.T) {

	t.Run("executes_concurrent_tools_in_parallel", func(t *testing.T) {
		// Source: StreamingToolExecutor.ts:39 — concurrent-safe tools can execute in parallel
		registry := tools.NewRegistry()
		spy1 := testharness.NewSpyTool("tool_a", true) // read-only = concurrent safe
		spy2 := testharness.NewSpyTool("tool_b", true)
		registry.Register(spy1)
		registry.Register(spy2)

		orch := tools.NewOrchestrator(registry)
		tc := &tools.ToolContext{CWD: t.TempDir()}

		exec := query.NewStreamingToolExecutor(context.Background(), registry, orch, tc)

		exec.AddTool("t1", "tool_a", json.RawMessage(`{}`))
		exec.AddTool("t2", "tool_b", json.RawMessage(`{}`))

		// Wait briefly for execution
		time.Sleep(50 * time.Millisecond)

		if exec.TrackedCount() != 2 {
			t.Errorf("expected 2 tracked tools, got %d", exec.TrackedCount())
		}

		results := exec.GetResults()
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("add_tool_starts_immediately", func(t *testing.T) {
		// Source: StreamingToolExecutor.ts:76 — starts executing immediately
		registry := tools.NewRegistry()
		spy := testharness.NewSpyTool("my_tool", true)
		registry.Register(spy)

		orch := tools.NewOrchestrator(registry)
		tc := &tools.ToolContext{CWD: t.TempDir()}

		exec := query.NewStreamingToolExecutor(context.Background(), registry, orch, tc)
		exec.AddTool("t1", "my_tool", json.RawMessage(`{}`))

		results := exec.GetResults()
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if spy.CallCount() != 1 {
			t.Errorf("expected 1 call, got %d", spy.CallCount())
		}
	})

	t.Run("discard_prevents_new_tools", func(t *testing.T) {
		// Source: StreamingToolExecutor.ts:69-71
		registry := tools.NewRegistry()
		spy := testharness.NewSpyTool("my_tool", true)
		registry.Register(spy)

		orch := tools.NewOrchestrator(registry)
		tc := &tools.ToolContext{CWD: t.TempDir()}

		exec := query.NewStreamingToolExecutor(context.Background(), registry, orch, tc)
		exec.Discard()
		exec.AddTool("t1", "my_tool", json.RawMessage(`{}`))

		if exec.TrackedCount() != 0 {
			t.Errorf("discarded executor should not track tools, got %d", exec.TrackedCount())
		}
	})
}
