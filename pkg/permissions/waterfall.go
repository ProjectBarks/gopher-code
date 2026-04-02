package permissions

import (
	"context"
	"fmt"
)

// Source: utils/permissions/permissions.ts:1158-1319

// WaterfallPolicy implements the TS permission evaluation waterfall:
// 1. Deny rules → deny
// 2. Ask rules → ask (unless sandbox auto-allow)
// 3. Tool-specific check (CheckPermissions)
// 4. Tool deny/ask → respect even in bypass mode
// 5. Bypass mode → allow
// 6. Allow rules → allow
// 7. Fallback → ask (prompt user)
//
// Source: utils/permissions/permissions.ts:1158-1319
type WaterfallPolicy struct {
	Mode       PermissionMode
	DenyRules  []PermissionRuleValue // always-deny rules
	AllowRules []PermissionRuleValue // always-allow rules
	AskRules   []PermissionRuleValue // always-ask rules
	// IsBypassAvailable is true when the user originally started with bypass mode
	// (for plan mode fallback)
	IsBypassAvailable bool
}

// NewWaterfallPolicy creates a waterfall policy with the given mode and rules.
func NewWaterfallPolicy(mode PermissionMode, denyRules, allowRules, askRules []PermissionRuleValue, isBypassAvailable bool) *WaterfallPolicy {
	return &WaterfallPolicy{
		Mode:              mode,
		DenyRules:         denyRules,
		AllowRules:        allowRules,
		AskRules:          askRules,
		IsBypassAvailable: isBypassAvailable,
	}
}

// Check evaluates the permission waterfall for a tool call.
// The toolID parameter carries the tool input for content-specific rule matching.
// Source: utils/permissions/permissions.ts:1158-1319
func (w *WaterfallPolicy) Check(_ context.Context, toolName string, toolInput string) PermissionDecision {
	// Step 1: Check deny rules
	// Source: permissions.ts:1170-1181
	for _, rule := range w.DenyRules {
		if RuleMatchesToolCall(rule, toolName, toolInput) {
			return DenyDecision{
				Reason: fmt.Sprintf("Permission to use %s has been denied.", toolName),
			}
		}
	}

	// Step 1b: Check ask rules
	// Source: permissions.ts:1184-1206
	for _, rule := range w.AskRules {
		if RuleMatchesToolCall(rule, toolName, toolInput) {
			return AskDecision{
				Message: fmt.Sprintf("Allow %s?", toolName),
			}
		}
	}

	// Steps 1c-1g: Tool-specific checks would go here
	// (requires per-tool CheckPermissions interface — deferred)

	// Step 2a: Check bypass mode
	// Source: permissions.ts:1268-1281
	shouldBypass := w.Mode == ModeBypassPermissions ||
		(w.Mode == ModePlan && w.IsBypassAvailable)
	if shouldBypass {
		return AllowDecision{}
	}

	// Step 2b: Check allow rules
	// Source: permissions.ts:1284-1297
	for _, rule := range w.AllowRules {
		if RuleMatchesToolCall(rule, toolName, toolInput) {
			return AllowDecision{}
		}
	}

	// Step 3: Fallback — mode-based decision
	// Source: permissions.ts:1300-1310
	switch w.Mode {
	case ModeDontAsk:
		return AllowDecision{}
	case ModeAcceptEdits:
		// File tools auto-approved, bash prompts
		if isFileOrReadOnlyTool(toolName) {
			return AllowDecision{}
		}
		return AskDecision{Message: fmt.Sprintf("Allow %s?", toolName)}
	case ModePlan:
		// Read-only tools auto-approved, mutating prompts
		if isReadOnlyToolName(toolName) {
			return AllowDecision{}
		}
		return AskDecision{Message: fmt.Sprintf("Plan mode: allow %s?", toolName)}
	default:
		// ModeDefault, ModeAuto — prompt
		return AskDecision{Message: fmt.Sprintf("Allow %s?", toolName)}
	}
}

// isFileOrReadOnlyTool checks if a tool is a file operation or read-only.
func isFileOrReadOnlyTool(name string) bool {
	switch name {
	case "Edit", "Write", "Read", "Glob", "Grep", "LS", "WebFetch", "WebSearch",
		"NotebookEdit", "ToolSearch", "AskUserQuestion", "EnterPlanMode", "ExitPlanMode",
		"TaskCreate", "TaskList", "TaskGet", "TaskUpdate", "TaskStop", "TaskOutput",
		"Sleep", "LSP", "SendMessage", "Brief", "SyntheticOutput", "Skill",
		"CronCreate", "CronDelete", "CronList", "ListMcpResources", "ReadMcpResource",
		"TodoWrite", "TodoRead", "Config", "Agent":
		return true
	}
	return false
}

// isReadOnlyToolName checks if a tool name is read-only (for plan mode).
func isReadOnlyToolName(name string) bool {
	switch name {
	case "Read", "Glob", "Grep", "LS", "WebFetch", "WebSearch", "ToolSearch",
		"AskUserQuestion", "EnterPlanMode", "ExitPlanMode",
		"TaskCreate", "TaskList", "TaskGet", "TaskUpdate", "TaskStop", "TaskOutput",
		"Sleep", "LSP", "ListMcpResources", "ReadMcpResource",
		"TodoWrite", "TodoRead", "Config", "Brief", "SyntheticOutput", "Skill",
		"CronCreate", "CronDelete", "CronList", "SendMessage", "Agent":
		return true
	}
	return false
}
