package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

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

	t.Run("both_read_only", func(t *testing.T) {
		if !enterTool.IsReadOnly() {
			t.Error("EnterPlanMode should be read-only")
		}
		if !exitTool.IsReadOnly() {
			t.Error("ExitPlanMode should be read-only")
		}
	})

	t.Run("enter_schema_valid", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(enterTool.InputSchema(), &parsed); err != nil {
			t.Fatalf("EnterPlanMode schema is not valid JSON: %v", err)
		}
	})

	t.Run("exit_schema_valid", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(exitTool.InputSchema(), &parsed); err != nil {
			t.Fatalf("ExitPlanMode schema is not valid JSON: %v", err)
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
		if !strings.Contains(out.Content, "Entered plan mode") {
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

		// Double enter is fine
		enterTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		enterTool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if !state.InPlanMode {
			t.Error("expected plan mode after double enter")
		}
	})

	t.Run("returns_two_tools", func(t *testing.T) {
		if len(planTools) != 2 {
			t.Errorf("expected 2 tools, got %d", len(planTools))
		}
	})
}

func TestPlanModeToolsIndependentState(t *testing.T) {
	state1, tools1 := tools.NewPlanModeTools()
	state2, _ := tools.NewPlanModeTools()

	// Enter plan mode on first set
	tools1[0].Execute(context.Background(), nil, json.RawMessage(`{}`))
	if !state1.InPlanMode {
		t.Error("state1 should be in plan mode")
	}
	if state2.InPlanMode {
		t.Error("state2 should not be affected by state1")
	}
}
