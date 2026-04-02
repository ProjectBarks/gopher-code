package tools

import (
	"context"
	"encoding/json"
)

// ToolOutput represents the result of executing a tool.
type ToolOutput struct {
	Content  string          `json:"content"`
	IsError  bool            `json:"is_error"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// SuccessOutput creates a successful tool output.
func SuccessOutput(content string) *ToolOutput {
	return &ToolOutput{Content: content, IsError: false}
}

// ErrorOutput creates an error tool output.
func ErrorOutput(content string) *ToolOutput {
	return &ToolOutput{Content: content, IsError: true}
}

// Tool is the interface every built-in or plugin tool must implement.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error)
	IsReadOnly() bool
}

// ConcurrencySafeChecker is an optional interface tools can implement
// to provide per-call concurrency safety evaluation.
// If not implemented, falls back to IsReadOnly().
// Source: Tool.ts:402 — isConcurrencySafe(input)
type ConcurrencySafeChecker interface {
	IsConcurrencySafe(input json.RawMessage) bool
}

// CheckConcurrencySafe checks if a tool call is safe for concurrent execution.
// Uses ConcurrencySafeChecker if available, otherwise falls back to IsReadOnly().
// Source: Tool.ts:759 — default false
func CheckConcurrencySafe(tool Tool, input json.RawMessage) bool {
	if checker, ok := tool.(ConcurrencySafeChecker); ok {
		return checker.IsConcurrencySafe(input)
	}
	return tool.IsReadOnly()
}

// DestructiveChecker is an optional interface tools can implement
// to signal that a call performs irreversible operations (delete, overwrite, send).
// Defaults to false when not implemented.
// Source: Tool.ts:405-406
type DestructiveChecker interface {
	IsDestructive(input json.RawMessage) bool
}

// CheckDestructive checks if a tool call is destructive (irreversible).
// Source: Tool.ts:752, 761 — default false
func CheckDestructive(tool Tool, input json.RawMessage) bool {
	if checker, ok := tool.(DestructiveChecker); ok {
		return checker.IsDestructive(input)
	}
	return false
}

// PermissionCheckResult is the result of a tool-specific permission check.
// Source: Tool.ts:500-503 — checkPermissions returns PermissionResult
type PermissionCheckResult struct {
	Behavior string // "allow", "deny", "ask", "passthrough"
	Message  string // Reason for deny/ask
}

// ToolPermissionChecker is an optional interface tools implement for
// tool-specific permission logic. Called before the generic waterfall.
// Default behavior is "allow" (passthrough to generic system).
// Source: Tool.ts:495-503, 762-766
type ToolPermissionChecker interface {
	CheckPermissions(ctx context.Context, tc *ToolContext, input json.RawMessage) PermissionCheckResult
}

// CheckToolPermissions calls the tool's CheckPermissions if implemented.
// Returns nil if the tool doesn't implement the interface (use generic check).
// Source: Tool.ts:762-766 — default: { behavior: 'allow', updatedInput: input }
func CheckToolPermissions(tool Tool, ctx context.Context, tc *ToolContext, input json.RawMessage) *PermissionCheckResult {
	if checker, ok := tool.(ToolPermissionChecker); ok {
		result := checker.CheckPermissions(ctx, tc, input)
		return &result
	}
	return nil // Default: passthrough to generic permission system
}
