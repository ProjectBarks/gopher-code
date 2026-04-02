package permissions

import (
	"context"
	"testing"
)

// Source: utils/permissions/permissions.ts:1158-1319

func TestWaterfallPolicy(t *testing.T) {

	t.Run("deny_rule_takes_precedence", func(t *testing.T) {
		// Source: permissions.ts:1170-1181
		w := NewWaterfallPolicy(ModeBypassPermissions,
			[]PermissionRuleValue{{ToolName: "Bash", RuleContent: "rm -rf"}}, // deny
			[]PermissionRuleValue{{ToolName: "Bash"}},                        // allow
			nil, false,
		)
		decision := w.Check(context.Background(), "Bash", "rm -rf /")
		if _, ok := decision.(DenyDecision); !ok {
			t.Errorf("deny rule should take precedence even in bypass mode, got %T", decision)
		}
	})

	t.Run("ask_rule_before_bypass", func(t *testing.T) {
		// Source: permissions.ts:1184-1206
		w := NewWaterfallPolicy(ModeBypassPermissions,
			nil,
			nil,
			[]PermissionRuleValue{{ToolName: "Bash", RuleContent: "npm publish"}}, // ask
			false,
		)
		decision := w.Check(context.Background(), "Bash", "npm publish")
		if _, ok := decision.(AskDecision); !ok {
			t.Errorf("ask rule should fire before bypass, got %T", decision)
		}
	})

	t.Run("bypass_mode_allows", func(t *testing.T) {
		// Source: permissions.ts:1268-1281
		w := NewWaterfallPolicy(ModeBypassPermissions, nil, nil, nil, false)
		decision := w.Check(context.Background(), "Bash", "npm install")
		if _, ok := decision.(AllowDecision); !ok {
			t.Errorf("bypass mode should allow, got %T", decision)
		}
	})

	t.Run("plan_mode_with_bypass_available_allows", func(t *testing.T) {
		// Source: permissions.ts:1270-1271
		w := NewWaterfallPolicy(ModePlan, nil, nil, nil, true) // isBypassAvailable
		decision := w.Check(context.Background(), "Bash", "npm install")
		if _, ok := decision.(AllowDecision); !ok {
			t.Errorf("plan mode + bypass available should allow, got %T", decision)
		}
	})

	t.Run("plan_mode_without_bypass_prompts_bash", func(t *testing.T) {
		w := NewWaterfallPolicy(ModePlan, nil, nil, nil, false)
		decision := w.Check(context.Background(), "Bash", "npm install")
		if _, ok := decision.(AskDecision); !ok {
			t.Errorf("plan mode without bypass should prompt for Bash, got %T", decision)
		}
	})

	t.Run("plan_mode_allows_read_only", func(t *testing.T) {
		w := NewWaterfallPolicy(ModePlan, nil, nil, nil, false)
		decision := w.Check(context.Background(), "Read", "/etc/hosts")
		if _, ok := decision.(AllowDecision); !ok {
			t.Errorf("plan mode should allow Read, got %T", decision)
		}
	})

	t.Run("allow_rule_matches", func(t *testing.T) {
		// Source: permissions.ts:1284-1297
		w := NewWaterfallPolicy(ModeDefault,
			nil,
			[]PermissionRuleValue{{ToolName: "Bash", RuleContent: "npm test"}},
			nil, false,
		)
		decision := w.Check(context.Background(), "Bash", "npm test")
		if _, ok := decision.(AllowDecision); !ok {
			t.Errorf("allow rule should match, got %T", decision)
		}
	})

	t.Run("default_mode_prompts", func(t *testing.T) {
		// Source: permissions.ts:1300-1310
		w := NewWaterfallPolicy(ModeDefault, nil, nil, nil, false)
		decision := w.Check(context.Background(), "Bash", "something")
		if _, ok := decision.(AskDecision); !ok {
			t.Errorf("default mode should prompt, got %T", decision)
		}
	})

	t.Run("accept_edits_allows_file_tools", func(t *testing.T) {
		w := NewWaterfallPolicy(ModeAcceptEdits, nil, nil, nil, false)
		for _, tool := range []string{"Edit", "Write", "Read", "Glob", "Grep"} {
			decision := w.Check(context.Background(), tool, "")
			if _, ok := decision.(AllowDecision); !ok {
				t.Errorf("acceptEdits should allow %s, got %T", tool, decision)
			}
		}
	})

	t.Run("accept_edits_prompts_bash", func(t *testing.T) {
		w := NewWaterfallPolicy(ModeAcceptEdits, nil, nil, nil, false)
		decision := w.Check(context.Background(), "Bash", "npm install")
		if _, ok := decision.(AskDecision); !ok {
			t.Errorf("acceptEdits should prompt for Bash, got %T", decision)
		}
	})

	t.Run("dont_ask_allows_all", func(t *testing.T) {
		w := NewWaterfallPolicy(ModeDontAsk, nil, nil, nil, false)
		decision := w.Check(context.Background(), "Bash", "rm -rf /")
		if _, ok := decision.(AllowDecision); !ok {
			t.Errorf("dontAsk should allow everything, got %T", decision)
		}
	})

	t.Run("glob_deny_rule", func(t *testing.T) {
		w := NewWaterfallPolicy(ModeDefault,
			[]PermissionRuleValue{{ToolName: "mcp__*"}}, // deny all MCP tools
			nil, nil, false,
		)
		decision := w.Check(context.Background(), "mcp__server__tool", "")
		if _, ok := decision.(DenyDecision); !ok {
			t.Errorf("glob deny should match MCP tools, got %T", decision)
		}
	})

	t.Run("waterfall_order_deny_before_allow", func(t *testing.T) {
		// Deny rules are checked before allow rules
		w := NewWaterfallPolicy(ModeDefault,
			[]PermissionRuleValue{{ToolName: "Bash"}}, // deny
			[]PermissionRuleValue{{ToolName: "Bash"}}, // allow
			nil, false,
		)
		decision := w.Check(context.Background(), "Bash", "ls")
		if _, ok := decision.(DenyDecision); !ok {
			t.Errorf("deny should win over allow, got %T", decision)
		}
	})
}
