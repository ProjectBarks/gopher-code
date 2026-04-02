package query

import "github.com/projectbarks/gopher-code/pkg/provider"

// QueryEventType identifies the kind of query event.
type QueryEventType string

const (
	QEventTextDelta    QueryEventType = "text_delta"
	QEventToolUseStart QueryEventType = "tool_use_start"
	QEventToolResult   QueryEventType = "tool_result"
	QEventTurnComplete QueryEventType = "turn_complete"
	QEventUsage        QueryEventType = "usage"
)

// QueryEvent is emitted during a query for the caller (CLI/UI) to observe.
type QueryEvent struct {
	Type          QueryEventType
	Text          string              // TextDelta
	ToolUseID     string              // ToolUseStart, ToolResult
	ToolName      string              // ToolUseStart
	Content       string              // ToolResult
	IsError       bool                // ToolResult
	StopReason    provider.StopReason // TurnComplete
	InputTokens   int                 // Usage
	OutputTokens  int                 // Usage
	CacheCreation *int                // Usage
	CacheRead     *int                // Usage
}

// EventCallback is the function signature for streaming query events.
type EventCallback func(QueryEvent)
