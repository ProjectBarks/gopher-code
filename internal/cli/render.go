package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/query"
)

// PrintEvent renders a QueryEvent to stdout with ANSI colors.
func PrintEvent(evt query.QueryEvent) {
	switch evt.Type {
	case query.QEventTextDelta:
		fmt.Print(evt.Text) // Stream text as it arrives

	case query.QEventToolUseStart:
		fmt.Printf("\n\033[36m⚙ %s\033[0m\n", evt.ToolName) // Cyan tool name

	case query.QEventToolResult:
		if evt.IsError {
			fmt.Printf("\033[31m✗ Error: %s\033[0m\n", truncate(evt.Content, 200))
		} else {
			fmt.Printf("\033[32m✓ %s\033[0m\n", truncate(evt.Content, 200))
		}

	case query.QEventTurnComplete:
		// Nothing needed

	case query.QEventUsage:
		// Optionally show usage — silent by default
	}
}

// PlainTextCallback prints just the text with no ANSI colors or formatting.
func PlainTextCallback(evt query.QueryEvent) {
	if evt.Type == query.QEventTextDelta {
		fmt.Print(evt.Text)
	}
}

// StreamJSONCallback prints each event as a JSON line (newline-delimited JSON).
// Uses ndjsonSafeStringify to escape U+2028/U+2029 line terminators.
// Source: cli/ndjsonSafeStringify.ts
func StreamJSONCallback(evt query.QueryEvent) {
	data, _ := json.Marshal(map[string]interface{}{
		"type":    string(evt.Type),
		"text":    evt.Text,
		"tool":    evt.ToolName,
		"content": evt.Content,
	})
	fmt.Println(ndjsonSafeStringify(string(data)))
}

// ndjsonSafeStringify escapes U+2028 LINE SEPARATOR and U+2029 PARAGRAPH SEPARATOR
// in a JSON string so the serialized output cannot be broken by line-splitting receivers.
// Source: cli/ndjsonSafeStringify.ts
func ndjsonSafeStringify(jsonStr string) string {
	// Go's encoding/json already escapes these characters in string values,
	// but they could appear in raw pre-encoded JSON or via custom marshalers.
	jsonStr = strings.ReplaceAll(jsonStr, "\u2028", `\u2028`)
	jsonStr = strings.ReplaceAll(jsonStr, "\u2029", `\u2029`)
	return jsonStr
}

// JSONCollector collects events and emits a final JSON envelope.
type JSONCollector struct {
	text        strings.Builder
	toolCalls   []jsonToolCall
	toolResults []jsonToolResult
	usage       *jsonUsage
	stopReason  string
}

type jsonToolCall struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jsonToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

type jsonUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// NewJSONCollector creates a new JSONCollector.
func NewJSONCollector() *JSONCollector {
	return &JSONCollector{}
}

// Callback processes a QueryEvent into the collector.
func (c *JSONCollector) Callback(evt query.QueryEvent) {
	switch evt.Type {
	case query.QEventTextDelta:
		c.text.WriteString(evt.Text)
	case query.QEventToolUseStart:
		c.toolCalls = append(c.toolCalls, jsonToolCall{ID: evt.ToolUseID, Name: evt.ToolName})
	case query.QEventToolResult:
		c.toolResults = append(c.toolResults, jsonToolResult{
			ToolUseID: evt.ToolUseID, Content: evt.Content, IsError: evt.IsError,
		})
	case query.QEventTurnComplete:
		c.stopReason = string(evt.StopReason)
	case query.QEventUsage:
		c.usage = &jsonUsage{InputTokens: evt.InputTokens, OutputTokens: evt.OutputTokens}
	}
}

// Emit prints the final JSON envelope to stdout.
func (c *JSONCollector) Emit() {
	result := map[string]interface{}{
		"type":        "result",
		"role":        "assistant",
		"content":     []map[string]interface{}{{"type": "text", "text": c.text.String()}},
		"stop_reason": c.stopReason,
	}
	if c.usage != nil {
		result["usage"] = c.usage
	}
	if len(c.toolCalls) > 0 {
		result["tool_calls"] = c.toolCalls
	}
	if len(c.toolResults) > 0 {
		result["tool_results"] = c.toolResults
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
