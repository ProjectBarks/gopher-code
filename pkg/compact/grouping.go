package compact

// Source: services/compact/grouping.ts

import "github.com/projectbarks/gopher-code/pkg/message"

// MessageIDFunc extracts a stable API-round identifier from a message.
// In the TS source, this is msg.message.id (the API response ID).
// Callers provide an implementation that maps their message storage
// to the API response ID. Messages with the same ID belong to the
// same API round.
type MessageIDFunc func(msg message.Message, index int) string

// GroupMessagesByAPIRound groups messages at API-round boundaries.
// A boundary fires when a new assistant message begins (different ID
// from the prior assistant). Streaming chunks from the same API response
// share an ID, so they stay in one group.
//
// The idFunc maps each message to its API response ID. For assistant
// messages this is the response message ID; for user messages it is
// typically empty (they inherit the current group).
//
// For well-formed conversations the API contract guarantees every tool_use
// is resolved before the next assistant turn, so ID-based boundaries are
// API-safe split points.
// Source: grouping.ts:22-63
func GroupMessagesByAPIRound(messages []message.Message, idFunc MessageIDFunc) [][]message.Message {
	var groups [][]message.Message
	var current []message.Message
	lastAssistantID := ""

	for i, msg := range messages {
		msgID := ""
		if idFunc != nil {
			msgID = idFunc(msg, i)
		}

		if msg.Role == message.RoleAssistant &&
			msgID != lastAssistantID &&
			len(current) > 0 {
			groups = append(groups, current)
			current = []message.Message{msg}
		} else {
			current = append(current, msg)
		}
		if msg.Role == message.RoleAssistant {
			lastAssistantID = msgID
		}
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

// GroupMessagesByToolUseID is a convenience MessageIDFunc that uses
// the first tool_use block's ID as the message ID. This is a reasonable
// approximation when the API response ID is not available.
func GroupMessagesByToolUseID(msg message.Message, _ int) string {
	if msg.Role != message.RoleAssistant {
		return ""
	}
	for _, block := range msg.Content {
		if block.Type == message.ContentToolUse {
			return block.ID
		}
	}
	return ""
}
