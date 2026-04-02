package permissions

import "strings"

// Source: utils/permissions/dangerousPatterns.ts, utils/permissions/permissionSetup.ts:94-147

// DangerousBashPatterns lists command prefixes that allow arbitrary code execution.
// An allow rule matching these bypasses the auto-mode classifier.
// Source: utils/permissions/dangerousPatterns.ts:18-80
var DangerousBashPatterns = []string{
	// Cross-platform interpreters
	// Source: dangerousPatterns.ts:18-42
	"python", "python3", "python2",
	"node", "deno", "tsx",
	"ruby", "perl", "php", "lua",
	// Package runners
	"npx", "bunx",
	"npm run", "yarn run", "pnpm run", "bun run",
	// Shells
	"bash", "sh", "zsh", "fish",
	// SSH
	"ssh",
	// Dangerous builtins
	// Source: dangerousPatterns.ts:46-50
	"eval", "exec", "env", "xargs", "sudo",
}

// IsDangerousBashPermission checks if a Bash allow rule would bypass the classifier.
// Source: utils/permissions/permissionSetup.ts:94-147
func IsDangerousBashPermission(toolName string, ruleContent string) bool {
	// Only check Bash rules
	if toolName != "Bash" {
		return false
	}

	// Tool-level allow (no content, or empty) — allows ALL commands
	// Source: permissionSetup.ts:104-106
	if ruleContent == "" {
		return true
	}

	content := strings.TrimSpace(strings.ToLower(ruleContent))

	// Standalone wildcard
	// Source: permissionSetup.ts:111-113
	if content == "*" {
		return true
	}

	// Check each dangerous pattern
	// Source: permissionSetup.ts:117-144
	for _, pattern := range DangerousBashPatterns {
		lp := strings.ToLower(pattern)

		// Exact match
		if content == lp {
			return true
		}
		// Prefix syntax: "python:*"
		if content == lp+":*" {
			return true
		}
		// Wildcard at end: "python*"
		if content == lp+"*" {
			return true
		}
		// Wildcard with space: "python *"
		if content == lp+" *" {
			return true
		}
		// Pattern like "python -*" (matches "python -c 'code'")
		if strings.HasPrefix(content, lp+" -") && strings.HasSuffix(content, "*") {
			return true
		}
	}

	return false
}

// StripDangerousPermissions removes dangerous Bash allow rules from a rule set.
// Returns the stripped rules for later restoration.
// Source: utils/permissions/permissionSetup.ts:510-553
func StripDangerousPermissions(allowRules []PermissionRuleValue) (safe []PermissionRuleValue, stripped []PermissionRuleValue) {
	for _, rule := range allowRules {
		if IsDangerousBashPermission(rule.ToolName, rule.RuleContent) {
			stripped = append(stripped, rule)
		} else {
			safe = append(safe, rule)
		}
	}
	return safe, stripped
}

// RestoreDangerousPermissions adds back previously stripped rules.
// Source: utils/permissions/permissionSetup.ts:561-578
func RestoreDangerousPermissions(current []PermissionRuleValue, stashed []PermissionRuleValue) []PermissionRuleValue {
	if len(stashed) == 0 {
		return current
	}
	return append(current, stashed...)
}
