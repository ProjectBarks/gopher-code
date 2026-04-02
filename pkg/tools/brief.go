package tools

import (
	"context"
	"encoding/json"
)

// BriefTool sends or receives briefing messages for context sharing between sessions.
type BriefTool struct{}

func (t *BriefTool) Name() string        { return "Brief" }
func (t *BriefTool) Description() string { return "Send or receive a briefing message for context sharing between sessions" }
func (t *BriefTool) IsReadOnly() bool    { return true }

func (t *BriefTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"message": {"type": "string", "description": "Briefing message to share"},
			"action": {"type": "string", "enum": ["send", "receive"], "description": "Send or receive a briefing"}
		},
		"required": ["message", "action"],
		"additionalProperties": false
	}`)
}

func (t *BriefTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Message string `json:"message"`
		Action  string `json:"action"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput("invalid input: " + err.Error()), nil
	}
	if params.Action == "send" {
		return SuccessOutput("Briefing sent: " + params.Message), nil
	}
	return SuccessOutput("No briefings available."), nil
}
