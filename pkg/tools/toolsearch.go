package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolSearchTool searches available tools by keyword.
type ToolSearchTool struct {
	registry *ToolRegistry
}

// NewToolSearchTool creates a ToolSearchTool with a reference to the tool registry.
func NewToolSearchTool(registry *ToolRegistry) *ToolSearchTool {
	return &ToolSearchTool{registry: registry}
}

type toolSearchInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

func (t *ToolSearchTool) Name() string        { return "ToolSearch" }
func (t *ToolSearchTool) Description() string { return "Search available tools by keyword" }
func (t *ToolSearchTool) IsReadOnly() bool    { return true }

func (t *ToolSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query to match against tool names and descriptions"},
			"max_results": {"type": "integer", "description": "Maximum number of results to return (default: 5)"}
		},
		"required": ["query"],
		"additionalProperties": false
	}`)
}

// Execute handles three query modes:
//  1. "select:ToolName" — direct selection by exact name (comma-separated for multiple)
//  2. "+keyword name" — require keyword in name, rank by remaining terms
//  3. Keyword search — match against tool names and descriptions
//
// Source: ToolSearchTool/ToolSearchTool.ts
func (t *ToolSearchTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in toolSearchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Query == "" {
		return ErrorOutput("query is required"), nil
	}
	if in.MaxResults <= 0 {
		in.MaxResults = 5
	}

	allTools := t.registry.All()

	// Mode 1: Direct selection — "select:Read,Edit,Grep"
	if strings.HasPrefix(in.Query, "select:") {
		names := strings.Split(strings.TrimPrefix(in.Query, "select:"), ",")
		var found []string
		for _, name := range names {
			name = strings.TrimSpace(name)
			if tool := t.registry.Get(name); tool != nil {
				found = append(found, name)
			}
		}
		if len(found) == 0 {
			return SuccessOutput(fmt.Sprintf("No tools found matching %q. Available: %s", in.Query, toolNameList(allTools))), nil
		}
		return SuccessOutput(formatToolResults(found, in.Query, allTools)), nil
	}

	// Mode 2: Required name keyword — "+keyword rest"
	query := in.Query
	var requiredInName string
	if strings.HasPrefix(query, "+") {
		parts := strings.SplitN(query[1:], " ", 2)
		requiredInName = strings.ToLower(parts[0])
		if len(parts) > 1 {
			query = parts[1]
		} else {
			query = ""
		}
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))

	type match struct {
		name  string
		score int
	}

	var matches []match

	for _, tool := range allTools {
		name := tool.Name()
		nameLower := strings.ToLower(name)
		desc := strings.ToLower(tool.Description())

		// If required keyword specified, filter by name
		if requiredInName != "" && !strings.Contains(nameLower, requiredInName) {
			continue
		}

		score := 0

		// Parse tool name into searchable parts (CamelCase + MCP)
		nameParts := parseToolNameParts(name)

		// Exact name match
		if nameLower == queryLower {
			score = 100
		} else if strings.Contains(nameLower, queryLower) {
			score = 50
		}

		// Check description
		if queryLower != "" && strings.Contains(desc, queryLower) {
			score += 25
		}

		// Word-level matching
		words := strings.Fields(queryLower)
		for _, word := range words {
			if strings.Contains(nameParts, word) {
				score += 15
			}
			if strings.Contains(desc, word) {
				score += 5
			}
		}

		// If required keyword matched and no other query, give base score
		if requiredInName != "" && queryLower == "" && score == 0 {
			score = 10
		}

		if score > 0 {
			matches = append(matches, match{name: name, score: score})
		}
	}

	// Sort by score descending
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].score > matches[j-1].score; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	if len(matches) > in.MaxResults {
		matches = matches[:in.MaxResults]
	}

	if len(matches) == 0 {
		return SuccessOutput(fmt.Sprintf("No tools found matching %q", in.Query)), nil
	}

	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.name
	}
	return SuccessOutput(formatToolResults(names, in.Query, allTools)), nil
}

// parseToolNameParts splits a tool name into searchable words.
// CamelCase → words, mcp__server__tool → server tool parts.
func parseToolNameParts(name string) string {
	if strings.HasPrefix(name, "mcp__") {
		without := strings.TrimPrefix(name, "mcp__")
		return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(without, "__", " "), "_", " "))
	}
	// CamelCase to spaces
	var sb strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			sb.WriteByte(' ')
		}
		sb.WriteRune(r)
	}
	return strings.ToLower(strings.ReplaceAll(sb.String(), "_", " "))
}

func formatToolResults(names []string, query string, allTools []Tool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d tool(s) matching %q:\n\n", len(names), query))
	for _, name := range names {
		sb.WriteString(fmt.Sprintf("- %s\n", name))
	}
	return sb.String()
}

func toolNameList(tools []Tool) string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name()
	}
	return strings.Join(names, ", ")
}
