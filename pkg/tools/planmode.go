package tools

import (
	"context"
	"encoding/json"
)

// PlanState tracks whether we're in plan mode.
type PlanState struct {
	InPlanMode bool
	Plan       string
}

// EnterPlanModeTool puts the assistant into planning mode.
type EnterPlanModeTool struct {
	state *PlanState
}

func (t *EnterPlanModeTool) Name() string        { return "EnterPlanMode" }
func (t *EnterPlanModeTool) Description() string { return "Enter planning mode to outline a plan before executing" }
func (t *EnterPlanModeTool) IsReadOnly() bool    { return true }

func (t *EnterPlanModeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *EnterPlanModeTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	t.state.InPlanMode = true
	return SuccessOutput("Entered plan mode. Outline your plan — no tools will be executed until you exit plan mode."), nil
}

// ExitPlanModeTool exits planning mode.
type ExitPlanModeTool struct {
	state *PlanState
}

func (t *ExitPlanModeTool) Name() string        { return "ExitPlanMode" }
func (t *ExitPlanModeTool) Description() string { return "Exit planning mode and begin execution" }
func (t *ExitPlanModeTool) IsReadOnly() bool    { return true }

func (t *ExitPlanModeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *ExitPlanModeTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	t.state.InPlanMode = false
	return SuccessOutput("Exited plan mode. Proceeding with execution."), nil
}

// NewPlanModeTools creates the plan mode tool pair sharing the same state.
func NewPlanModeTools() (*PlanState, []Tool) {
	state := &PlanState{}
	return state, []Tool{
		&EnterPlanModeTool{state: state},
		&ExitPlanModeTool{state: state},
	}
}
