package permissions

import (
	"path/filepath"
	"strings"
)

// Source: utils/permissions/permissionRuleParser.ts

// PermissionRuleValue represents a parsed permission rule.
// Source: types/permissions.ts:67-70
type PermissionRuleValue struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// PermissionRule is a rule with its source and behavior.
// Source: types/permissions.ts:75-79
type PermissionRule struct {
	Source       string              `json:"source"`       // userSettings, projectSettings, localSettings, etc.
	RuleBehavior string             `json:"ruleBehavior"` // allow, deny, ask
	RuleValue    PermissionRuleValue `json:"ruleValue"`
}

// legacyToolNameAliases maps old tool names to canonical names.
// Source: permissionRuleParser.ts:21-29
var legacyToolNameAliases = map[string]string{
	"Task":            "Agent",
	"KillShell":       "TaskStop",
	"AgentOutputTool": "TaskOutput",
	"BashOutputTool":  "TaskOutput",
}

// NormalizeLegacyToolName maps legacy tool names to canonical names.
// Source: permissionRuleParser.ts:31-33
func NormalizeLegacyToolName(name string) string {
	if canonical, ok := legacyToolNameAliases[name]; ok {
		return canonical
	}
	return name
}

// ParsePermissionRuleValue parses a rule string like "Bash(npm install)" into components.
// Source: permissionRuleParser.ts:93-133
func ParsePermissionRuleValue(ruleString string) PermissionRuleValue {
	openIdx := findFirstUnescapedChar(ruleString, '(')
	if openIdx == -1 {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(ruleString)}
	}

	closeIdx := findLastUnescapedChar(ruleString, ')')
	if closeIdx == -1 || closeIdx <= openIdx {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(ruleString)}
	}

	if closeIdx != len(ruleString)-1 {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(ruleString)}
	}

	toolName := ruleString[:openIdx]
	rawContent := ruleString[openIdx+1 : closeIdx]

	if toolName == "" {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(ruleString)}
	}

	// Empty content or standalone wildcard = tool-wide rule
	// Source: permissionRuleParser.ts:126-128
	if rawContent == "" || rawContent == "*" {
		return PermissionRuleValue{ToolName: NormalizeLegacyToolName(toolName)}
	}

	return PermissionRuleValue{
		ToolName:    NormalizeLegacyToolName(toolName),
		RuleContent: unescapeRuleContent(rawContent),
	}
}

// PermissionRuleValueToString converts a rule value to its string form.
// Source: permissionRuleParser.ts:144-152
func PermissionRuleValueToString(rv PermissionRuleValue) string {
	if rv.RuleContent == "" {
		return rv.ToolName
	}
	return rv.ToolName + "(" + escapeRuleContent(rv.RuleContent) + ")"
}

// RuleMatchesToolCall checks if a permission rule matches a tool name and optional input.
// Supports exact match and glob patterns.
// Source: utils/permissions/shellRuleMatching.ts:159-184
func RuleMatchesToolCall(rule PermissionRuleValue, toolName string, toolInput string) bool {
	// Check tool name match (with legacy normalization)
	canonical := NormalizeLegacyToolName(toolName)
	if rule.ToolName != canonical && rule.ToolName != toolName {
		// Try glob match on tool name
		if matched, _ := filepath.Match(rule.ToolName, canonical); !matched {
			if matched2, _ := filepath.Match(rule.ToolName, toolName); !matched2 {
				return false
			}
		}
	}

	// If no rule content, it matches any call to this tool
	if rule.RuleContent == "" {
		return true
	}

	// If there's rule content, check against the tool input
	if toolInput == "" {
		return false
	}

	// Check for wildcard/glob pattern in rule content
	if strings.Contains(rule.RuleContent, "*") || strings.Contains(rule.RuleContent, "?") {
		if matched, _ := filepath.Match(rule.RuleContent, toolInput); matched {
			return true
		}
	}

	// Check prefix match (legacy :* syntax becomes prefix)
	if strings.HasSuffix(rule.RuleContent, ":*") {
		prefix := rule.RuleContent[:len(rule.RuleContent)-2]
		return strings.HasPrefix(toolInput, prefix)
	}

	// Exact match
	return rule.RuleContent == toolInput || strings.HasPrefix(toolInput, rule.RuleContent)
}

// escapeRuleContent escapes special characters for storage.
// Source: permissionRuleParser.ts:55-60
func escapeRuleContent(content string) string {
	content = strings.ReplaceAll(content, `\`, `\\`)
	content = strings.ReplaceAll(content, `(`, `\(`)
	content = strings.ReplaceAll(content, `)`, `\)`)
	return content
}

// unescapeRuleContent reverses escaping.
// Source: permissionRuleParser.ts:74-79
func unescapeRuleContent(content string) string {
	content = strings.ReplaceAll(content, `\(`, `(`)
	content = strings.ReplaceAll(content, `\)`, `)`)
	content = strings.ReplaceAll(content, `\\`, `\`)
	return content
}

// findFirstUnescapedChar finds the first unescaped occurrence of a character.
// Source: permissionRuleParser.ts:158-175
func findFirstUnescapedChar(s string, ch byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			backslashes := 0
			for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				return i
			}
		}
	}
	return -1
}

// findLastUnescapedChar finds the last unescaped occurrence of a character.
// Source: permissionRuleParser.ts:181-198
func findLastUnescapedChar(s string, ch byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ch {
			backslashes := 0
			for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				return i
			}
		}
	}
	return -1
}
