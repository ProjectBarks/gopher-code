package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/projectbarks/gopher-code/pkg/permissions"
)

// ToolCall represents a pending tool call from the model response.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// ToolCallResult is the result of executing a single tool call.
type ToolCallResult struct {
	ToolUseID string
	Output    ToolOutput
}

// ToolOrchestrator executes batches of tool calls.
type ToolOrchestrator struct {
	registry *ToolRegistry
}

// NewOrchestrator creates a new orchestrator backed by a registry.
func NewOrchestrator(registry *ToolRegistry) *ToolOrchestrator {
	return &ToolOrchestrator{registry: registry}
}

// ExecuteBatch executes a batch of tool calls. Read-only tools run concurrently,
// mutating tools run sequentially.
func (o *ToolOrchestrator) ExecuteBatch(ctx context.Context, calls []ToolCall, tc *ToolContext) []ToolCallResult {
	results := make([]ToolCallResult, 0, len(calls))

	var concurrent, sequential []ToolCall
	for _, call := range calls {
		tool := o.registry.Get(call.Name)
		if tool != nil && tool.IsReadOnly() {
			concurrent = append(concurrent, call)
		} else {
			sequential = append(sequential, call)
		}
	}

	if len(concurrent) > 0 {
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, call := range concurrent {
			wg.Add(1)
			go func(c ToolCall) {
				defer wg.Done()
				r := o.executeSingle(ctx, c, tc)
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
			}(call)
		}
		wg.Wait()
	}

	for _, call := range sequential {
		results = append(results, o.executeSingle(ctx, call, tc))
	}

	return results
}

func (o *ToolOrchestrator) executeSingle(ctx context.Context, call ToolCall, tc *ToolContext) ToolCallResult {
	tool := o.registry.Get(call.Name)
	if tool == nil {
		return ToolCallResult{
			ToolUseID: call.ID,
			Output:    *ErrorOutput(fmt.Sprintf("unknown tool: %s", call.Name)),
		}
	}

	if !tool.IsReadOnly() && tc.Permissions != nil {
		decision := tc.Permissions.Check(ctx, call.Name, call.ID)
		switch d := decision.(type) {
		case permissions.DenyDecision:
			return ToolCallResult{
				ToolUseID: call.ID,
				Output:    *ErrorOutput(fmt.Sprintf("permission denied: %s", d.Reason)),
			}
		case permissions.AskDecision:
			return ToolCallResult{
				ToolUseID: call.ID,
				Output:    *ErrorOutput(fmt.Sprintf("permission required: %s", d.Message)),
			}
		}
	}

	output, err := tool.Execute(ctx, tc, call.Input)
	if err != nil {
		return ToolCallResult{
			ToolUseID: call.ID,
			Output:    *ErrorOutput(fmt.Sprintf("tool execution failed: %s", err)),
		}
	}

	return ToolCallResult{
		ToolUseID: call.ID,
		Output:    *output,
	}
}
