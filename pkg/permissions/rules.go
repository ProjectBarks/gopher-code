package permissions

import (
	"context"
	"fmt"
)

// RuleBasedPolicy implements PermissionPolicy with simple mode-based rules.
type RuleBasedPolicy struct {
	Mode PermissionMode
}

func NewRuleBasedPolicy(mode PermissionMode) *RuleBasedPolicy {
	return &RuleBasedPolicy{Mode: mode}
}

// Check evaluates permissions based on the configured mode.
// Source: types/permissions.ts:16-29
func (p *RuleBasedPolicy) Check(_ context.Context, toolName string, _ string) PermissionDecision {
	switch p.Mode {
	case ModeBypassPermissions, ModeDontAsk:
		// bypassPermissions/dontAsk: auto-approve everything
		return AllowDecision{}
	case ModeAcceptEdits:
		// acceptEdits: auto-approve file changes (Edit, Write), prompt for bash
		// Source: types/permissions.ts:17
		switch toolName {
		case "Edit", "Write", "Read", "Glob", "Grep", "LS", "WebFetch", "WebSearch",
			"NotebookEdit", "ToolSearch", "AskUserQuestion", "EnterPlanMode", "ExitPlanMode",
			"TaskCreate", "TaskList", "TaskGet", "TaskUpdate", "TaskStop", "TaskOutput",
			"Sleep", "LSP", "SendMessage", "Brief", "SyntheticOutput", "Skill",
			"CronCreate", "CronDelete", "CronList", "ListMcpResources", "ReadMcpResource",
			"TodoWrite", "TodoRead", "Config", "Agent":
			return AllowDecision{}
		default:
			return AskDecision{Message: fmt.Sprintf("approve tool %s?", toolName)}
		}
	case ModePlan:
		// plan: restrict to read-only tools only
		// Source: types/permissions.ts:21
		switch toolName {
		case "Read", "Glob", "Grep", "LS", "WebFetch", "WebSearch", "ToolSearch",
			"AskUserQuestion", "EnterPlanMode", "ExitPlanMode",
			"TaskCreate", "TaskList", "TaskGet", "TaskUpdate", "TaskStop", "TaskOutput",
			"Sleep", "LSP", "ListMcpResources", "ReadMcpResource",
			"TodoWrite", "TodoRead", "Config", "Brief", "SyntheticOutput", "Skill",
			"CronCreate", "CronDelete", "CronList", "SendMessage", "Agent":
			return AllowDecision{}
		default:
			return AskDecision{Message: fmt.Sprintf("plan mode: approve tool %s?", toolName)}
		}
	case Deny:
		return DenyDecision{Reason: "permission denied by policy"}
	case ModeDefault:
		// default: prompt for everything except read-only
		return AskDecision{Message: fmt.Sprintf("approve tool %s?", toolName)}
	case ModeAuto:
		// auto: uses classifier (not implemented yet), fallback to ask
		return AskDecision{Message: fmt.Sprintf("approve tool %s?", toolName)}
	default:
		return AllowDecision{}
	}
}
