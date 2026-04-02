package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// AskUserQuestionTool asks the user a question and waits for their response.
// In non-interactive mode, the question is shown but not answered.
type AskUserQuestionTool struct{}

func (t *AskUserQuestionTool) Name() string        { return "AskUserQuestion" }
func (t *AskUserQuestionTool) Description() string { return "Ask the user a question and wait for their response" }
func (t *AskUserQuestionTool) IsReadOnly() bool    { return true }

func (t *AskUserQuestionTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"question": {
				"type": "string",
				"description": "The question to ask the user"
			}
		},
		"required": ["question"],
		"additionalProperties": false
	}`)
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Question string `json:"question"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Question == "" {
		return ErrorOutput("question is required"), nil
	}

	// In the current simple implementation, just return the question as output.
	// The REPL layer would need to intercept this for interactive mode.
	return SuccessOutput(fmt.Sprintf("[Question for user]: %s\n(In non-interactive mode, questions are shown but not answered)", params.Question)), nil
}
