package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

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

// ProgressCallback is called when a tool starts, makes progress, or completes.
// Source: services/tools/toolExecution.ts — ToolProgress callbacks
type ProgressCallback func(toolUseID, toolName string, event ProgressEvent)

// ProgressEvent describes a tool execution lifecycle event.
type ProgressEvent struct {
	Type     string // "started", "progress", "completed", "error"
	Duration time.Duration
	Message  string
}

// ToolOrchestrator executes batches of tool calls.
// Source: services/tools/toolExecution.ts — runTools
type ToolOrchestrator struct {
	registry   *ToolRegistry
	hookRunner HookRunner
	onProgress ProgressCallback // optional progress reporting
}

// NewOrchestrator creates a new orchestrator backed by a registry.
func NewOrchestrator(registry *ToolRegistry) *ToolOrchestrator {
	return &ToolOrchestrator{registry: registry}
}

// SetHookRunner sets the hook runner for pre/post tool execution hooks.
func (o *ToolOrchestrator) SetHookRunner(hr HookRunner) {
	o.hookRunner = hr
}

// SetProgressCallback sets the progress callback for tool execution lifecycle events.
func (o *ToolOrchestrator) SetProgressCallback(cb ProgressCallback) {
	o.onProgress = cb
}

func (o *ToolOrchestrator) emitProgress(toolUseID, toolName string, event ProgressEvent) {
	if o.onProgress != nil {
		o.onProgress(toolUseID, toolName, event)
	}
}

// ExecuteBatch executes a batch of tool calls. Read-only tools run concurrently,
// mutating tools run sequentially.
func (o *ToolOrchestrator) ExecuteBatch(ctx context.Context, calls []ToolCall, tc *ToolContext) []ToolCallResult {
	// Auto-set hooks from orchestrator if not already set on context
	if tc.Hooks == nil && o.hookRunner != nil {
		tc.Hooks = o.hookRunner
	}

	results := make([]ToolCallResult, 0, len(calls))

	var concurrent, sequential []ToolCall
	for _, call := range calls {
		tool := o.registry.Get(call.Name)
		if tool != nil && CheckConcurrencySafe(tool, call.Input) {
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
	start := time.Now()

	tool := o.registry.Get(call.Name)
	if tool == nil {
		return ToolCallResult{
			ToolUseID: call.ID,
			Output:    *ErrorOutput(fmt.Sprintf("unknown tool: %s", call.Name)),
		}
	}

	o.emitProgress(call.ID, call.Name, ProgressEvent{Type: "started"})

	// Pre-tool hook
	if tc.Hooks != nil {
		blocked, msg, _ := tc.Hooks.RunForOrchestrator(ctx, "PreToolUse", call.Name, call.Input)
		if blocked {
			o.emitProgress(call.ID, call.Name, ProgressEvent{Type: "error", Message: msg, Duration: time.Since(start)})
			return ToolCallResult{
				ToolUseID: call.ID,
				Output:    *ErrorOutput(fmt.Sprintf("blocked by hook: %s", msg)),
			}
		}
	}

	// Tool-specific permission check (security, validation, destructive warnings).
	if result := CheckToolPermissions(tool, ctx, tc, call.Input); result != nil {
		switch result.Behavior {
		case "deny":
			o.emitProgress(call.ID, call.Name, ProgressEvent{Type: "error", Message: result.Message, Duration: time.Since(start)})
			return ToolCallResult{
				ToolUseID: call.ID,
				Output:    *ErrorOutput(fmt.Sprintf("permission denied: %s", result.Message)),
			}
		case "ask":
			// For "ask", fall through to the generic permission system.
		}
	}

	if !tool.IsReadOnly() && tc.Permissions != nil {
		decision := tc.Permissions.Check(ctx, call.Name, call.ID)
		switch d := decision.(type) {
		case permissions.DenyDecision:
			o.emitProgress(call.ID, call.Name, ProgressEvent{Type: "error", Message: d.Reason, Duration: time.Since(start)})
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
	elapsed := time.Since(start)

	if err != nil {
		o.emitProgress(call.ID, call.Name, ProgressEvent{Type: "error", Message: err.Error(), Duration: elapsed})
		slog.Debug("tool execution failed", "tool", call.Name, "error", err, "duration_ms", elapsed.Milliseconds())
		return ToolCallResult{
			ToolUseID: call.ID,
			Output:    *ErrorOutput(fmt.Sprintf("tool execution failed: %s", err)),
		}
	}

	// Post-tool hook
	if tc.Hooks != nil {
		tc.Hooks.RunForOrchestrator(ctx, "PostToolUse", call.Name, call.Input)
	}

	o.emitProgress(call.ID, call.Name, ProgressEvent{Type: "completed", Duration: elapsed})
	slog.Debug("tool completed", "tool", call.Name, "duration_ms", elapsed.Milliseconds(), "is_error", output.IsError)

	return ToolCallResult{
		ToolUseID: call.ID,
		Output:    *output,
	}
}
