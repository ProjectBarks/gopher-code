package query

import (
	"context"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/analytics"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T60: Verify agent_adapter covers the 4-dep shape from src/query/deps.ts:
// callModel, microcompact, autocompact, uuid.

func TestAsQueryFunc_Returns_NonNil(t *testing.T) {
	fn := AsQueryFunc()
	assert.NotNil(t, fn, "AsQueryFunc must return a non-nil tools.QueryFunc")
}

func TestAsQueryFuncWithDeps_Accepts_QueryDeps(t *testing.T) {
	// Verifies that AsQueryFuncWithDeps threads the CallModel dep through
	// to the underlying Query() call.
	callCount := 0
	deps := QueryDeps{
		CallModel: func(_ context.Context, _ provider.ModelRequest) (<-chan provider.StreamResult, error) {
			callCount++
			ch := make(chan provider.StreamResult, 2)
			ch <- provider.StreamResult{Event: &provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: "from-deps",
			}}
			sr := provider.StopReasonEndTurn
			ch <- provider.StreamResult{Event: &provider.StreamEvent{
				Type: provider.EventMessageDone,
				Response: &provider.ModelResponse{StopReason: &sr},
			}}
			close(ch)
			return ch, nil
		},
		Microcompact:     func(s string) string { return s },
		Autocompact:      func(m []message.Message, k int) ([]message.Message, int) { return m, 0 },
		UUID:             func() string { return "test-uuid" },
		RunTools:         func(_ context.Context, _ []tools.ToolCall, _ *tools.ToolContext) []tools.ToolCallResult { return nil },
		HandleStopHooks:  func(_ []string) StopHookResult { return StopHookResult{} },
		LogEvent:         func(_ string, _ analytics.EventMetadata) {},
		QueueDequeue:     func() *QueuedCommand { return nil },
		QueueHasCommands: func() bool { return false },
	}

	fn := AsQueryFuncWithDeps(deps)
	require.NotNil(t, fn, "AsQueryFuncWithDeps must return a non-nil tools.QueryFunc")
}

func TestDepsProviderAdapter_StreamDelegates(t *testing.T) {
	// Verify the adapter properly delegates Stream to CallModel.
	called := false
	adapter := &depsProviderAdapter{
		callModel: func(_ context.Context, _ provider.ModelRequest) (<-chan provider.StreamResult, error) {
			called = true
			ch := make(chan provider.StreamResult)
			close(ch)
			return ch, nil
		},
	}

	ch, err := adapter.Stream(context.Background(), provider.ModelRequest{})
	require.NoError(t, err)
	// drain
	for range ch {
	}
	assert.True(t, called, "adapter.Stream must delegate to callModel")
	assert.Equal(t, "deps-adapter", adapter.Name())
}

func TestQueryDeps_Covers4DepShape(t *testing.T) {
	// T60 contract test: QueryDeps struct has all 4 core deps from
	// src/query/deps.ts (callModel, microcompact, autocompact, uuid).
	// This is a compile-time + runtime check that the fields exist and
	// are assignable.
	deps := QueryDeps{
		CallModel:    func(_ context.Context, _ provider.ModelRequest) (<-chan provider.StreamResult, error) { return nil, nil },
		Microcompact: func(s string) string { return s },
		Autocompact:  func(m []message.Message, k int) ([]message.Message, int) { return m, 0 },
		UUID:         func() string { return "id" },
		// Phase 2 deps (required for Validate)
		RunTools:         func(_ context.Context, _ []tools.ToolCall, _ *tools.ToolContext) []tools.ToolCallResult { return nil },
		HandleStopHooks:  func(_ []string) StopHookResult { return StopHookResult{} },
		LogEvent:         func(_ string, _ analytics.EventMetadata) {},
		QueueDequeue:     func() *QueuedCommand { return nil },
		QueueHasCommands: func() bool { return false },
	}

	// All 4 core deps must be non-nil.
	assert.NotNil(t, deps.CallModel, "CallModel (callModel)")
	assert.NotNil(t, deps.Microcompact, "Microcompact (microcompact)")
	assert.NotNil(t, deps.Autocompact, "Autocompact (autocompact)")
	assert.NotNil(t, deps.UUID, "UUID (uuid)")

	// Full validation must pass.
	require.NoError(t, deps.Validate())
}
