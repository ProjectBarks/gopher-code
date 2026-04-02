package provider

import "encoding/json"

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type RequestContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   *bool           `json:"is_error,omitempty"`
}

type RequestMessage struct {
	Role    string           `json:"role"`
	Content []RequestContent `json:"content"`
}

// ThinkingConfig controls extended thinking / reasoning effort.
type ThinkingConfig struct {
	Type         string `json:"type"`                    // "enabled" or "disabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"` // token budget for thinking
}

type ModelRequest struct {
	Model       string           `json:"model"`
	System      string           `json:"system,omitempty"`
	Messages    []RequestMessage `json:"messages"`
	MaxTokens   int              `json:"max_tokens"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	Thinking    *ThinkingConfig  `json:"thinking,omitempty"`
	JSONSchema  json.RawMessage  `json:"json_schema,omitempty"`
}
