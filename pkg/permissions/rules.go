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

func (p *RuleBasedPolicy) Check(_ context.Context, toolName string, _ string) PermissionDecision {
	switch p.Mode {
	case AutoApprove:
		return AllowDecision{}
	case Deny:
		return DenyDecision{Reason: "permission denied by policy"}
	case Interactive:
		return AskDecision{Message: fmt.Sprintf("approve tool %s?", toolName)}
	default:
		return AllowDecision{}
	}
}
