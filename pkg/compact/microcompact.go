package compact

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/message"
)

// Source: services/compact/microCompact.ts

// ImageMaxTokenSize is the estimated token count for image/document blocks.
// Source: services/compact/microCompact.ts:38
const ImageMaxTokenSize = 2000

// TimeBasedMCClearedMessage is the replacement text for cleared tool results.
// Source: services/compact/microCompact.ts:36
const TimeBasedMCClearedMessage = "[Old tool result content cleared]"

// CompactableTools is the set of tool names eligible for micro-compaction.
// Source: services/compact/microCompact.ts:41-50
var CompactableTools = map[string]bool{
	"Read":      true,
	"Bash":      true,
	"Grep":      true,
	"Glob":      true,
	"WebSearch": true,
	"WebFetch":  true,
	"Edit":      true,
	"Write":     true,
}

// RoughTokenCountEstimation estimates token count from text length.
// Uses the ~4 chars per token heuristic.
// Source: services/tokenEstimation.ts (roughTokenCountEstimation)
func RoughTokenCountEstimation(text string) int {
	return int(math.Ceil(float64(len(text)) / 4.0))
}

// EstimateToolResultTokens estimates the token count of a tool_result content block.
// Source: services/compact/microCompact.ts:138-157
func EstimateToolResultTokens(content string) int {
	if content == "" {
		return 0
	}
	return RoughTokenCountEstimation(content)
}

// EstimateMessageTokens estimates token count for a slice of messages.
// Pads by 4/3 for conservative approximation.
// Source: services/compact/microCompact.ts:164-205
func EstimateMessageTokens(messages []message.Message) int {
	totalTokens := 0
	for _, msg := range messages {
		for _, block := range msg.Content {
			switch block.Type {
			case message.ContentText:
				totalTokens += RoughTokenCountEstimation(block.Text)
			case message.ContentToolResult:
				totalTokens += EstimateToolResultTokens(block.Content)
			case message.ContentToolUse:
				input := string(block.Input)
				if input == "" {
					input = "{}"
				}
				totalTokens += RoughTokenCountEstimation(block.Name + input)
			case message.ContentThinking:
				totalTokens += RoughTokenCountEstimation(block.Thinking)
			default:
				// For other block types, estimate from JSON representation
				data, _ := json.Marshal(block)
				totalTokens += RoughTokenCountEstimation(string(data))
			}
		}
	}
	// Pad by 4/3 for conservative estimation
	// Source: services/compact/microCompact.ts:204
	return int(math.Ceil(float64(totalTokens) * 4.0 / 3.0))
}

// CollectCompactableToolIDs returns tool_use IDs for compactable tools, in encounter order.
// Source: services/compact/microCompact.ts:226-241
func CollectCompactableToolIDs(messages []message.Message) []string {
	var ids []string
	for _, msg := range messages {
		if msg.Role != message.RoleAssistant {
			continue
		}
		for _, block := range msg.Content {
			if block.Type == message.ContentToolUse && CompactableTools[block.Name] {
				ids = append(ids, block.ID)
			}
		}
	}
	return ids
}

// MicroCompactMessages clears old tool results for eligible tools,
// keeping the most recent keepRecent results intact.
// Source: services/compact/microCompact.ts:446-493
func MicroCompactMessages(messages []message.Message, keepRecent int) ([]message.Message, int) {
	compactableIDs := CollectCompactableToolIDs(messages)

	// Floor at 1: always keep at least the last result
	// Source: services/compact/microCompact.ts:461
	if keepRecent < 1 {
		keepRecent = 1
	}

	// Build the keep set from the last keepRecent IDs
	// Source: services/compact/microCompact.ts:462
	keepSet := make(map[string]bool)
	start := len(compactableIDs) - keepRecent
	if start < 0 {
		start = 0
	}
	for _, id := range compactableIDs[start:] {
		keepSet[id] = true
	}

	// Build the clear set (everything not in keep)
	clearSet := make(map[string]bool)
	for _, id := range compactableIDs {
		if !keepSet[id] {
			clearSet[id] = true
		}
	}

	if len(clearSet) == 0 {
		return messages, 0
	}

	tokensSaved := 0
	result := make([]message.Message, len(messages))
	for i, msg := range messages {
		if msg.Role != message.RoleUser {
			result[i] = msg
			continue
		}
		touched := false
		newContent := make([]message.ContentBlock, len(msg.Content))
		for j, block := range msg.Content {
			if block.Type == message.ContentToolResult &&
				clearSet[block.ToolUseID] &&
				block.Content != TimeBasedMCClearedMessage {
				tokensSaved += EstimateToolResultTokens(block.Content)
				newContent[j] = message.ContentBlock{
					Type:      message.ContentToolResult,
					ToolUseID: block.ToolUseID,
					Content:   TimeBasedMCClearedMessage,
					IsError:   block.IsError,
				}
				touched = true
			} else {
				newContent[j] = block
			}
		}
		if touched {
			result[i] = message.Message{Role: msg.Role, Content: newContent}
		} else {
			result[i] = msg
		}
	}

	return result, tokensSaved
}

// AdjustIndexToPreserveAPIInvariants adjusts a compaction start index backwards
// to ensure tool_use/tool_result pairs are never split.
// Source: services/compact/sessionMemoryCompact.ts:232-314
func AdjustIndexToPreserveAPIInvariants(messages []message.Message, startIndex int) int {
	if startIndex <= 0 || startIndex >= len(messages) {
		return startIndex
	}

	adjustedIndex := startIndex

	// Step 1: Collect all tool_result IDs in the kept range (startIndex..end)
	// Source: services/compact/sessionMemoryCompact.ts:244-247
	var allToolResultIDs []string
	for i := startIndex; i < len(messages); i++ {
		for _, block := range messages[i].Content {
			if block.Type == message.ContentToolResult {
				allToolResultIDs = append(allToolResultIDs, block.ToolUseID)
			}
		}
	}

	if len(allToolResultIDs) > 0 {
		// Collect tool_use IDs already in the kept range
		// Source: services/compact/sessionMemoryCompact.ts:251-261
		toolUseIDsInKept := make(map[string]bool)
		for i := adjustedIndex; i < len(messages); i++ {
			if messages[i].Role == message.RoleAssistant {
				for _, block := range messages[i].Content {
					if block.Type == message.ContentToolUse {
						toolUseIDsInKept[block.ID] = true
					}
				}
			}
		}

		// Find tool_use IDs that have results but no use in the kept range
		// Source: services/compact/sessionMemoryCompact.ts:264-266
		neededIDs := make(map[string]bool)
		for _, id := range allToolResultIDs {
			if !toolUseIDsInKept[id] {
				neededIDs[id] = true
			}
		}

		// Walk backwards to find assistant messages with matching tool_use
		// Source: services/compact/sessionMemoryCompact.ts:269-285
		for i := adjustedIndex - 1; i >= 0 && len(neededIDs) > 0; i-- {
			if messages[i].Role != message.RoleAssistant {
				continue
			}
			found := false
			for _, block := range messages[i].Content {
				if block.Type == message.ContentToolUse && neededIDs[block.ID] {
					found = true
					delete(neededIDs, block.ID)
				}
			}
			if found {
				adjustedIndex = i
			}
		}
	}

	return adjustedIndex
}

// IsCompactableToolResult checks if a tool_result belongs to a compactable tool.
func IsCompactableToolResult(toolName string) bool {
	return CompactableTools[toolName]
}

// HasTextBlocks returns true if any content block in the message is a text block.
func HasTextBlocks(msg message.Message) bool {
	for _, b := range msg.Content {
		if b.Type == message.ContentText && strings.TrimSpace(b.Text) != "" {
			return true
		}
	}
	return false
}
