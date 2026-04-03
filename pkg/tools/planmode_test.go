package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// Source: tools/EnterPlanModeTool/EnterPlanModeTool.ts, tools/ExitPlanModeTool/ExitPlanModeV2Tool.ts

func TestPlanModeTools(t *testing.T) {
	state, planTools := tools.NewPlanModeTools()

	var enterTool, exitTool tools.Tool
	for _, pt := range planTools {
		switch pt.Name() {
		case "EnterPlanMode":
			enterTool = pt
		case "ExitPlanMode":
			exitTool = pt
		}
	}
	if enterTool == nil || exitTool == nil {
		t.Fatal("NewPlanModeTools must return both EnterPlanMode and ExitPlanMode")
	}

	t.Run("enter_name", func(t *testing.T) {
		if enterTool.Name() != "EnterPlanMode" {
			t.Errorf("expected 'EnterPlanMode', got %q", enterTool.Name())
		}
	})

	t.Run("exit_name", func(t *testing.T) {
		if exitTool.Name() != "ExitPlanMode" {
			t.Errorf("expected 'ExitPlanMode', got %q", exitTool.Name())
		}
	})

	t.Run("enter_is_read_only", func(t *testing.T) {
		// Source: EnterPlanModeTool.ts — isReadOnly returns true
		if !enterTool.IsReadOnly() {
			t.Error("EnterPlanMode should be read-only")
		}
	})

	t.Run("exit_is_not_read_only", func(t *testing.T) {
		// Source: ExitPlanModeV2Tool.ts — isReadOnly returns false (writes plan file)
		if exitTool.IsReadOnly() {
			t.Error("ExitPlanMode should NOT be read-only (writes plan)")
		}
	})

	t.Run("enter_schema_valid", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(enterTool.InputSchema(), &parsed); err != nil {
			t.Fatalf("EnterPlanMode schema is not valid JSON: %v", err)
		}
	})

	t.Run("exit_schema_has_allowedPrompts", func(t *testing.T) {
		// Source: ExitPlanModeV2Tool.ts — allowedPrompts input field
		var parsed map[string]interface{}
		if err := json.Unmarshal(exitTool.InputSchema(), &parsed); err != nil {
			t.Fatalf("ExitPlanMode schema is not valid JSON: %v", err)
		}
		props := parsed["properties"].(map[string]interface{})
		if _, ok := props["allowedPrompts"]; !ok {
			t.Error("ExitPlanMode schema should have allowedPrompts")
		}
	})

	t.Run("enter_sets_plan_mode", func(t *testing.T) {
		state.InPlanMode = false
		out, err := enterTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !state.InPlanMode {
			t.Error("expected InPlanMode to be true after EnterPlanMode")
		}
		if !strings.Contains(out.Content, "plan mode") {
			t.Errorf("unexpected output: %q", out.Content)
		}
	})

	t.Run("exit_clears_plan_mode", func(t *testing.T) {
		state.InPlanMode = true
		out, err := exitTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if state.InPlanMode {
			t.Error("expected InPlanMode to be false after ExitPlanMode")
		}
		if !strings.Contains(out.Content, "Exited plan mode") {
			t.Errorf("unexpected output: %q", out.Content)
		}
	})

	t.Run("enter_exit_cycle", func(t *testing.T) {
		state.InPlanMode = false

		enterTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if !state.InPlanMode {
			t.Error("expected plan mode after enter")
		}

		exitTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if state.InPlanMode {
			t.Error("expected no plan mode after exit")
		}

		// Enter again — should work
		enterTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if !state.InPlanMode {
			t.Error("expected plan mode after re-enter")
		}
	})

	t.Run("enter_already_in_plan", func(t *testing.T) {
		state.InPlanMode = true
		out, _ := enterTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if out.Content != "Already in plan mode." {
			t.Errorf("output = %q", out.Content)
		}
	})

	t.Run("exit_not_in_plan", func(t *testing.T) {
		state.InPlanMode = false
		out, _ := exitTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if out.Content != "Not currently in plan mode." {
			t.Errorf("output = %q", out.Content)
		}
	})

	t.Run("returns_two_tools", func(t *testing.T) {
		if len(planTools) != 2 {
			t.Errorf("expected 2 tools, got %d", len(planTools))
		}
	})
}

func TestPlanState_PreservesMode(t *testing.T) {
	// Source: EnterPlanModeTool.ts:83-94 — saves current mode as prePlanMode
	state, planTools := tools.NewPlanModeToolsWithMode(func() string { return "acceptEdits" })

	planTools[0].Execute(context.Background(), nil, json.RawMessage(`{}`))
	if state.PrePlanMode != "acceptEdits" {
		t.Errorf("prePlanMode = %q, want 'acceptEdits'", state.PrePlanMode)
	}

	// Source: ExitPlanModeV2Tool.ts:357-403 — restores prePlanMode
	planTools[1].Execute(context.Background(), nil, json.RawMessage(`{}`))
	if state.PrePlanMode != "" {
		t.Errorf("prePlanMode should be cleared after exit, got %q", state.PrePlanMode)
	}
}

func TestPlanState_HasExitedPlanMode(t *testing.T) {
	// Source: ExitPlanModeV2Tool.ts — sets HasExitedPlanMode
	state, planTools := tools.NewPlanModeTools()

	if state.HasExitedPlanMode {
		t.Error("should start false")
	}

	planTools[0].Execute(context.Background(), nil, nil)
	planTools[1].Execute(context.Background(), nil, json.RawMessage(`{}`))

	if !state.HasExitedPlanMode {
		t.Error("should be true after exit")
	}
}

func TestPlanState_NeedsPlanModeExitAttachment(t *testing.T) {
	// Source: bootstrap/state.ts:1349-1363
	state, planTools := tools.NewPlanModeTools()

	planTools[0].Execute(context.Background(), nil, nil) // enter
	if state.NeedsPlanModeExitAttachment {
		t.Error("should be false after enter")
	}

	planTools[1].Execute(context.Background(), nil, json.RawMessage(`{}`)) // exit
	if !state.NeedsPlanModeExitAttachment {
		t.Error("should be true after exit")
	}
}

func TestHandlePlanModeTransition(t *testing.T) {
	// Source: bootstrap/state.ts:1349-1363
	t.Run("entering_plan", func(t *testing.T) {
		state := &tools.PlanState{NeedsPlanModeExitAttachment: true}
		tools.HandlePlanModeTransition(state, "default", "plan")
		if state.NeedsPlanModeExitAttachment {
			t.Error("entering plan should clear exit attachment")
		}
	})

	t.Run("exiting_plan", func(t *testing.T) {
		state := &tools.PlanState{}
		tools.HandlePlanModeTransition(state, "plan", "default")
		if !state.NeedsPlanModeExitAttachment {
			t.Error("exiting plan should set exit attachment")
		}
	})
}

func TestExitPlanMode_AllowedPrompts(t *testing.T) {
	// Source: ExitPlanModeV2Tool.ts — allowedPrompts input
	state, planTools := tools.NewPlanModeTools()
	planTools[0].Execute(context.Background(), nil, nil) // enter

	input := json.RawMessage(`{
		"allowedPrompts": [
			{"tool": "Bash", "prompt": "run tests"},
			{"tool": "Bash", "prompt": "install deps"}
		]
	}`)
	out, err := planTools[1].Execute(context.Background(), nil, input)
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Errorf("unexpected error: %s", out.Content)
	}
	if !strings.Contains(out.Content, "run tests") {
		t.Errorf("should mention allowed prompts: %q", out.Content)
	}
	if state.InPlanMode {
		t.Error("should have exited plan mode")
	}
}

func TestPlanModeToolsIndependentState(t *testing.T) {
	state1, tools1 := tools.NewPlanModeTools()
	state2, _ := tools.NewPlanModeTools()

	tools1[0].Execute(context.Background(), nil, json.RawMessage(`{}`))
	if !state1.InPlanMode {
		t.Error("state1 should be in plan mode")
	}
	if state2.InPlanMode {
		t.Error("state2 should not be affected by state1")
	}
}

func TestExitPlanModeResult_JSON(t *testing.T) {
	// Source: ExitPlanModeV2Tool.ts output schema
	result := tools.ExitPlanModeResult{
		Plan:                   "Step 1: Read\nStep 2: Edit",
		IsAgent:                true,
		FilePath:               "/tmp/plan.md",
		AwaitingLeaderApproval: true,
		RequestID:              "req-abc",
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)
	if parsed["isAgent"] != true {
		t.Error("isAgent should be true")
	}
	if parsed["awaitingLeaderApproval"] != true {
		t.Error("awaitingLeaderApproval should be true")
	}
	if parsed["requestId"] != "req-abc" {
		t.Error("requestId wrong")
	}
}
