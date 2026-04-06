package query

import (
	"context"


	"github.com/google/uuid"
	"github.com/projectbarks/gopher-code/pkg/analytics"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// QueryDeps is the dependency-injection surface for the query loop.
//
// Passing a QueryDeps into Query() lets tests inject fakes directly instead
// of spying on per-module globals. The most common mocks (CallModel,
// Autocompact) are each used in multiple test files today.
//
// Phase 1 (T48): 4 core deps — CallModel, Microcompact, Autocompact, UUID.
// Phase 2 (T50): expanded deps — RunTools, HandleStopHooks, LogEvent, queue ops.
//
// Source: src/query/deps.ts — QueryDeps type
type QueryDeps struct {
	// --- Phase 1: core deps (T48) ---

	// CallModel streams a model request and returns a channel of events.
	// Production: provider.ModelProvider.Stream
	CallModel func(ctx context.Context, req provider.ModelRequest) (<-chan provider.StreamResult, error)

	// Microcompact truncates individual tool results exceeding the threshold.
	// Production: query.microCompact (local helper)
	Microcompact func(content string) string

	// Autocompact compresses message history when context grows too large.
	// Production: compact.MicroCompactMessages
	Autocompact func(messages []message.Message, keepRecent int) ([]message.Message, int)

	// UUID generates a unique identifier string.
	// Production: uuid.New().String()
	UUID func() string

	// --- Phase 2: expanded deps (T50) ---

	// RunTools executes a batch of tool calls and returns results.
	// Production: tools.ToolOrchestrator.ExecuteBatch
	RunTools func(ctx context.Context, calls []tools.ToolCall, tc *tools.ToolContext) []tools.ToolCallResult

	// HandleStopHooks runs stop hooks after the model completes a turn with
	// no tool calls. Returns whether to prevent continuation and any blocking
	// errors to inject.
	// Production: session.StopHookRunner callback
	HandleStopHooks func(assistantTexts []string) StopHookResult

	// LogEvent sends an analytics event.
	// Production: analytics.LogEvent
	LogEvent func(eventName string, metadata analytics.EventMetadata)

	// QueueDequeue removes and returns the next command from the input queue.
	// Returns nil if the queue is empty.
	// Production: CommandQueue.Dequeue
	QueueDequeue func() *QueuedCommand

	// QueueHasCommands returns true if the input queue has pending commands.
	// Production: CommandQueue.HasCommands
	QueueHasCommands func() bool
}

// ProductionDeps creates a QueryDeps wired to real implementations.
//
// prov is the model provider for streaming API calls.
// orchestrator + toolCtx are used for tool execution.
// queue is the command input queue (may be nil for non-interactive sessions).
// stopRunner is the optional stop hook callback (may be nil).
//
// Source: src/query/deps.ts — productionDeps()
func ProductionDeps(
	prov provider.ModelProvider,
	orchestrator *tools.ToolOrchestrator,
	toolCtx *tools.ToolContext,
	queue *CommandQueue,
	stopRunner StopHookRunner,
) QueryDeps {
	deps := QueryDeps{
		// Phase 1 (T48-T49)
		CallModel:    prov.Stream,
		Microcompact: microCompact,
		Autocompact:  compact.MicroCompactMessages,
		UUID:         func() string { return uuid.New().String() },

		// Phase 2 (T50)
		RunTools: func(ctx context.Context, calls []tools.ToolCall, tc *tools.ToolContext) []tools.ToolCallResult {
			return orchestrator.ExecuteBatch(ctx, calls, tc)
		},
		LogEvent: analytics.LogEvent,
	}

	// Stop hooks: wire if provided, else no-op.
	if stopRunner != nil {
		deps.HandleStopHooks = stopRunner
	} else {
		deps.HandleStopHooks = func(_ []string) StopHookResult { return StopHookResult{} }
	}

	// Queue ops: wire if provided, else no-op stubs.
	if queue != nil {
		deps.QueueDequeue = queue.Dequeue
		deps.QueueHasCommands = queue.HasCommands
	} else {
		deps.QueueDequeue = func() *QueuedCommand { return nil }
		deps.QueueHasCommands = func() bool { return false }
	}

	return deps
}

// Validate returns an error if any required dependency is nil.
// This is a development-time safety net — production code should always
// use ProductionDeps or a fully-populated test fixture.
func (d *QueryDeps) Validate() error {
	if d.CallModel == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.CallModel is nil"}
	}
	if d.Microcompact == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.Microcompact is nil"}
	}
	if d.Autocompact == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.Autocompact is nil"}
	}
	if d.UUID == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.UUID is nil"}
	}
	if d.RunTools == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.RunTools is nil"}
	}
	if d.HandleStopHooks == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.HandleStopHooks is nil"}
	}
	if d.LogEvent == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.LogEvent is nil"}
	}
	if d.QueueDequeue == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.QueueDequeue is nil"}
	}
	if d.QueueHasCommands == nil {
		return &AgentError{Kind: ErrProvider, Detail: "QueryDeps.QueueHasCommands is nil"}
	}
	return nil
}
