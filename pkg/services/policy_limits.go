package services

// Source: services/policyLimits/index.ts

// PolicyName identifies a policy that can be enabled/disabled.
type PolicyName string

const (
	PolicyAllowRemoteSessions PolicyName = "allow_remote_sessions"
	PolicyAllowAutoMode       PolicyName = "allow_auto_mode"
	PolicyAllowWebSearch      PolicyName = "allow_web_search"
	PolicyAllowWebFetch       PolicyName = "allow_web_fetch"
)

// PolicyChecker checks whether a policy is allowed.
// Source: services/policyLimits/index.ts — isPolicyAllowed
type PolicyChecker struct {
	overrides map[PolicyName]bool
}

// NewPolicyChecker creates a policy checker with default allow-all behavior.
func NewPolicyChecker() *PolicyChecker {
	return &PolicyChecker{overrides: make(map[PolicyName]bool)}
}

// SetPolicy sets whether a specific policy is allowed.
func (c *PolicyChecker) SetPolicy(name PolicyName, allowed bool) {
	c.overrides[name] = allowed
}

// IsAllowed returns true if the policy is allowed (default: true).
func (c *PolicyChecker) IsAllowed(name PolicyName) bool {
	if v, ok := c.overrides[name]; ok {
		return v
	}
	return true // default allow
}
