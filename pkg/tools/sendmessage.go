package tools

import (
	"context"
	"encoding/json"
)

// SendMessageTool is a placeholder for Claude Code's multi-agent messaging tool.
type SendMessageTool struct{}

func (t *SendMessageTool) Name() string        { return "SendMessage" }
func (t *SendMessageTool) Description() string { return "Send a message to a running agent or teammate" }
func (t *SendMessageTool) IsReadOnly() bool    { return true }

func (t *SendMessageTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"recipient": {"type": "string", "description": "The agent or teammate to send the message to"},
			"message": {"type": "string", "description": "The message content to send"}
		},
		"required": ["recipient", "message"],
		"additionalProperties": false
	}`)
}

func (t *SendMessageTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	return ErrorOutput("SendMessage is not available: this feature requires multi-agent mode, which is not yet supported"), nil
}
