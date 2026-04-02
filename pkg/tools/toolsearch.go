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

	query := strings.ToLower(in.Query)
	allTools := t.registry.All()

	type match struct {
		name        string
		description string
		score       int
	}

	var matches []match

	for _, tool := range allTools {
		name := tool.Name()
		desc := tool.Description()
		nameLower := strings.ToLower(name)
		descLower := strings.ToLower(desc)

		score := 0

		// Exact name match gets highest score
		if nameLower == query {
			score = 100
		} else if strings.Contains(nameLower, query) {
			score = 50
		}

		// Check description
		if strings.Contains(descLower, query) {
			score += 25
		}

		// Check individual query words
		words := strings.Fields(query)
		for _, word := range words {
			if strings.Contains(nameLower, word) {
				score += 10
			}
			if strings.Contains(descLower, word) {
				score += 5
			}
		}

		if score > 0 {
			matches = append(matches, match{name: name, description: desc, score: score})
		}
	}

	// Sort by score descending (simple insertion sort for small lists)
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].score > matches[j-1].score; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	// Limit results
	if len(matches) > in.MaxResults {
		matches = matches[:in.MaxResults]
	}

	if len(matches) == 0 {
		return SuccessOutput(fmt.Sprintf("No tools found matching %q", in.Query)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d tool(s) matching %q:\n\n", len(matches), in.Query))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", m.name, m.description))
	}

	return SuccessOutput(sb.String()), nil
}
