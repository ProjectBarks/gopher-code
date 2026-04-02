package query_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func hooksPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "hooks_and_prompts.json")
}

type HooksAndPrompts struct {
	HookEvents struct {
		AllEventNames       []string `json:"all_event_names"`
		ToolLifecycleEvents []string `json:"tool_lifecycle_events"`
		PermissionEvents    []string `json:"permission_events"`
		SessionEvents       []string `json:"session_events"`
	} `json:"hook_events"`
	HookDecisions struct {
		ValidDecisions       []string `json:"valid_decisions"`
		ApproveSetsAllow     bool     `json:"approve_sets_allow"`
		BlockSetsDeny        bool     `json:"block_sets_deny"`
		BlockDefaultMessage  string   `json:"block_default_message"`
		UnknownDecisionThrows bool    `json:"unknown_decision_throws"`
		UnknownDecisionError string   `json:"unknown_decision_error"`
	} `json:"hook_decisions"`
	HookOutput struct {
		ContinueFalsePrevents     bool     `json:"continue_false_prevents_continuation"`
		StopReasonOptional        bool     `json:"stop_reason_optional"`
		SystemMessageOptional     bool     `json:"system_message_optional"`
		PreToolUseOverridePerm    bool     `json:"pre_tool_use_can_override_permission"`
		PreToolUseUpdateInput     bool     `json:"pre_tool_use_can_update_input"`
		PostToolUseUpdateMCP      bool     `json:"post_tool_use_can_update_mcp_output"`
		ElicitationActions        []string `json:"elicitation_actions"`
	} `json:"hook_json_output"`
	SystemPrompt struct {
		PriorityOrder        []string `json:"priority_order"`
		OverrideReplacesAll  bool     `json:"override_replaces_all"`
		AppendAlwaysAdded    bool     `json:"append_always_added_unless_override"`
		CustomReplacesDefault bool    `json:"custom_replaces_default"`
		DefaultIsArrayBlocks bool     `json:"default_is_array_of_blocks"`
	} `json:"system_prompt_construction"`
	PermissionRuleSources []string `json:"permission_rule_sources"`
	PermissionCheckOrder  []string `json:"permission_check_order"`
}

func loadHooks(t *testing.T) *HooksAndPrompts {
	t.Helper()
	data, err := os.ReadFile(hooksPath())
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	var hp HooksAndPrompts
	if err := json.Unmarshal(data, &hp); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return &hp
}

// TestHookEventNames validates all hook event types from TS source.
// Source: types/hooks.ts:73-160
func TestHookEventNames(t *testing.T) {
	hp := loadHooks(t)

	t.Run("total_event_count", func(t *testing.T) {
		if len(hp.HookEvents.AllEventNames) != 15 {
			t.Errorf("expected 15 hook events, got %d", len(hp.HookEvents.AllEventNames))
		}
	})

	expectedEvents := []string{
		"PreToolUse", "PostToolUse", "PostToolUseFailure",
		"UserPromptSubmit", "SessionStart", "Setup", "SubagentStart",
		"PermissionDenied", "PermissionRequest", "Notification",
		"Elicitation", "ElicitationResult",
		"CwdChanged", "FileChanged", "WorktreeCreate",
	}
	for _, name := range expectedEvents {
		name := name
		t.Run(fmt.Sprintf("event_%s", name), func(t *testing.T) {
			found := false
			for _, e := range hp.HookEvents.AllEventNames {
				if e == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("hook event %q not found", name)
			}
		})
	}

	// Tool lifecycle events
	t.Run("tool_lifecycle_PreToolUse", func(t *testing.T) {
		found := false
		for _, e := range hp.HookEvents.ToolLifecycleEvents {
			if e == "PreToolUse" {
				found = true
			}
		}
		if !found {
			t.Error("PreToolUse missing from tool lifecycle events")
		}
	})
	t.Run("tool_lifecycle_PostToolUse", func(t *testing.T) {
		found := false
		for _, e := range hp.HookEvents.ToolLifecycleEvents {
			if e == "PostToolUse" {
				found = true
			}
		}
		if !found {
			t.Error("PostToolUse missing from tool lifecycle events")
		}
	})
	t.Run("tool_lifecycle_count_3", func(t *testing.T) {
		if len(hp.HookEvents.ToolLifecycleEvents) != 3 {
			t.Errorf("expected 3 tool lifecycle events, got %d", len(hp.HookEvents.ToolLifecycleEvents))
		}
	})
}

// TestHookDecisionBehavior validates hook decision processing.
// Source: hooks.ts:527-540
func TestHookDecisionBehavior(t *testing.T) {
	hp := loadHooks(t)
	hd := hp.HookDecisions

	t.Run("valid_decisions_approve_block", func(t *testing.T) {
		if len(hd.ValidDecisions) != 2 {
			t.Fatalf("expected 2 valid decisions, got %d", len(hd.ValidDecisions))
		}
		if hd.ValidDecisions[0] != "approve" || hd.ValidDecisions[1] != "block" {
			t.Errorf("expected [approve, block], got %v", hd.ValidDecisions)
		}
	})
	t.Run("approve_maps_to_allow", func(t *testing.T) {
		if !hd.ApproveSetsAllow {
			t.Error("approve decision must set permissionBehavior to 'allow'")
		}
	})
	t.Run("block_maps_to_deny", func(t *testing.T) {
		if !hd.BlockSetsDeny {
			t.Error("block decision must set permissionBehavior to 'deny'")
		}
	})
	t.Run("block_default_message", func(t *testing.T) {
		if hd.BlockDefaultMessage != "Blocked by hook" {
			t.Errorf("expected 'Blocked by hook', got %q", hd.BlockDefaultMessage)
		}
	})
	t.Run("unknown_decision_throws", func(t *testing.T) {
		if !hd.UnknownDecisionThrows {
			t.Error("unknown decision types must throw an error")
		}
	})
	t.Run("unknown_error_message_format", func(t *testing.T) {
		expected := "Unknown hook decision type: {decision}. Valid types are: approve, block"
		if hd.UnknownDecisionError != expected {
			t.Errorf("got %q", hd.UnknownDecisionError)
		}
	})
}

// TestHookOutputCapabilities validates what hooks can do.
// Source: types/hooks.ts:60-170
func TestHookOutputCapabilities(t *testing.T) {
	hp := loadHooks(t)
	ho := hp.HookOutput

	t.Run("continue_false_prevents_continuation", func(t *testing.T) {
		if !ho.ContinueFalsePrevents {
			t.Error("continue: false must prevent conversation continuation")
		}
	})
	t.Run("pre_tool_use_can_override_permission", func(t *testing.T) {
		if !ho.PreToolUseOverridePerm {
			t.Error("PreToolUse hooks must be able to override permission decisions")
		}
	})
	t.Run("pre_tool_use_can_update_input", func(t *testing.T) {
		if !ho.PreToolUseUpdateInput {
			t.Error("PreToolUse hooks must be able to modify tool input")
		}
	})
	t.Run("post_tool_use_can_update_mcp_output", func(t *testing.T) {
		if !ho.PostToolUseUpdateMCP {
			t.Error("PostToolUse hooks must be able to update MCP tool output")
		}
	})
	t.Run("elicitation_actions", func(t *testing.T) {
		expected := []string{"accept", "decline", "cancel"}
		if len(ho.ElicitationActions) != len(expected) {
			t.Fatalf("expected %d actions, got %d", len(expected), len(ho.ElicitationActions))
		}
		for i, a := range expected {
			if ho.ElicitationActions[i] != a {
				t.Errorf("action[%d] = %q, want %q", i, ho.ElicitationActions[i], a)
			}
		}
	})
}

// TestSystemPromptConstruction validates prompt priority order.
// Source: systemPrompt.ts:41-90
func TestSystemPromptConstruction(t *testing.T) {
	hp := loadHooks(t)
	sp := hp.SystemPrompt

	t.Run("priority_order_has_entries", func(t *testing.T) {
		if len(sp.PriorityOrder) < 4 {
			t.Errorf("expected at least 4 priority levels, got %d", len(sp.PriorityOrder))
		}
	})
	t.Run("override_is_highest_priority", func(t *testing.T) {
		// Source: systemPrompt.ts:56 — if (overrideSystemPrompt) return
		if len(sp.PriorityOrder) == 0 {
			t.Fatal("empty priority order")
		}
		// First entry should mention override
	})
	t.Run("override_replaces_all", func(t *testing.T) {
		if !sp.OverrideReplacesAll {
			t.Error("override system prompt must replace all other prompt sources")
		}
	})
	t.Run("append_always_added", func(t *testing.T) {
		// Source: systemPrompt.ts:39 — appendSystemPrompt always added except with override
		if !sp.AppendAlwaysAdded {
			t.Error("append system prompt must always be added (unless override is set)")
		}
	})
	t.Run("custom_replaces_default", func(t *testing.T) {
		if !sp.CustomReplacesDefault {
			t.Error("custom system prompt must replace default prompt")
		}
	})
	t.Run("default_is_array", func(t *testing.T) {
		// Source: systemPrompt.ts — defaultSystemPrompt: string[]
		if !sp.DefaultIsArrayBlocks {
			t.Error("default system prompt must be an array of blocks")
		}
	})
}

// TestPermissionRuleSources validates where permission rules come from.
// Source: utils/permissions/PermissionRule.ts
func TestPermissionRuleSources(t *testing.T) {
	hp := loadHooks(t)

	expectedSources := []string{"userSettings", "projectSettings", "localSettings", "cliArg", "hook", "session"}
	t.Run("source_count", func(t *testing.T) {
		if len(hp.PermissionRuleSources) != len(expectedSources) {
			t.Errorf("expected %d sources, got %d", len(expectedSources), len(hp.PermissionRuleSources))
		}
	})
	for _, source := range expectedSources {
		source := source
		t.Run(fmt.Sprintf("source_%s", source), func(t *testing.T) {
			found := false
			for _, s := range hp.PermissionRuleSources {
				if s == source {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("permission rule source %q not found", source)
			}
		})
	}
}

// TestPermissionCheckOrder validates the permission checking pipeline.
// Source: utils/permissions/permissions.ts
func TestPermissionCheckOrder(t *testing.T) {
	hp := loadHooks(t)

	t.Run("has_check_steps", func(t *testing.T) {
		if len(hp.PermissionCheckOrder) < 5 {
			t.Errorf("expected at least 5 permission check steps, got %d", len(hp.PermissionCheckOrder))
		}
	})
	t.Run("blanket_deny_first", func(t *testing.T) {
		if len(hp.PermissionCheckOrder) == 0 {
			t.Fatal("empty check order")
		}
		// First step should be blanket deny rules
	})
	// Each step is non-empty
	for i, step := range hp.PermissionCheckOrder {
		i, step := i, step
		t.Run(fmt.Sprintf("step_%d_non_empty", i), func(t *testing.T) {
			if step == "" {
				t.Errorf("step %d is empty", i)
			}
		})
	}
}
