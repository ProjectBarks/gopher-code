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

// FeatureGatedTool is an optional interface tools implement to control
// whether they are available in the current environment.
// Default is true (enabled) when not implemented.
// Source: Tool.ts:403, 758
type FeatureGatedTool interface {
	IsEnabled() bool
}

// IsToolEnabled checks if a tool is enabled. Defaults to true.
// Source: Tool.ts:749, 758 — isEnabled: () => true
func IsToolEnabled(tool Tool) bool {
	if gated, ok := tool.(FeatureGatedTool); ok {
		return gated.IsEnabled()
	}
	return true
}

// DeferrableTool is an optional interface for tools that should be deferred
// (sent with defer_loading: true) until ToolSearch is used.
// Source: Tool.ts:438-442
type DeferrableTool interface {
	ShouldDefer() bool
}

// Tool result size constants.
// Source: constants/toolLimits.ts:1-56
const (
	// DefaultMaxResultSizeChars is the system-wide default for tool result persistence.
	// Source: constants/toolLimits.ts:13
	DefaultMaxResultSizeChars = 50_000

	// MaxToolResultTokens is the upper bound for tool result token count.
	// Source: constants/toolLimits.ts:22
	MaxToolResultTokens = 100_000

	// BytesPerToken is the conservative estimate for token-to-byte conversion.
	// Source: constants/toolLimits.ts:28
	BytesPerToken = 4

	// MaxToolResultBytes is the byte limit derived from token limit.
	// Source: constants/toolLimits.ts:33
	MaxToolResultBytes = MaxToolResultTokens * BytesPerToken

	// MaxToolResultsPerMessageChars is the aggregate cap per user message.
	// Source: constants/toolLimits.ts:49
	MaxToolResultsPerMessageChars = 200_000

	// ToolSummaryMaxLength is the max length for compact tool summaries.
	// Source: constants/toolLimits.ts:56
	ToolSummaryMaxLength = 50
)

// maxResultSizeByTool maps tool names to their per-tool maxResultSizeChars.
// Source: grep of `maxResultSizeChars:` across all TS tool definitions.
// Tools not listed use DefaultMaxResultSizeChars. Infinity means never persist.
var maxResultSizeByTool = map[string]int{
	"Bash":             30_000,  // BashTool.tsx:424
	"Read":             -1,     // Infinity — FileReadTool.ts:342
	"Grep":             30_000, // GrepTool.ts
	"Glob":             30_000, // GlobTool.ts
	"Agent":            100_000, // AgentTool.tsx
	"ListMcpResources": 100_000, // ListMcpResourcesTool.ts:53
	"TaskOutput":       100_000, // TaskOutputTool.tsx:147
	"TeamDelete":       100_000, // TeamDeleteTool.ts:35
	"Brief":            100_000, // BriefTool.ts:141
	"RemoteTrigger":    100_000, // RemoteTriggerTool.ts:49
}

// MaxResultSizeCharsProvider is an optional interface for tools to declare their
// max result size. Source: Tool.ts:456-466
type MaxResultSizeCharsProvider interface {
	MaxResultSizeChars() int
}

// GetMaxResultSizeChars returns the max result size for a tool.
// Returns -1 for Infinity (never persist). Uses interface, then map, then default.
func GetMaxResultSizeChars(tool Tool) int {
	if p, ok := tool.(MaxResultSizeCharsProvider); ok {
		return p.MaxResultSizeChars()
	}
	if size, ok := maxResultSizeByTool[tool.Name()]; ok {
		return size
	}
	return DefaultMaxResultSizeChars
}

// ShouldPersistResult checks if a tool result is too large and should be persisted.
// Returns true if the result exceeds the tool's maxResultSizeChars.
// Returns false for tools with Infinity (Read).
func ShouldPersistResult(tool Tool, resultLen int) bool {
	maxSize := GetMaxResultSizeChars(tool)
	if maxSize < 0 {
		return false // Infinity — never persist
	}
	return resultLen > maxSize
}

// deferredToolNames is the set of tool names that are deferred by default.
// Source: grep of `shouldDefer: true` across all TS tool definitions.
var deferredToolNames = map[string]bool{
	"AskUserQuestion": true, "Config": true,
	"CronCreate": true, "CronDelete": true, "CronList": true,
	"EnterPlanMode": true, "ExitPlanMode": true,
	"EnterWorktree": true, "ExitWorktree": true,
	"ListMcpResources": true, "LSP": true, "NotebookEdit": true,
	"ReadMcpResource": true, "RemoteTrigger": true, "SendMessage": true,
	"TaskCreate": true, "TaskGet": true, "TaskList": true,
	"TaskOutput": true, "TaskStop": true, "TaskUpdate": true,
	"TeamCreate": true, "TeamDelete": true, "TodoWrite": true,
	"WebFetch": true, "WebSearch": true,
}

// IsToolDeferred checks if a tool should be deferred. Checks the interface first,
// then falls back to the built-in deferred names set.
// Source: Tool.ts:442
func IsToolDeferred(tool Tool) bool {
	if d, ok := tool.(DeferrableTool); ok {
		return d.ShouldDefer()
	}
	return deferredToolNames[tool.Name()]
}
