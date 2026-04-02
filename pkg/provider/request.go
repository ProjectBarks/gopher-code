package provider

import "encoding/json"

// CacheControl marks a content block for prompt caching.
// Source: services/api/claude.ts:358-374
type CacheControl struct {
	Type  string `json:"type"`            // "ephemeral"
	TTL   string `json:"ttl,omitempty"`   // "1h" for extended caching
	Scope string `json:"scope,omitempty"` // "global" for cross-org caching
}

type ToolDefinition struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	DeferLoading bool            `json:"defer_loading,omitempty"` // Source: Tool.ts:439-442
	CacheControl *CacheControl   `json:"cache_control,omitempty"` // Source: claude.ts:1388
}

type RequestContent struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	Content      string          `json:"content,omitempty"`
	IsError      *bool           `json:"is_error,omitempty"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"` // Source: claude.ts:603-663
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
	Model        string           `json:"model"`
	System       string           `json:"system,omitempty"`
	SystemBlocks []SystemBlock    `json:"system_blocks,omitempty"` // Alternative: array form with cache_control
	Messages     []RequestMessage `json:"messages"`
	MaxTokens    int              `json:"max_tokens"`
	Tools        []ToolDefinition `json:"tools,omitempty"`
	Temperature  *float64         `json:"temperature,omitempty"`
	Thinking     *ThinkingConfig  `json:"thinking,omitempty"`
	JSONSchema   json.RawMessage  `json:"json_schema,omitempty"`
}

// SystemBlock is a content block in the system prompt array (for cache_control).
// Source: claude.ts:603-615
type SystemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// EphemeralCacheControl returns a cache_control marker for prompt caching.
// Source: services/api/claude.ts:358-374
func EphemeralCacheControl() *CacheControl {
	return &CacheControl{Type: "ephemeral"}
}
