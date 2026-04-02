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
