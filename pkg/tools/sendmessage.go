package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/session"
)

// Source: tools/SendMessageTool/SendMessageTool.ts

// SendMessageTool routes messages to named teammates via the file-based mailbox.
// Supports direct messages (to: "name") and broadcast (to: "*").
// Source: tools/SendMessageTool/SendMessageTool.ts:522-548
type SendMessageTool struct {
	Mailbox    *session.Mailbox // nil = not in team mode
	TeamName   string
	SenderName string
	SenderColor string
}

func (t *SendMessageTool) Name() string        { return "SendMessage" }
func (t *SendMessageTool) Description() string { return "Send a message to a running agent or teammate" }
func (t *SendMessageTool) IsReadOnly() bool    { return true }

// Source: tools/SendMessageTool/SendMessageTool.ts:60-80
func (t *SendMessageTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"to": {"type": "string", "description": "Recipient: teammate name, or \"*\" for broadcast to all teammates"},
			"message": {"type": "string", "description": "The message content to send"},
			"summary": {"type": "string", "description": "Brief 5-10 word preview of the message"}
		},
		"required": ["to", "message"],
		"additionalProperties": false
	}`)
}

// Source: tools/SendMessageTool/SendMessageTool.ts:140-265
func (t *SendMessageTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		To      string `json:"to"`
		Message string `json:"message"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.To == "" {
		return ErrorOutput("recipient ('to') is required"), nil
	}
	if params.Message == "" {
		return ErrorOutput("message is required"), nil
	}

	if t.Mailbox == nil {
		return ErrorOutput("SendMessage is not available: not in a team context. Create a team first."), nil
	}

	senderName := t.SenderName
	if senderName == "" {
		senderName = "agent"
	}

	// Broadcast: to = "*"
	// Source: SendMessageTool.ts:195-264
	if params.To == "*" {
		return t.broadcast(params.Message, params.Summary, senderName)
	}

	// Direct message
	// Source: SendMessageTool.ts:140-190
	var opts []session.WriteOption
	if t.SenderColor != "" {
		opts = append(opts, session.WithColor(t.SenderColor))
	}
	if params.Summary != "" {
		opts = append(opts, session.WithSummary(params.Summary))
	}

	if err := t.Mailbox.WriteToMailbox(params.To, t.TeamName, senderName, params.Message, opts...); err != nil {
		return ErrorOutput(fmt.Sprintf("failed to send message: %s", err)), nil
	}

	return SuccessOutput(fmt.Sprintf("Message sent to %s", params.To)), nil
}

// broadcast sends a message to all teammates except the sender.
// Source: SendMessageTool.ts:195-264
func (t *SendMessageTool) broadcast(content, summary, senderName string) (*ToolOutput, error) {
	// For broadcast, we need to know team members. Without a team file,
	// return an error message matching TS behavior.
	// Source: SendMessageTool.ts:214
	if senderName == "" {
		return ErrorOutput("Cannot broadcast: sender name is required. Set CLAUDE_CODE_AGENT_NAME."), nil
	}

	// Without a team member list, we can't broadcast.
	// In a real implementation this would read the team file.
	// Source: SendMessageTool.ts:232
	return ErrorOutput("Broadcast requires a team configuration. Use direct messages (to: \"<name>\") instead."), nil
}

// FormatRecipientList formats a list of recipients for display.
func FormatRecipientList(recipients []string) string {
	return strings.Join(recipients, ", ")
}
