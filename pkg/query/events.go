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

// StopHookResult is the result of running stop hooks after a model turn completes.
// Source: query/stopHooks.ts:60-63
type StopHookResult struct {
	// BlockingErrors are messages to inject back into the conversation
	// (e.g., hook stderr output). The query loop will continue with these.
	BlockingErrors []string
	// PreventContinuation stops the query loop entirely when true.
	// Source: query/stopHooks.ts:62
	PreventContinuation bool
}

// StopHookRunner is an optional callback that runs after the model completes
// a turn with no tool calls (the "completed" path). It can:
// - Prevent continuation entirely (PreventContinuation=true)
// - Inject blocking error messages that cause the loop to continue
// Source: query.ts:1267-1305
type StopHookRunner func(messages []string) StopHookResult
