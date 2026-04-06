package permissions

import (
	"log/slog"
	"sync"
	"time"
)

// Source: src/hooks/toolPermission/permissionLogging.ts

// PermissionLogger receives permission decision events for telemetry/audit.
type PermissionLogger interface {
	LogDecision(record DecisionRecord)
	LogCancelled(toolName, toolUseID, messageID string)
}

// noopLogger silently discards all events.
type noopLogger struct{}

func (*noopLogger) LogDecision(DecisionRecord)                      {}
func (*noopLogger) LogCancelled(string, string, string)             {}

// SourceToString converts an ApprovalSource or RejectionSource to a
// human-readable label used in analytics and OTel events.
// Source: permissionLogging.ts — sourceToString
func ApprovalSourceToString(s ApprovalSource) string {
	switch s.Type {
	case ApprovalConfig:
		return "config"
	case ApprovalClassifier:
		return "classifier"
	case ApprovalHook:
		return "hook"
	case ApprovalUser:
		if s.Permanent {
			return "user_permanent"
		}
		return "user_temporary"
	default:
		return "unknown"
	}
}

// RejectionSourceToString converts a RejectionSource to a label.
func RejectionSourceToString(s RejectionSource) string {
	switch s.Type {
	case RejectionConfig:
		return "config"
	case RejectionHook:
		return "hook"
	case RejectionUserAbort:
		return "user_abort"
	case RejectionUserReject:
		return "user_reject"
	default:
		return "unknown"
	}
}

// CodeEditingTools is the set of tools that receive OTel counter enrichment.
// Source: permissionLogging.ts — CODE_EDITING_TOOLS
var CodeEditingTools = map[string]bool{
	"Edit":         true,
	"Write":        true,
	"NotebookEdit": true,
}

// IsCodeEditingTool checks if a tool name is a code editing tool.
// Source: permissionLogging.ts — isCodeEditingTool
func IsCodeEditingTool(toolName string) bool {
	return CodeEditingTools[toolName]
}

// PermissionEventName returns the analytics event name for a decision.
// Source: permissionLogging.ts — logApprovalEvent / logRejectionEvent
//
// Approval events:
//
//	config     → tengu_tool_use_granted_in_config
//	classifier → tengu_tool_use_granted_by_classifier
//	user perm  → tengu_tool_use_granted_in_prompt_permanent
//	user temp  → tengu_tool_use_granted_in_prompt_temporary
//	hook       → tengu_tool_use_granted_by_permission_hook
//
// Rejection events:
//
//	config → tengu_tool_use_denied_in_config
//	other  → tengu_tool_use_rejected_in_prompt
func ApprovalEventName(source ApprovalSource) string {
	switch source.Type {
	case ApprovalConfig:
		return "tengu_tool_use_granted_in_config"
	case ApprovalClassifier:
		return "tengu_tool_use_granted_by_classifier"
	case ApprovalUser:
		if source.Permanent {
			return "tengu_tool_use_granted_in_prompt_permanent"
		}
		return "tengu_tool_use_granted_in_prompt_temporary"
	case ApprovalHook:
		return "tengu_tool_use_granted_by_permission_hook"
	default:
		return "tengu_tool_use_granted_in_prompt_temporary"
	}
}

// RejectionEventName returns the analytics event name for a rejection.
func RejectionEventName(source RejectionSource) string {
	if source.Type == RejectionConfig {
		return "tengu_tool_use_denied_in_config"
	}
	return "tengu_tool_use_rejected_in_prompt"
}

// SlogLogger is a PermissionLogger backed by slog.
type SlogLogger struct {
	Logger *slog.Logger
}

// LogDecision logs a permission decision as a structured slog event.
func (l *SlogLogger) LogDecision(r DecisionRecord) {
	l.Logger.Info("permission_decision",
		slog.String("tool", r.ToolName),
		slog.String("tool_use_id", r.ToolUseID),
		slog.String("decision", r.Decision),
		slog.String("source", r.Source),
		slog.Time("timestamp", r.Timestamp),
	)
}

// LogCancelled logs a tool-use cancellation.
func (l *SlogLogger) LogCancelled(toolName, toolUseID, messageID string) {
	l.Logger.Info("tengu_tool_use_cancelled",
		slog.String("tool", toolName),
		slog.String("tool_use_id", toolUseID),
		slog.String("message_id", messageID),
		slog.Time("timestamp", time.Now()),
	)
}

// CollectingLogger captures decisions in a slice for testing.
type CollectingLogger struct {
	mu         sync.Mutex
	Decisions  []DecisionRecord
	Cancelled  []string // toolUseIDs
}

func (l *CollectingLogger) LogDecision(r DecisionRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Decisions = append(l.Decisions, r)
}

func (l *CollectingLogger) LogCancelled(_, toolUseID, _ string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Cancelled = append(l.Cancelled, toolUseID)
}
