package query

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

// Source: services/tools/StreamingToolExecutor.ts

// ToolStatus tracks the state of a tool in the streaming executor.
// Source: StreamingToolExecutor.ts:19
type ToolStatus string

const (
	ToolQueued    ToolStatus = "queued"
	ToolExecuting ToolStatus = "executing"
	ToolCompleted ToolStatus = "completed"
)

// TrackedTool is a tool queued for execution in the streaming executor.
// Source: StreamingToolExecutor.ts:21-32
type TrackedTool struct {
	ID                string
	Name              string
	Input             json.RawMessage
	Status            ToolStatus
	IsConcurrencySafe bool
	Result            *tools.ToolCallResult
}

// StreamingToolExecutor executes tools as they stream in with concurrency control.
// Concurrent-safe tools execute in parallel; non-concurrent tools execute exclusively.
// Results are buffered and emitted in the order tools were received.
// Source: services/tools/StreamingToolExecutor.ts:40-62
type StreamingToolExecutor struct {
	mu           sync.Mutex
	tracked      []*TrackedTool
	registry     *tools.ToolRegistry
	orchestrator *tools.ToolOrchestrator
	toolCtx      *tools.ToolContext
	ctx          context.Context
	discarded    bool
}

// NewStreamingToolExecutor creates a new executor.
func NewStreamingToolExecutor(
	ctx context.Context,
	registry *tools.ToolRegistry,
	orchestrator *tools.ToolOrchestrator,
	toolCtx *tools.ToolContext,
) *StreamingToolExecutor {
	return &StreamingToolExecutor{
		ctx:          ctx,
		registry:     registry,
		orchestrator: orchestrator,
		toolCtx:      toolCtx,
	}
}

// AddTool queues a tool for execution. Starts executing immediately if safe.
// Source: StreamingToolExecutor.ts:76
func (e *StreamingToolExecutor) AddTool(id, name string, input json.RawMessage) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.discarded {
		return
	}

	tool := e.registry.Get(name)
	isSafe := false
	if tool != nil {
		isSafe = tools.CheckConcurrencySafe(tool, input)
	}

	tracked := &TrackedTool{
		ID:                id,
		Name:              name,
		Input:             input,
		Status:            ToolQueued,
		IsConcurrencySafe: isSafe,
	}
	e.tracked = append(e.tracked, tracked)

	// Start execution if possible
	e.tryExecute(tracked)
}

// tryExecute attempts to start a tool. Must be called with mu held.
func (e *StreamingToolExecutor) tryExecute(t *TrackedTool) {
	if t.Status != ToolQueued {
		return
	}

	// Check if any non-concurrent tool is executing
	for _, other := range e.tracked {
		if other.Status == ToolExecuting && !other.IsConcurrencySafe {
			return // Can't start — exclusive tool running
		}
	}

	// If this tool is non-concurrent, check if anything is executing
	if !t.IsConcurrencySafe {
		for _, other := range e.tracked {
			if other.Status == ToolExecuting {
				return // Can't start — another tool running
			}
		}
	}

	t.Status = ToolExecuting
	go e.executeTool(t)
}

// executeTool runs a single tool and stores the result.
func (e *StreamingToolExecutor) executeTool(t *TrackedTool) {
	call := tools.ToolCall{ID: t.ID, Name: t.Name, Input: t.Input}
	results := e.orchestrator.ExecuteBatch(e.ctx, []tools.ToolCall{call}, e.toolCtx)

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(results) > 0 {
		t.Result = &results[0]
	}
	t.Status = ToolCompleted

	// Try to start queued tools now that this one is done
	for _, other := range e.tracked {
		if other.Status == ToolQueued {
			e.tryExecute(other)
		}
	}
}

// Discard abandons all pending tools.
// Source: StreamingToolExecutor.ts:69-71
func (e *StreamingToolExecutor) Discard() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.discarded = true
}

// GetResults returns all completed results in order, as tool_result content blocks.
// Blocks until all tracked tools are complete.
func (e *StreamingToolExecutor) GetResults() []message.ContentBlock {
	// Wait for all tools to complete
	for {
		e.mu.Lock()
		allDone := true
		for _, t := range e.tracked {
			if t.Status != ToolCompleted {
				allDone = false
				break
			}
		}
		e.mu.Unlock()

		if allDone {
			break
		}
		// Yield to let goroutines complete
		// In production this would use a sync.Cond or channel
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	var blocks []message.ContentBlock
	for _, t := range e.tracked {
		if t.Result != nil {
			blocks = append(blocks, message.ToolResultBlock(
				t.Result.ToolUseID,
				t.Result.Output.Content,
				t.Result.Output.IsError,
			))
		}
	}
	return blocks
}

// TrackedCount returns the number of tracked tools.
func (e *StreamingToolExecutor) TrackedCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.tracked)
}
