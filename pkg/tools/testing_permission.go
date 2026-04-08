package tools

import (
	"context"
	"encoding/json"
)

// Source: tools/testing/TestingPermissionTool.tsx
// TestingPermissionTool is a test-only tool that triggers a permission check.
// It's only registered when running in test mode (NODE_ENV=test in TS).

// TestingPermissionTool is used by integration tests to verify permission flows.
type TestingPermissionTool struct{}

func (t *TestingPermissionTool) Name() string        { return "TestingPermission" }
func (t *TestingPermissionTool) Description() string  { return "Test tool for permission validation" }
func (t *TestingPermissionTool) IsReadOnly() bool     { return false }
func (t *TestingPermissionTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"action":{"type":"string"}},"required":["action"]}`)
}

func (t *TestingPermissionTool) Execute(_ context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput("invalid input: " + err.Error()), nil
	}
	return SuccessOutput("TestingPermission: " + params.Action), nil
}

// CheckToolPermissions returns "ask" to trigger the permission flow.
func (t *TestingPermissionTool) CheckToolPermissions(_ *ToolContext, _ json.RawMessage) string {
	return "ask"
}
