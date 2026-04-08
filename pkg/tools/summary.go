package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Source: services/toolUseSummary/toolUseSummaryGenerator.ts

// ToolUseSummarySystemPrompt is the system prompt sent to Haiku for generating
// tool use summaries. These appear as single-line labels in SDK/mobile clients.
const ToolUseSummarySystemPrompt = `Write a short summary label describing what these tool calls accomplished. It appears as a single-line row in a mobile app and truncates around 30 characters, so think git-commit-subject, not sentence.

Keep the verb in past tense and the most distinctive noun. Drop articles, connectors, and long location context first.

Examples:
- Searched in auth/
- Fixed NPE in UserService
- Created signup endpoint
- Read config.json
- Ran failing tests`

// ToolSummaryInfo holds the tool name, input, and output for summarization.
type ToolSummaryInfo struct {
	Name   string `json:"name"`
	Input  any    `json:"input"`
	Output any    `json:"output"`
}

// FormatToolSummaryPrompt builds the user message for the Haiku summarization call.
// Source: toolUseSummaryGenerator.ts:57-78
func FormatToolSummaryPrompt(tools []ToolSummaryInfo, lastAssistantText string) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Summarize what these tool calls accomplished:\n\n")

	for _, t := range tools {
		inputStr := truncateJSON(t.Input, 300)
		outputStr := truncateJSON(t.Output, 300)
		sb.WriteString(fmt.Sprintf("Tool: %s\nInput: %s\nOutput: %s\n\n", t.Name, inputStr, outputStr))
	}

	if lastAssistantText != "" {
		truncated := lastAssistantText
		if len(truncated) > 200 {
			truncated = truncated[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("Context (assistant's last message): %s\n", truncated))
	}

	return sb.String()
}

// truncateJSON marshals v to JSON and truncates to maxLen characters.
func truncateJSON(v any, maxLen int) string {
	if v == nil {
		return "null"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	s := string(b)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
