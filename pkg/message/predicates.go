package message

// Message predicates — pure functions on Message types.
// Source: utils/messages.ts, utils/messagePredicates.ts

// IsAssistantMessage returns true if the message has role "assistant".
func IsAssistantMessage(m Message) bool {
	return m.Role == RoleAssistant
}

// IsUserMessage returns true if the message has role "user".
func IsUserMessage(m Message) bool {
	return m.Role == RoleUser
}

// IsToolUseMessage returns true if the assistant message contains at least one tool_use block.
// Source: utils/messages.ts:829-836
func IsToolUseMessage(m Message) bool {
	if m.Role != RoleAssistant {
		return false
	}
	for _, b := range m.Content {
		if b.Type == ContentToolUse {
			return true
		}
	}
	return false
}

// IsToolResultMessage returns true if the user message starts with a tool_result block.
// Source: utils/messages.ts:843-851
func IsToolResultMessage(m Message) bool {
	if m.Role != RoleUser {
		return false
	}
	return len(m.Content) > 0 && m.Content[0].Type == ContentToolResult
}

// IsHumanTurn returns true for user messages that are genuine human input
// (not tool results or meta messages). In our Go model, tool_result messages
// always start with a tool_result content block.
// Source: utils/messagePredicates.ts:6-7
func IsHumanTurn(m Message) bool {
	if m.Role != RoleUser {
		return false
	}
	// If the first block is a tool_result, this is a tool result message, not a human turn.
	if len(m.Content) > 0 && m.Content[0].Type == ContentToolResult {
		return false
	}
	return true
}

// IsSyntheticMessage returns true if the message's first text block is a known synthetic message.
// Source: utils/messages.ts:310-318
func IsSyntheticMessage(m Message) bool {
	if len(m.Content) == 0 {
		return false
	}
	if m.Content[0].Type != ContentText {
		return false
	}
	return SyntheticMessages[m.Content[0].Text]
}

// GetLastAssistantMessage returns the last assistant message, or nil if none.
// Source: utils/messages.ts:331-338
func GetLastAssistantMessage(messages []Message) *Message {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleAssistant {
			return &messages[i]
		}
	}
	return nil
}

// HasToolCallsInLastAssistantTurn checks if the last assistant message has tool_use blocks.
// Source: utils/messages.ts:341-353
func HasToolCallsInLastAssistantTurn(messages []Message) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleAssistant {
			return IsToolUseMessage(messages[i])
		}
	}
	return false
}

// HasSuccessfulToolCall checks if the most recent call of the named tool succeeded.
// Searches backwards for efficiency.
// Source: utils/messages.ts:4719-4755
func HasSuccessfulToolCall(messages []Message, toolName string) bool {
	// Find the most recent tool_use for this tool.
	var mostRecentID string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != RoleAssistant {
			continue
		}
		for _, b := range messages[i].Content {
			if b.Type == ContentToolUse && b.Name == toolName {
				mostRecentID = b.ID
				break
			}
		}
		if mostRecentID != "" {
			break
		}
	}
	if mostRecentID == "" {
		return false
	}

	// Find the corresponding tool_result.
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != RoleUser {
			continue
		}
		for _, b := range messages[i].Content {
			if b.Type == ContentToolResult && b.ToolUseID == mostRecentID {
				return !b.IsError
			}
		}
	}
	return false
}
