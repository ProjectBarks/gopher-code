package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// SyntheticOutputTool generates synthetic output for internal use.
// It simply returns the given text as-is, used to inject synthetic messages
// into the conversation.
type SyntheticOutputTool struct{}

func (t *SyntheticOutputTool) Name() string        { return "SyntheticOutput" }
func (t *SyntheticOutputTool) Description() string { return "Generate synthetic output for internal use" }
func (t *SyntheticOutputTool) IsReadOnly() bool    { return true }

func (t *SyntheticOutputTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"text": {"type": "string", "description": "Text to return as synthetic output"}
		},
		"required": ["text"],
		"additionalProperties": false
	}`)
}

func (t *SyntheticOutputTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Text == "" {
		return ErrorOutput("text is required"), nil
	}
	return SuccessOutput(params.Text), nil
}
