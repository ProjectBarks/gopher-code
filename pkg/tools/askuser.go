package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AskUserQuestionTool asks the user a question and waits for their response.
// Source: tools/AskUserQuestionTool/AskUserQuestionTool.tsx
type AskUserQuestionTool struct{}

func (t *AskUserQuestionTool) Name() string { return "AskUserQuestion" }
func (t *AskUserQuestionTool) Description() string {
	return "Asks the user multiple choice questions to gather information, clarify ambiguity, understand preferences, make decisions or offer them choices."
}
func (t *AskUserQuestionTool) IsReadOnly() bool { return true }

// Source: AskUserQuestionTool.tsx:14-24
func (t *AskUserQuestionTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"question": {
				"type": "string",
				"description": "The complete question to ask the user. Should be clear, specific, and end with a question mark."
			},
			"header": {
				"type": "string",
				"description": "Very short label displayed as a chip/tag (max 12 chars). Examples: \"Auth method\", \"Library\", \"Approach\"."
			},
			"options": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"label": {"type": "string", "description": "Display text for this option (1-5 words)"},
						"description": {"type": "string", "description": "Explanation of what this option means"},
						"preview": {"type": "string", "description": "Optional preview content (markdown for mockups, code snippets)"}
					},
					"required": ["label", "description"]
				},
				"minItems": 2,
				"maxItems": 4,
				"description": "Available choices (2-4 options). No 'Other' option needed — provided automatically."
			},
			"multiSelect": {
				"type": "boolean",
				"description": "Allow multiple answers to be selected (default false)"
			}
		},
		"required": ["question", "options"],
		"additionalProperties": false
	}`)
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Question    string `json:"question"`
		Header      string `json:"header"`
		MultiSelect bool   `json:"multiSelect"`
		Options     []struct {
			Label       string `json:"label"`
			Description string `json:"description"`
			Preview     string `json:"preview,omitempty"`
		} `json:"options"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Question == "" {
		return ErrorOutput("question is required"), nil
	}

	// Format the question with options for display.
	// In interactive mode, the REPL/TUI would render this as a selection UI.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Question for user]: %s\n", params.Question))
	if len(params.Options) > 0 {
		sb.WriteString("\nOptions:\n")
		for i, opt := range params.Options {
			sb.WriteString(fmt.Sprintf("  %d. %s — %s\n", i+1, opt.Label, opt.Description))
		}
		sb.WriteString("  (or type a custom response)\n")
	}
	if params.MultiSelect {
		sb.WriteString("\n(Multiple selections allowed)")
	}

	return SuccessOutput(sb.String()), nil
}
