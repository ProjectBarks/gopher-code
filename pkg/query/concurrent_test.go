// Concurrency tests for the query loop — designed to surface data races when
// run with `go test -race`.
//
// These tests verify:
//   - Multiple read-only tools executing in parallel do not race
//   - Two independent Query() calls on separate sessions do not interfere
//   - Event callbacks are safe under concurrent emission

package query_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// TestConcurrency_ToolExecutionRaceSafe — runs multiple read-only tools
// concurrently via a single multi-tool turn. Under -race, any unsynchronized
// access to shared state will be caught.
func TestConcurrency_ToolExecutionRaceSafe(t *testing.T) {
	// Create several read-only spy tools
	const numTools = 5
	spies := make([]*testharness.SpyTool, numTools)
	specs := make([]testharness.ToolSpec, numTools)

	registry := tools.NewRegistry()
	for i := 0; i < numTools; i++ {
		name := "read_tool_" + string(rune('a'+i))
		spies[i] = testharness.NewSpyTool(name, true) // all read-only
		registry.Register(spies[i])
		specs[i] = testharness.ToolSpec{
			ID:    "t" + string(rune('0'+i)),
			Name:  name,
			Input: json.RawMessage(`{}`),
		}
	}

	prov := testharness.NewScriptedProvider(
		testharness.MakeMultiToolTurn(specs, provider.StopReasonToolUse),
		testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
	)

	orchestrator := tools.NewOrchestrator(registry)
	callback, eventLog := testharness.NewEventCollector()

	sess := testharness.MakeSession()
	err := query.Query(context.Background(), sess, prov, registry, orchestrator, callback)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// All tools should have been called
	for i, spy := range spies {
		if spy.CallCount() != 1 {
			t.Errorf("spy[%d] expected 1 call, got %d", i, spy.CallCount())
		}
	}

	// All tool results should be present
	results := eventLog.ToolResults()
	if len(results) != numTools {
		t.Errorf("expected %d tool results, got %d", numTools, len(results))
	}
}

// TestConcurrency_ParallelQueryCallsIndependent — two Query() calls running
// on separate goroutines with separate sessions and providers must not
// interfere with each other.
func TestConcurrency_ParallelQueryCallsIndependent(t *testing.T) {
	var wg sync.WaitGroup
	errs := make([]error, 2)
	sessions := make([]*session.SessionState, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			spy := testharness.NewSpyTool("tool_"+string(rune('a'+idx)), false)
			prov := testharness.NewScriptedProvider(
				testharness.MakeToolTurn(
					"t1", spy.Name(), json.RawMessage(`{}`), provider.StopReasonToolUse,
				),
				testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
			)

			registry := tools.NewRegistry()
			registry.Register(spy)
			orchestrator := tools.NewOrchestrator(registry)

			sess := testharness.MakeSessionWithConfig(session.SessionConfig{
				Model:          "test-model",
				MaxTurns:       100,
				PermissionMode: permissions.AutoApprove,
			})

			sessions[idx] = sess
			errs[idx] = query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
		}(i)
	}

	wg.Wait()

	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Errorf("query[%d] returned error: %v", i, errs[i])
		}
	}

	// Verify the sessions are independent (different IDs)
	if sessions[0].ID == sessions[1].ID {
		t.Error("sessions should have different IDs")
	}

	// Each session should have 4 messages: user, assistant(tool), user(result), assistant(text)
	for i, sess := range sessions {
		if len(sess.Messages) != 4 {
			t.Errorf("session[%d]: expected 4 messages, got %d", i, len(sess.Messages))
		}
	}
}

// TestConcurrency_EventCallbackUnderLoad — fires many events rapidly to
// ensure the callback mechanism is race-free.
func TestConcurrency_EventCallbackUnderLoad(t *testing.T) {
	// Build a turn with many text deltas to stress the callback path
	chunks := make([]string, 100)
	for i := range chunks {
		chunks[i] = "x"
	}

	prov := testharness.NewScriptedProvider(
		testharness.MakeChunkedTextTurn(chunks, provider.StopReasonEndTurn),
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
	if len(deltas) != 100 {
		t.Errorf("expected 100 text deltas, got %d", len(deltas))
	}
}

// TestConcurrency_MixedReadWriteToolBatch — a batch containing both read-only
// and mutating tools. Read-only run concurrently, mutating run sequentially.
// Under -race, any incorrect concurrency will be caught.
func TestConcurrency_MixedReadWriteToolBatch(t *testing.T) {
	readA := testharness.NewSpyTool("read_a", true)
	readB := testharness.NewSpyTool("read_b", true)
	writeC := testharness.NewSpyTool("write_c", false)

	prov := testharness.NewScriptedProvider(
		testharness.MakeMultiToolTurn([]testharness.ToolSpec{
			{ID: "t1", Name: "read_a", Input: json.RawMessage(`{}`)},
			{ID: "t2", Name: "read_b", Input: json.RawMessage(`{}`)},
			{ID: "t3", Name: "write_c", Input: json.RawMessage(`{}`)},
		}, provider.StopReasonToolUse),
		testharness.MakeTextTurn("done", provider.StopReasonEndTurn),
	)

	registry := tools.NewRegistry()
	registry.Register(readA)
	registry.Register(readB)
	registry.Register(writeC)
	orchestrator := tools.NewOrchestrator(registry)

	sess := testharness.MakeSession()
	err := query.Query(context.Background(), sess, prov, registry, orchestrator, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if readA.CallCount() != 1 {
		t.Errorf("read_a: expected 1 call, got %d", readA.CallCount())
	}
	if readB.CallCount() != 1 {
		t.Errorf("read_b: expected 1 call, got %d", readB.CallCount())
	}
	if writeC.CallCount() != 1 {
		t.Errorf("write_c: expected 1 call, got %d", writeC.CallCount())
	}
}
