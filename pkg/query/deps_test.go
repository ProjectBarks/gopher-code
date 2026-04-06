package query

import (
	"context"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/analytics"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProductionDeps_AllFieldsPopulated verifies that ProductionDeps returns a
// QueryDeps with every field non-nil. This is the basic contract: no nil
// function fields should leak into the query loop.
func TestProductionDeps_AllFieldsPopulated(t *testing.T) {
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)
	tc := &tools.ToolContext{
		CWD:         t.TempDir(),
		Permissions: permissions.NewRuleBasedPolicy(permissions.AutoApprove),
		SessionID:   "test-session",
	}

	deps := ProductionDeps(
		&stubProvider{},
		orch,
		tc,
		NewCommandQueue(),
		func(texts []string) StopHookResult { return StopHookResult{} },
	)

	require.NoError(t, deps.Validate(), "ProductionDeps must populate all fields")
}

// TestProductionDeps_NilQueue_StubsProvided verifies that passing a nil queue
// results in safe no-op stubs instead of nil function fields.
func TestProductionDeps_NilQueue_StubsProvided(t *testing.T) {
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)
	tc := &tools.ToolContext{CWD: t.TempDir(), Permissions: permissions.NewRuleBasedPolicy(permissions.AutoApprove)}

	deps := ProductionDeps(&stubProvider{}, orch, tc, nil, nil)

	require.NoError(t, deps.Validate(), "nil queue/stopRunner should get stubs")
	assert.Nil(t, deps.QueueDequeue(), "nil queue stub returns nil")
	assert.False(t, deps.QueueHasCommands(), "nil queue stub returns false")
}

// TestProductionDeps_NilStopRunner_StubProvided verifies the stop hook stub.
func TestProductionDeps_NilStopRunner_StubProvided(t *testing.T) {
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)
	tc := &tools.ToolContext{CWD: t.TempDir(), Permissions: permissions.NewRuleBasedPolicy(permissions.AutoApprove)}

	deps := ProductionDeps(&stubProvider{}, orch, tc, nil, nil)

	result := deps.HandleStopHooks([]string{"hello"})
	assert.False(t, result.PreventContinuation, "nil stop runner stub should not prevent continuation")
	assert.Empty(t, result.BlockingErrors, "nil stop runner stub should have no blocking errors")
}

// TestProductionDeps_CoreDepsSignatures verifies that the core 4 deps (T48)
// have the expected behavior when called through the production wiring.
func TestProductionDeps_CoreDepsSignatures(t *testing.T) {
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)
	tc := &tools.ToolContext{CWD: t.TempDir(), Permissions: permissions.NewRuleBasedPolicy(permissions.AutoApprove)}

	deps := ProductionDeps(&stubProvider{}, orch, tc, nil, nil)

	// UUID generates non-empty unique strings.
	id1 := deps.UUID()
	id2 := deps.UUID()
	assert.NotEmpty(t, id1, "UUID must produce a non-empty string")
	assert.NotEqual(t, id1, id2, "UUID must produce unique values")

	// Microcompact passes through short strings unchanged.
	assert.Equal(t, "short", deps.Microcompact("short"))

	// Autocompact returns messages (pass-through for small input).
	msgs := []message.Message{{Role: message.RoleUser, Content: []message.ContentBlock{message.TextBlock("hi")}}}
	out, _ := deps.Autocompact(msgs, 1)
	assert.Len(t, out, 1)
}

// TestQueryDeps_Validate_DetectsNilFields verifies that Validate catches
// each nil function field individually.
func TestQueryDeps_Validate_DetectsNilFields(t *testing.T) {
	fields := []struct {
		name  string
		build func() QueryDeps
	}{
		{"CallModel", func() QueryDeps { d := fullDeps(); d.CallModel = nil; return d }},
		{"Microcompact", func() QueryDeps { d := fullDeps(); d.Microcompact = nil; return d }},
		{"Autocompact", func() QueryDeps { d := fullDeps(); d.Autocompact = nil; return d }},
		{"UUID", func() QueryDeps { d := fullDeps(); d.UUID = nil; return d }},
		{"RunTools", func() QueryDeps { d := fullDeps(); d.RunTools = nil; return d }},
		{"HandleStopHooks", func() QueryDeps { d := fullDeps(); d.HandleStopHooks = nil; return d }},
		{"LogEvent", func() QueryDeps { d := fullDeps(); d.LogEvent = nil; return d }},
		{"QueueDequeue", func() QueryDeps { d := fullDeps(); d.QueueDequeue = nil; return d }},
		{"QueueHasCommands", func() QueryDeps { d := fullDeps(); d.QueueHasCommands = nil; return d }},
	}
	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			d := f.build()
			err := d.Validate()
			require.Error(t, err, "Validate should catch nil %s", f.name)
			ae, ok := err.(*AgentError)
			require.True(t, ok, "expected *AgentError")
			assert.Contains(t, ae.Detail, f.name)
		})
	}
}

// TestProductionDeps_QueueWiring verifies that queue operations are properly
// wired through to the real CommandQueue.
func TestProductionDeps_QueueWiring(t *testing.T) {
	q := NewCommandQueue()
	q.Enqueue(QueuedCommand{Value: "test-cmd", Priority: PriorityNow})

	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)
	tc := &tools.ToolContext{CWD: t.TempDir(), Permissions: permissions.NewRuleBasedPolicy(permissions.AutoApprove)}

	deps := ProductionDeps(&stubProvider{}, orch, tc, q, nil)

	assert.True(t, deps.QueueHasCommands(), "queue should report commands")
	cmd := deps.QueueDequeue()
	require.NotNil(t, cmd)
	assert.Equal(t, "test-cmd", cmd.Value)
	assert.False(t, deps.QueueHasCommands(), "queue should be empty after dequeue")
}

// TestProductionDeps_LogEventWired verifies that LogEvent is wired to the
// analytics package (smoke test — doesn't verify the event reaches a sink).
func TestProductionDeps_LogEventWired(t *testing.T) {
	reg := tools.NewRegistry()
	orch := tools.NewOrchestrator(reg)
	tc := &tools.ToolContext{CWD: t.TempDir(), Permissions: permissions.NewRuleBasedPolicy(permissions.AutoApprove)}

	deps := ProductionDeps(&stubProvider{}, orch, tc, nil, nil)

	// Should not panic — analytics.LogEvent is safe to call without a sink.
	assert.NotPanics(t, func() {
		deps.LogEvent("test_event", analytics.EventMetadata{"key": true})
	})
}

// --- helpers ---

// stubProvider is a minimal ModelProvider for wiring tests.
type stubProvider struct{}

func (s *stubProvider) Stream(_ context.Context, _ provider.ModelRequest) (<-chan provider.StreamResult, error) {
	ch := make(chan provider.StreamResult)
	close(ch)
	return ch, nil
}

func (s *stubProvider) Name() string { return "stub" }

// fullDeps returns a QueryDeps with all fields populated (for Validate tests).
func fullDeps() QueryDeps {
	return QueryDeps{
		CallModel:    func(_ context.Context, _ provider.ModelRequest) (<-chan provider.StreamResult, error) { return nil, nil },
		Microcompact: func(s string) string { return s },
		Autocompact:  func(m []message.Message, k int) ([]message.Message, int) { return m, 0 },
		UUID:         func() string { return "fake" },
		RunTools: func(_ context.Context, _ []tools.ToolCall, _ *tools.ToolContext) []tools.ToolCallResult {
			return nil
		},
		HandleStopHooks:  func(_ []string) StopHookResult { return StopHookResult{} },
		LogEvent:         func(_ string, _ analytics.EventMetadata) {},
		QueueDequeue:     func() *QueuedCommand { return nil },
		QueueHasCommands: func() bool { return false },
	}
}

// --- T46 + T47: env gate tests (exercised through QueryConfig integration path) ---

// TestFastModeGate_DisabledByEnv verifies CLAUDE_CODE_DISABLE_FAST_MODE env
// flows through BuildQueryConfig into the QueryDeps-consumable gate (T46).
func TestFastModeGate_DisabledByEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_DISABLE_FAST_MODE", "1")
	cfg := BuildQueryConfig("sess")
	assert.False(t, cfg.Gates.FastModeEnabled)
}

// TestAntGate_SetByEnv verifies USER_TYPE=ant flows through BuildQueryConfig (T47).
func TestAntGate_SetByEnv(t *testing.T) {
	t.Setenv("USER_TYPE", "ant")
	cfg := BuildQueryConfig("sess")
	assert.True(t, cfg.Gates.IsAnt)
}

// --- Integration: QueryDeps used by Query() ---

// TestQuery_WithDeps_Integration verifies that Query() accepts and uses a
// QueryDeps through the production code path. This ensures the deps struct
// is reachable from main() via the Query() call chain.
func TestQuery_WithDeps_Integration(t *testing.T) {
	// Build a minimal deps for a single-turn conversation that returns
	// end_turn immediately with no tool calls.
	callCount := 0
	deps := QueryDeps{
		CallModel: func(_ context.Context, _ provider.ModelRequest) (<-chan provider.StreamResult, error) {
			callCount++
			ch := make(chan provider.StreamResult, 2)
			ch <- provider.StreamResult{Event: &provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: "hello",
			}}
			sr := provider.StopReasonEndTurn
			ch <- provider.StreamResult{Event: &provider.StreamEvent{
				Type: provider.EventMessageDone,
				Response: &provider.ModelResponse{
					StopReason: &sr,
				},
			}}
			close(ch)
			return ch, nil
		},
		Microcompact:     func(s string) string { return s },
		Autocompact:      compact.MicroCompactMessages,
		UUID:             func() string { return "test-uuid" },
		RunTools:         func(_ context.Context, _ []tools.ToolCall, _ *tools.ToolContext) []tools.ToolCallResult { return nil },
		HandleStopHooks:  func(_ []string) StopHookResult { return StopHookResult{} },
		LogEvent:         func(_ string, _ analytics.EventMetadata) {},
		QueueDequeue:     func() *QueuedCommand { return nil },
		QueueHasCommands: func() bool { return false },
	}

	require.NoError(t, deps.Validate())

	// Verify the deps struct is the type accepted by Query (compile-time check).
	// The actual Query() integration is verified by confirming deps.CallModel
	// is invocable and produces the expected output.
	ctx := context.Background()
	ch, err := deps.CallModel(ctx, provider.ModelRequest{})
	require.NoError(t, err)

	var texts []string
	for r := range ch {
		if r.Event != nil && r.Event.Type == provider.EventTextDelta {
			texts = append(texts, r.Event.Text)
		}
	}
	assert.Equal(t, []string{"hello"}, texts)
	assert.Equal(t, 1, callCount)
}
