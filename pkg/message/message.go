package message

import (
	"encoding/json"
	"strings"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ContentBlockType string

const (
	ContentText       ContentBlockType = "text"
	ContentToolUse    ContentBlockType = "tool_use"
	ContentToolResult ContentBlockType = "tool_result"
)

// ContentBlock is a tagged union. Go lacks sum types, so we use a struct with a Type discriminator.
type ContentBlock struct {
	Type      ContentBlockType `json:"type"`
	Text      string           `json:"text,omitempty"`        // for text blocks
	ID        string           `json:"id,omitempty"`          // for tool_use
	Name      string           `json:"name,omitempty"`        // for tool_use
	Input     json.RawMessage  `json:"input,omitempty"`       // for tool_use (deferred parsing)
	ToolUseID string           `json:"tool_use_id,omitempty"` // for tool_result
	Content   string           `json:"content,omitempty"`     // for tool_result
	IsError   bool             `json:"is_error,omitempty"`    // for tool_result
}

type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// UserMessage creates a user message with a single text block.
func UserMessage(text string) Message {
	return Message{
		Role:    RoleUser,
		Content: []ContentBlock{{Type: ContentText, Text: text}},
	}
}

// TextBlock creates a text content block.
func TextBlock(text string) ContentBlock {
	return ContentBlock{Type: ContentText, Text: text}
}

// ToolUseBlock creates a tool_use content block.
func ToolUseBlock(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{Type: ContentToolUse, ID: id, Name: name, Input: input}
}

// ToolResultBlock creates a tool_result content block.
func ToolResultBlock(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{Type: ContentToolResult, ToolUseID: toolUseID, Content: content, IsError: isError}
}

// ToolUses returns all tool_use blocks from this message.
func (m Message) ToolUses() []ContentBlock {
	var uses []ContentBlock
	for _, b := range m.Content {
		if b.Type == ContentToolUse {
			uses = append(uses, b)
		}
	}
	return uses
}

// --- Message Normalization ---
// Source: utils/messages.ts:1989-2370 (normalizeMessagesForAPI)

// SyntheticToolResultPlaceholder is the placeholder text for missing tool results.
// Source: utils/messages.ts:247
const SyntheticToolResultPlaceholder = "[Tool result missing due to internal error]"

// SystemReminderPrefix is the XML tag that wraps system-injected context.
// Source: utils/messages.ts:3097-3098
const SystemReminderPrefix = "<system-reminder>"

// WrapInSystemReminder wraps content in <system-reminder> tags.
// Source: utils/messages.ts:3097-3098
func WrapInSystemReminder(content string) string {
	return "<system-reminder>\n" + content + "\n</system-reminder>"
}

// NormalizeForAPI produces API-ready messages matching the TS normalizeMessagesForAPI pipeline:
// 1. Smoosh consecutive same-role messages
// 2. Ensure every tool_use has a matching tool_result (synthesize missing ones)
// 3. Smoosh <system-reminder> siblings into adjacent tool_results
// 4. Filter whitespace-only assistant messages
// 5. Ensure non-empty assistant content
// Source: utils/messages.ts:1989
func NormalizeForAPI(messages []Message) []Message {
	// Step 1: Smoosh consecutive same-role messages
	smooshed := smooshConsecutive(messages)

	// Step 2: Ensure tool_use/tool_result pairing
	paired := ensureToolResultPairing(smooshed)

	// Step 3: Smoosh system-reminder text siblings into tool_results
	// Source: utils/messages.ts:2334-2338
	withSR := smooshSystemReminderSiblings(paired)

	// Step 4: Filter whitespace-only assistant messages
	filtered := filterWhitespaceOnlyAssistants(withSR)

	// Step 5: Ensure non-empty assistant content
	result := ensureNonEmptyAssistantContent(filtered)

	return result
}

// smooshConsecutive merges consecutive messages with the same role.
// Source: utils/messages.ts:2188-2199 (user merging), 2250-2264 (assistant merging)
func smooshConsecutive(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}
	result := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if len(result) == 0 {
			result = append(result, msg)
			continue
		}
		last := &result[len(result)-1]
		if last.Role == msg.Role {
			if msg.Role == RoleUser {
				// Source: utils/messages.ts:2411-2449 (mergeUserMessages)
				last.Content = hoistToolResults(joinTextAtSeam(last.Content, msg.Content))
			} else {
				// Source: utils/messages.ts:2389-2400 (mergeAssistantMessages)
				last.Content = append(last.Content, msg.Content...)
			}
		} else {
			result = append(result, msg)
		}
	}
	return result
}

// hoistToolResults moves tool_result blocks before other blocks in a content array.
// Source: utils/messages.ts:2470-2483
func hoistToolResults(content []ContentBlock) []ContentBlock {
	var toolResults, other []ContentBlock
	for _, b := range content {
		if b.Type == ContentToolResult {
			toolResults = append(toolResults, b)
		} else {
			other = append(other, b)
		}
	}
	result := make([]ContentBlock, 0, len(content))
	result = append(result, toolResults...)
	result = append(result, other...)
	return result
}

// joinTextAtSeam appends \n to the last text block of a when both sides end/start with text.
// Source: utils/messages.ts:2505-2515
func joinTextAtSeam(a, b []ContentBlock) []ContentBlock {
	if len(a) > 0 && len(b) > 0 && a[len(a)-1].Type == ContentText && b[0].Type == ContentText {
		merged := make([]ContentBlock, len(a)-1, len(a)+len(b))
		copy(merged, a[:len(a)-1])
		lastA := a[len(a)-1]
		lastA.Text += "\n"
		merged = append(merged, lastA)
		merged = append(merged, b...)
		return merged
	}
	result := make([]ContentBlock, 0, len(a)+len(b))
	result = append(result, a...)
	result = append(result, b...)
	return result
}

// ensureToolResultPairing ensures every tool_use has a matching tool_result.
// If a tool_use has no result, a synthetic one is inserted.
// Also deduplicates tool_use blocks by ID.
// Source: utils/messages.ts:5133-5454
func ensureToolResultPairing(messages []Message) []Message {
	result := make([]Message, 0, len(messages))
	allSeenToolUseIDs := make(map[string]bool)

	for i := 0; i < len(messages); i++ {
		msg := messages[i]

		if msg.Role != RoleAssistant {
			// Strip orphaned tool_results from user messages not preceded by an assistant
			if msg.Role == RoleUser && (len(result) == 0 || result[len(result)-1].Role != RoleAssistant) {
				stripped := make([]ContentBlock, 0, len(msg.Content))
				for _, b := range msg.Content {
					if b.Type != ContentToolResult {
						stripped = append(stripped, b)
					}
				}
				if len(stripped) != len(msg.Content) {
					if len(stripped) == 0 && len(result) == 0 {
						// Keep a placeholder so API gets a user message first
						stripped = []ContentBlock{{Type: ContentText, Text: "[Orphaned tool result removed due to conversation resume]"}}
					}
					if len(stripped) > 0 {
						result = append(result, Message{Role: msg.Role, Content: stripped})
					}
					continue
				}
			}
			result = append(result, msg)
			continue
		}

		// Assistant message: dedupe tool_use IDs
		seenIDs := make(map[string]bool)
		finalContent := make([]ContentBlock, 0, len(msg.Content))
		for _, b := range msg.Content {
			if b.Type == ContentToolUse {
				if allSeenToolUseIDs[b.ID] {
					continue // Duplicate, skip
				}
				allSeenToolUseIDs[b.ID] = true
				seenIDs[b.ID] = true
			}
			finalContent = append(finalContent, b)
		}

		if len(finalContent) == 0 {
			finalContent = []ContentBlock{{Type: ContentText, Text: ""}}
		}

		result = append(result, Message{Role: RoleAssistant, Content: finalContent})

		// Collect tool_use IDs that need results
		var toolUseIDs []string
		for id := range seenIDs {
			toolUseIDs = append(toolUseIDs, id)
		}
		if len(toolUseIDs) == 0 {
			continue
		}

		// Check next message for tool_results
		existingResults := make(map[string]bool)
		if i+1 < len(messages) && messages[i+1].Role == RoleUser {
			for _, b := range messages[i+1].Content {
				if b.Type == ContentToolResult {
					existingResults[b.ToolUseID] = true
				}
			}
		}

		// Synthesize missing results
		var syntheticBlocks []ContentBlock
		for _, id := range toolUseIDs {
			if !existingResults[id] {
				syntheticBlocks = append(syntheticBlocks, ToolResultBlock(id, SyntheticToolResultPlaceholder, true))
			}
		}

		if len(syntheticBlocks) > 0 {
			if i+1 < len(messages) && messages[i+1].Role == RoleUser {
				// Prepend synthetic results to existing user message
				combined := make([]ContentBlock, 0, len(syntheticBlocks)+len(messages[i+1].Content))
				combined = append(combined, syntheticBlocks...)
				combined = append(combined, messages[i+1].Content...)
				result = append(result, Message{Role: RoleUser, Content: combined})
				i++ // Skip the next message since we merged it
			} else {
				// Insert a new user message with synthetic results
				result = append(result, Message{Role: RoleUser, Content: syntheticBlocks})
			}
		}
	}

	return result
}

// filterWhitespaceOnlyAssistants removes assistant messages with only whitespace text.
// Source: utils/messages.ts:2324 (filterWhitespaceOnlyAssistantMessages)
func filterWhitespaceOnlyAssistants(messages []Message) []Message {
	result := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == RoleAssistant {
			hasNonWhitespace := false
			for _, b := range msg.Content {
				if b.Type != ContentText {
					hasNonWhitespace = true
					break
				}
				if len(b.Text) > 0 {
					for _, r := range b.Text {
						if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
							hasNonWhitespace = true
							break
						}
					}
				}
				if hasNonWhitespace {
					break
				}
			}
			if !hasNonWhitespace {
				continue // Skip this message
			}
		}
		result = append(result, msg)
	}
	// Re-smoosh after filtering (may create consecutive same-role)
	return smooshConsecutive(result)
}

// ensureNonEmptyAssistantContent ensures assistant messages have at least one content block.
// Source: utils/messages.ts:2325 (ensureNonEmptyAssistantContent)
func ensureNonEmptyAssistantContent(messages []Message) []Message {
	for i := range messages {
		if messages[i].Role == RoleAssistant && len(messages[i].Content) == 0 {
			messages[i].Content = []ContentBlock{{Type: ContentText, Text: ""}}
		}
	}
	return messages
}

// smooshSystemReminderSiblings moves <system-reminder>-prefixed text blocks
// into the last tool_result of the same user message.
// Source: utils/messages.ts:1835-1873
func smooshSystemReminderSiblings(messages []Message) []Message {
	result := make([]Message, len(messages))
	for i, msg := range messages {
		if msg.Role != RoleUser {
			result[i] = msg
			continue
		}

		// Check if this user message has any tool_results
		hasToolResult := false
		for _, b := range msg.Content {
			if b.Type == ContentToolResult {
				hasToolResult = true
				break
			}
		}
		if !hasToolResult {
			result[i] = msg
			continue
		}

		// Separate <system-reminder> text blocks from everything else
		var srTexts []string
		var kept []ContentBlock
		for _, b := range msg.Content {
			if b.Type == ContentText && strings.HasPrefix(b.Text, SystemReminderPrefix) {
				srTexts = append(srTexts, b.Text)
			} else {
				kept = append(kept, b)
			}
		}
		if len(srTexts) == 0 {
			result[i] = msg
			continue
		}

		// Find the LAST tool_result and smoosh SR text into it
		// Source: utils/messages.ts:1858
		lastTrIdx := -1
		for j := len(kept) - 1; j >= 0; j-- {
			if kept[j].Type == ContentToolResult {
				lastTrIdx = j
				break
			}
		}
		if lastTrIdx < 0 {
			result[i] = msg
			continue
		}

		// Append SR text to the tool_result's content
		tr := kept[lastTrIdx]
		for _, sr := range srTexts {
			if tr.Content != "" {
				tr.Content += "\n"
			}
			tr.Content += sr
		}
		kept[lastTrIdx] = tr

		result[i] = Message{Role: msg.Role, Content: kept}
	}
	return result
}

// isSystemReminder checks if a text block is a system reminder.
func isSystemReminder(text string) bool {
	return strings.HasPrefix(text, SystemReminderPrefix)
}
