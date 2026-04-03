package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Source: tools/EnterPlanModeTool/EnterPlanModeTool.ts, tools/ExitPlanModeTool/ExitPlanModeV2Tool.ts

// PlanState tracks plan mode state with full transition support.
// Source: bootstrap/state.ts:1349-1363
type PlanState struct {
	mu                          sync.RWMutex
	InPlanMode                  bool   // currently in plan mode
	PrePlanMode                 string // saved mode to restore on exit (e.g. "default", "acceptEdits")
	HasExitedPlanMode           bool   // has user exited plan at least once in session
	NeedsPlanModeExitAttachment bool   // one-time plan exit notification
	Plan                        string // the plan content (if written to file)
	PlanFilePath                string // path to plan file on disk
}

// EnterPlan transitions into plan mode, saving the previous mode.
// Source: EnterPlanModeTool.ts:83-94
func (s *PlanState) EnterPlan(currentMode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PrePlanMode = currentMode
	s.InPlanMode = true
	s.NeedsPlanModeExitAttachment = false
}

// ExitPlan transitions out of plan mode, returning the mode to restore.
// Source: ExitPlanModeV2Tool.ts:357-403
func (s *PlanState) ExitPlan() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InPlanMode = false
	s.HasExitedPlanMode = true
	s.NeedsPlanModeExitAttachment = true

	restoreMode := s.PrePlanMode
	if restoreMode == "" {
		restoreMode = "default"
	}
	s.PrePlanMode = ""
	return restoreMode
}

// IsInPlanMode returns true if currently in plan mode.
func (s *PlanState) IsInPlanMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.InPlanMode
}

// ExitPlanModeResult is the structured result from ExitPlanMode.
// Source: ExitPlanModeV2Tool.ts output schema
type ExitPlanModeResult struct {
	Plan                    string `json:"plan,omitempty"`
	IsAgent                 bool   `json:"isAgent"`
	FilePath                string `json:"filePath,omitempty"`
	PlanWasEdited           bool   `json:"planWasEdited,omitempty"`
	AwaitingLeaderApproval  bool   `json:"awaitingLeaderApproval,omitempty"`
	RequestID               string `json:"requestId,omitempty"`
}

// EnterPlanModeTool puts the assistant into planning mode.
// Source: tools/EnterPlanModeTool/EnterPlanModeTool.ts
type EnterPlanModeTool struct {
	state       *PlanState
	currentMode func() string // returns current permission mode
}

func (t *EnterPlanModeTool) Name() string        { return "EnterPlanMode" }
func (t *EnterPlanModeTool) Description() string {
	return "Enter planning mode to create a plan before making changes. In plan mode, you can only read files and search — no edits or commands."
}
func (t *EnterPlanModeTool) IsReadOnly() bool { return true }

func (t *EnterPlanModeTool) InputSchema() json.RawMessage {
	// Source: EnterPlanModeTool.ts — empty strict object
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *EnterPlanModeTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	if t.state.IsInPlanMode() {
		return SuccessOutput("Already in plan mode."), nil
	}

	mode := "default"
	if t.currentMode != nil {
		mode = t.currentMode()
	}
	t.state.EnterPlan(mode)
	return SuccessOutput("Entered plan mode. Outline your plan — only read-only tools are available until you exit plan mode."), nil
}

// ExitPlanModeTool exits planning mode with optional plan approval.
// Source: tools/ExitPlanModeTool/ExitPlanModeV2Tool.ts
type ExitPlanModeTool struct {
	state *PlanState
}

func (t *ExitPlanModeTool) Name() string { return "ExitPlanMode" }
func (t *ExitPlanModeTool) Description() string {
	return "Exit plan mode and begin implementing the plan. Returns the plan for approval."
}
func (t *ExitPlanModeTool) IsReadOnly() bool { return false }

func (t *ExitPlanModeTool) InputSchema() json.RawMessage {
	// Source: ExitPlanModeV2Tool.ts — with optional allowedPrompts
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"allowedPrompts": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"tool": {"type": "string", "enum": ["Bash"]},
						"prompt": {"type": "string", "description": "Semantic description of allowed command (e.g., 'run tests', 'install dependencies')"}
					},
					"required": ["tool", "prompt"]
				},
				"description": "Bash commands the plan requires permission to run"
			}
		},
		"additionalProperties": false
	}`)
}

type exitPlanInput struct {
	AllowedPrompts []struct {
		Tool   string `json:"tool"`
		Prompt string `json:"prompt"`
	} `json:"allowedPrompts,omitempty"`
}

func (t *ExitPlanModeTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	if !t.state.IsInPlanMode() {
		return SuccessOutput("Not currently in plan mode."), nil
	}

	var params exitPlanInput
	if input != nil && len(input) > 0 {
		json.Unmarshal(input, &params)
	}

	restoredMode := t.state.ExitPlan()

	result := ExitPlanModeResult{
		Plan:    t.state.Plan,
		IsAgent: false,
	}
	if t.state.PlanFilePath != "" {
		result.FilePath = t.state.PlanFilePath
	}

	// Build response message
	msg := fmt.Sprintf("Exited plan mode. Restored to '%s' mode. Proceeding with implementation.", restoredMode)
	if len(params.AllowedPrompts) > 0 {
		msg += "\n\nRequested permissions:"
		for _, p := range params.AllowedPrompts {
			msg += fmt.Sprintf("\n  - %s: %s", p.Tool, p.Prompt)
		}
	}

	return SuccessOutput(msg), nil
}

// NewPlanModeTools creates the plan mode tool pair sharing the same state.
func NewPlanModeTools() (*PlanState, []Tool) {
	state := &PlanState{}
	return state, []Tool{
		&EnterPlanModeTool{state: state},
		&ExitPlanModeTool{state: state},
	}
}

// NewPlanModeToolsWithMode creates plan mode tools with a mode provider function.
func NewPlanModeToolsWithMode(currentMode func() string) (*PlanState, []Tool) {
	state := &PlanState{}
	return state, []Tool{
		&EnterPlanModeTool{state: state, currentMode: currentMode},
		&ExitPlanModeTool{state: state},
	}
}

// HandlePlanModeTransition processes the side effects of mode transitions.
// Source: bootstrap/state.ts:1349-1363
func HandlePlanModeTransition(state *PlanState, fromMode, toMode string) {
	if toMode == "plan" && fromMode != "plan" {
		state.mu.Lock()
		state.NeedsPlanModeExitAttachment = false
		state.mu.Unlock()
	}
	if fromMode == "plan" && toMode != "plan" {
		state.mu.Lock()
		state.NeedsPlanModeExitAttachment = true
		state.mu.Unlock()
	}
}
