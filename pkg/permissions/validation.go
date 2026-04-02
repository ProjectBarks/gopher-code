package permissions

import (
	"fmt"
	"strings"
)

// Source: utils/settings/permissionValidation.ts, utils/settings/validation.ts

// ValidationResult is the result of validating a permission rule.
// Source: utils/settings/permissionValidation.ts:58-63
type ValidationResult struct {
	Valid      bool
	Error      string
	Suggestion string
}

// ValidatePermissionRule checks if a permission rule string is well-formed.
// Source: utils/settings/permissionValidation.ts:58-98
func ValidatePermissionRule(rule string) ValidationResult {
	// Empty rule check
	// Source: permissionValidation.ts:65-67
	if strings.TrimSpace(rule) == "" {
		return ValidationResult{Valid: false, Error: "Permission rule cannot be empty"}
	}

	// Check parentheses matching (unescaped only)
	// Source: permissionValidation.ts:70-79
	openCount := countUnescaped(rule, '(')
	closeCount := countUnescaped(rule, ')')
	if openCount != closeCount {
		return ValidationResult{
			Valid:      false,
			Error:      "Mismatched parentheses",
			Suggestion: "Ensure all opening parentheses have matching closing parentheses",
		}
	}

	// Check for empty parentheses
	// Source: permissionValidation.ts:82-97
	if hasEmptyParens(rule) {
		toolName := ""
		if idx := strings.Index(rule, "("); idx > 0 {
			toolName = rule[:idx]
		}
		if toolName == "" {
			return ValidationResult{
				Valid:      false,
				Error:      "Empty parentheses with no tool name",
				Suggestion: "Specify a tool name before the parentheses",
			}
		}
		return ValidationResult{
			Valid:      false,
			Error:      "Empty parentheses",
			Suggestion: fmt.Sprintf("Either specify a pattern or use just %q without parentheses", toolName),
		}
	}

	// Parse the rule — if it parses, it's valid
	parsed := ParsePermissionRuleValue(rule)
	if parsed.ToolName == "" {
		return ValidationResult{Valid: false, Error: "Could not parse tool name"}
	}

	return ValidationResult{Valid: true}
}

// ValidationWarning represents a warning about an invalid rule that was filtered.
// Source: utils/settings/validation.ts:224-265
type ValidationWarning struct {
	File         string
	Path         string
	Message      string
	InvalidValue interface{}
}

// FilterInvalidPermissionRules removes invalid rules from allow/deny/ask arrays.
// Returns warnings for each removed rule. Modifies the input map in-place.
// Source: utils/settings/validation.ts:224-265
func FilterInvalidPermissionRules(rules []string, behavior string, filePath string) ([]string, []ValidationWarning) {
	var valid []string
	var warnings []ValidationWarning

	for _, rule := range rules {
		result := ValidatePermissionRule(rule)
		if !result.Valid {
			msg := fmt.Sprintf("Invalid permission rule %q was skipped", rule)
			if result.Error != "" {
				msg += ": " + result.Error
			}
			if result.Suggestion != "" {
				msg += ". " + result.Suggestion
			}
			warnings = append(warnings, ValidationWarning{
				File:         filePath,
				Path:         "permissions." + behavior,
				Message:      msg,
				InvalidValue: rule,
			})
			continue
		}
		valid = append(valid, rule)
	}

	return valid, warnings
}

// countUnescaped counts unescaped occurrences of a character.
// Source: utils/settings/permissionValidation.ts:30-52
func countUnescaped(s string, ch byte) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			backslashes := 0
			for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				count++
			}
		}
	}
	return count
}

// hasEmptyParens checks for unescaped empty parentheses like "Tool()"
func hasEmptyParens(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '(' && s[i+1] == ')' {
			backslashes := 0
			for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				return true
			}
		}
	}
	return false
}
