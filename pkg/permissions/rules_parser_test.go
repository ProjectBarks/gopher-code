package permissions

import "testing"

// Source: utils/permissions/permissionRuleParser.ts

func TestParsePermissionRuleValue(t *testing.T) {
	// Source: permissionRuleParser.ts:93-133

	t.Run("tool_name_only", func(t *testing.T) {
		// Source: permissionRuleParser.ts:89
		rv := ParsePermissionRuleValue("Bash")
		if rv.ToolName != "Bash" {
			t.Errorf("expected Bash, got %s", rv.ToolName)
		}
		if rv.RuleContent != "" {
			t.Errorf("expected empty content, got %q", rv.RuleContent)
		}
	})

	t.Run("tool_with_content", func(t *testing.T) {
		// Source: permissionRuleParser.ts:90
		rv := ParsePermissionRuleValue("Bash(npm install)")
		if rv.ToolName != "Bash" {
			t.Errorf("expected Bash, got %s", rv.ToolName)
		}
		if rv.RuleContent != "npm install" {
			t.Errorf("expected 'npm install', got %q", rv.RuleContent)
		}
	})

	t.Run("escaped_parentheses", func(t *testing.T) {
		// Source: permissionRuleParser.ts:91
		rv := ParsePermissionRuleValue(`Bash(python -c "print\(1\)")`)
		if rv.ToolName != "Bash" {
			t.Errorf("expected Bash, got %s", rv.ToolName)
		}
		if rv.RuleContent != `python -c "print(1)"` {
			t.Errorf("expected unescaped content, got %q", rv.RuleContent)
		}
	})

	t.Run("empty_content_is_tool_only", func(t *testing.T) {
		// Source: permissionRuleParser.ts:126
		rv := ParsePermissionRuleValue("Bash()")
		if rv.ToolName != "Bash" {
			t.Errorf("expected Bash, got %s", rv.ToolName)
		}
		if rv.RuleContent != "" {
			t.Errorf("expected empty content, got %q", rv.RuleContent)
		}
	})

	t.Run("wildcard_content_is_tool_only", func(t *testing.T) {
		// Source: permissionRuleParser.ts:126
		rv := ParsePermissionRuleValue("Bash(*)")
		if rv.ToolName != "Bash" {
			t.Errorf("expected Bash, got %s", rv.ToolName)
		}
		if rv.RuleContent != "" {
			t.Errorf("expected empty content for wildcard, got %q", rv.RuleContent)
		}
	})

	t.Run("legacy_tool_name", func(t *testing.T) {
		// Source: permissionRuleParser.ts:21-24
		rv := ParsePermissionRuleValue("Task")
		if rv.ToolName != "Agent" {
			t.Errorf("expected Agent (legacy alias), got %s", rv.ToolName)
		}
	})

	t.Run("legacy_tool_name_with_content", func(t *testing.T) {
		rv := ParsePermissionRuleValue("Task(do something)")
		if rv.ToolName != "Agent" {
			t.Errorf("expected Agent, got %s", rv.ToolName)
		}
		if rv.RuleContent != "do something" {
			t.Errorf("expected 'do something', got %q", rv.RuleContent)
		}
	})

	t.Run("malformed_no_close_paren", func(t *testing.T) {
		rv := ParsePermissionRuleValue("Bash(npm")
		if rv.ToolName != "Bash(npm" {
			t.Errorf("expected entire string as tool name, got %s", rv.ToolName)
		}
	})
}

func TestPermissionRuleValueToString(t *testing.T) {
	// Source: permissionRuleParser.ts:144-152

	t.Run("tool_only", func(t *testing.T) {
		s := PermissionRuleValueToString(PermissionRuleValue{ToolName: "Bash"})
		if s != "Bash" {
			t.Errorf("expected Bash, got %s", s)
		}
	})

	t.Run("with_content", func(t *testing.T) {
		s := PermissionRuleValueToString(PermissionRuleValue{ToolName: "Bash", RuleContent: "npm install"})
		if s != "Bash(npm install)" {
			t.Errorf("expected 'Bash(npm install)', got %s", s)
		}
	})

	t.Run("escapes_parens", func(t *testing.T) {
		s := PermissionRuleValueToString(PermissionRuleValue{ToolName: "Bash", RuleContent: `print(1)`})
		if s != `Bash(print\(1\))` {
			t.Errorf("expected escaped parens, got %s", s)
		}
	})
}

func TestRuleMatchesToolCall(t *testing.T) {
	tests := []struct {
		name      string
		rule      PermissionRuleValue
		toolName  string
		toolInput string
		matches   bool
	}{
		{"exact_tool_match", PermissionRuleValue{ToolName: "Bash"}, "Bash", "", true},
		{"exact_tool_no_match", PermissionRuleValue{ToolName: "Bash"}, "Read", "", false},
		{"tool_with_content_match", PermissionRuleValue{ToolName: "Bash", RuleContent: "npm install"}, "Bash", "npm install", true},
		{"tool_with_content_prefix", PermissionRuleValue{ToolName: "Bash", RuleContent: "npm"}, "Bash", "npm install", true},
		{"tool_with_content_no_match", PermissionRuleValue{ToolName: "Bash", RuleContent: "npm install"}, "Bash", "yarn add", false},
		{"content_rule_no_input", PermissionRuleValue{ToolName: "Bash", RuleContent: "npm"}, "Bash", "", false},
		{"glob_tool_name", PermissionRuleValue{ToolName: "mcp__*"}, "mcp__my_server__tool", "", true},
		{"glob_content", PermissionRuleValue{ToolName: "Bash", RuleContent: "git *"}, "Bash", "git push", true},
		{"legacy_tool_name", PermissionRuleValue{ToolName: "Agent"}, "Task", "", true}, // Task normalizes to Agent
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RuleMatchesToolCall(tt.rule, tt.toolName, tt.toolInput)
			if got != tt.matches {
				t.Errorf("RuleMatchesToolCall(%v, %q, %q) = %v, want %v",
					tt.rule, tt.toolName, tt.toolInput, got, tt.matches)
			}
		})
	}
}

func TestGetLegacyToolNames(t *testing.T) {
	// Source: permissionRuleParser.ts:35-41

	t.Run("agent_has_legacy_name", func(t *testing.T) {
		names := GetLegacyToolNames("Agent")
		if len(names) != 1 || names[0] != "Task" {
			t.Errorf("Agent legacy names = %v, want [Task]", names)
		}
	})

	t.Run("task_output_has_two_legacy_names", func(t *testing.T) {
		names := GetLegacyToolNames("TaskOutput")
		if len(names) != 2 {
			t.Errorf("TaskOutput legacy names = %v, want 2 entries", names)
		}
		found := map[string]bool{}
		for _, n := range names {
			found[n] = true
		}
		if !found["AgentOutputTool"] || !found["BashOutputTool"] {
			t.Errorf("TaskOutput legacy names = %v, want AgentOutputTool+BashOutputTool", names)
		}
	})

	t.Run("no_legacy_names", func(t *testing.T) {
		names := GetLegacyToolNames("Bash")
		if len(names) != 0 {
			t.Errorf("Bash should have no legacy names, got %v", names)
		}
	})
}

func TestNormalizeLegacyToolName(t *testing.T) {
	// Source: permissionRuleParser.ts:21-29, 31-33
	tests := []struct{ input, expected string }{
		{"Task", "Agent"},
		{"KillShell", "TaskStop"},
		{"AgentOutputTool", "TaskOutput"},
		{"BashOutputTool", "TaskOutput"},
		{"Bash", "Bash"},     // No alias
		{"Read", "Read"},     // No alias
		{"Unknown", "Unknown"}, // Passthrough
	}
	for _, tt := range tests {
		got := NormalizeLegacyToolName(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeLegacyToolName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
