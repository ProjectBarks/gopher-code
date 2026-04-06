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

// Source: tools/SendMessageTool/constants.ts
func (t *SendMessageTool) Name() string { return "SendMessage" }

// Source: tools/SendMessageTool/prompt.ts — DESCRIPTION
func (t *SendMessageTool) Description() string { return "Send a message to another agent" }

// Source: SendMessageTool.ts:539 — isReadOnly returns true only for string messages;
// the tool writes to the mailbox, so the default is false.
func (t *SendMessageTool) IsReadOnly() bool { return false }

// SearchHint returns a search hint for ToolSearch discovery.
// Source: SendMessageTool.ts:524
func (t *SendMessageTool) SearchHint() string {
	return "send messages to agent teammates (swarm protocol)"
}

// MaxResultSizeChars returns the per-tool result size limit.
// Source: SendMessageTool.ts:524
func (t *SendMessageTool) MaxResultSizeChars() int { return 100_000 }

// Source: tools/SendMessageTool/SendMessageTool.ts:60-80
func (t *SendMessageTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"to": {"type": "string", "description": "Recipient: teammate name, or \"*\" for broadcast to all teammates"},
			"summary": {"type": "string", "description": "A 5-10 word summary shown as a preview in the UI (required when message is a string)"},
			"message": {"type": "string", "description": "The message content to send"}
		},
		"required": ["to", "message"],
		"additionalProperties": false
	}`)
}

// Prompt returns the system prompt section guiding the model on SendMessage usage.
// Source: tools/SendMessageTool/prompt.ts — getPrompt()
func (t *SendMessageTool) Prompt() string {
	return "# SendMessage\n\nSend a message to another agent.\n\n" +
		"```json\n" +
		"{\"to\": \"researcher\", \"summary\": \"assign task 1\", \"message\": \"start on task #1\"}\n" +
		"```\n\n" +
		"| `to` | |\n" +
		"|---|---|\n" +
		"| `\"researcher\"` | Teammate by name |\n" +
		"| `\"*\"` | Broadcast to all teammates \u2014 expensive (linear in team size), use only when everyone genuinely needs it |\n\n" +
		"Your plain text output is NOT visible to other agents \u2014 to communicate, you MUST call this tool. " +
		"Messages from teammates are delivered automatically; you don't check an inbox. " +
		"Refer to teammates by name, never by UUID. " +
		"When relaying, don't quote the original \u2014 it's already rendered to the user.\n\n" +
		"## Protocol responses (legacy)\n\n" +
		"If you receive a JSON message with `type: \"shutdown_request\"` or `type: \"plan_approval_request\"`, " +
		"respond with the matching `_response` type \u2014 echo the `request_id`, set `approve` true/false:\n\n" +
		"```json\n" +
		"{\"to\": \"team-lead\", \"message\": {\"type\": \"shutdown_response\", \"request_id\": \"...\", \"approve\": true}}\n" +
		"{\"to\": \"researcher\", \"message\": {\"type\": \"plan_approval_response\", \"request_id\": \"...\", \"approve\": false, \"feedback\": \"add error handling\"}}\n" +
		"```\n\n" +
		"Approving shutdown terminates your process. " +
		"Rejecting plan sends the teammate back to revise. " +
		"Don't originate `shutdown_request` unless asked. " +
		"Don't send structured JSON status messages \u2014 use TaskUpdate."
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

	// Source: SendMessageTool.ts:605-609 — validateInput: empty to
	if strings.TrimSpace(params.To) == "" {
		return ErrorOutput("to must not be empty"), nil
	}
	if params.Message == "" {
		return ErrorOutput("message is required"), nil
	}

	// Source: SendMessageTool.ts:623-628 — validateInput: @ in to field
	if strings.Contains(params.To, "@") {
		return ErrorOutput("to must be a bare teammate name or \"*\" \u2014 there is only one team per session"), nil
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

	// Source: SendMessageTool.ts:179 — "Message sent to {name}'s inbox"
	return SuccessOutput(fmt.Sprintf("Message sent to %s's inbox", params.To)), nil
}

// broadcast sends a message to all teammates except the sender.
// Source: SendMessageTool.ts:195-264
func (t *SendMessageTool) broadcast(content, summary, senderName string) (*ToolOutput, error) {
	// Source: SendMessageTool.ts:214
	if senderName == "" {
		return ErrorOutput("Cannot broadcast: sender name is required. Set CLAUDE_CODE_AGENT_NAME."), nil
	}

	// Source: SendMessageTool.ts:199-203
	if t.TeamName == "" {
		return ErrorOutput("Not in a team context. Create a team with Teammate spawnTeam first, or set CLAUDE_CODE_TEAM_NAME."), nil
	}

	// Read team file to get member list for fan-out
	// Source: SendMessageTool.ts:205-206
	teamFile, err := session.ReadTeamFileFromDir(t.Mailbox.TeamsDir(), t.TeamName)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("Team %q does not exist", t.TeamName)), nil
	}

	// Filter out the sender
	// Source: SendMessageTool.ts:220-225
	var recipients []string
	for _, member := range teamFile.Members {
		if strings.EqualFold(member.Name, senderName) {
			continue
		}
		recipients = append(recipients, member.Name)
	}

	// Source: SendMessageTool.ts:228-231
	if len(recipients) == 0 {
		return SuccessOutput("No teammates to broadcast to (you are the only team member)"), nil
	}

	// Fan-out: write to each recipient's inbox
	// Source: SendMessageTool.ts:238-250
	var opts []session.WriteOption
	if t.SenderColor != "" {
		opts = append(opts, session.WithColor(t.SenderColor))
	}
	if summary != "" {
		opts = append(opts, session.WithSummary(summary))
	}

	for _, name := range recipients {
		if err := t.Mailbox.WriteToMailbox(name, t.TeamName, senderName, content, opts...); err != nil {
			return ErrorOutput(fmt.Sprintf("failed to broadcast to %s: %s", name, err)), nil
		}
	}

	// Source: SendMessageTool.ts:253
	return SuccessOutput(fmt.Sprintf("Message broadcast to %d teammate(s): %s", len(recipients), strings.Join(recipients, ", "))), nil
}

// FormatRecipientList formats a list of recipients for display.
func FormatRecipientList(recipients []string) string {
	return strings.Join(recipients, ", ")
}
