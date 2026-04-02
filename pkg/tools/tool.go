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
