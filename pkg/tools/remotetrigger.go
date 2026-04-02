package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// RemoteTriggerTool triggers a remote agent execution.
// Currently a placeholder that returns a not-configured message.
type RemoteTriggerTool struct{}

func (t *RemoteTriggerTool) Name() string        { return "RemoteTrigger" }
func (t *RemoteTriggerTool) Description() string { return "Trigger a remote agent execution" }
func (t *RemoteTriggerTool) IsReadOnly() bool    { return false }

func (t *RemoteTriggerTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"agent": {"type": "string", "description": "Name or ID of the remote agent to trigger"},
			"prompt": {"type": "string", "description": "Prompt to send to the remote agent"}
		},
		"required": ["agent", "prompt"],
		"additionalProperties": false
	}`)
}

func (t *RemoteTriggerTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Agent  string `json:"agent"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Agent == "" {
		return ErrorOutput("agent is required"), nil
	}
	if params.Prompt == "" {
		return ErrorOutput("prompt is required"), nil
	}
	return ErrorOutput("Remote agents are not configured. Set up AGENT_TRIGGERS_REMOTE to enable remote agent execution."), nil
}
