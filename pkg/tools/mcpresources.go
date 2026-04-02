package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ListMcpResourcesTool lists available MCP resources.
type ListMcpResourcesTool struct{}

func (t *ListMcpResourcesTool) Name() string        { return "ListMcpResources" }
func (t *ListMcpResourcesTool) Description() string { return "List available MCP resources" }
func (t *ListMcpResourcesTool) IsReadOnly() bool    { return true }

func (t *ListMcpResourcesTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *ListMcpResourcesTool) Execute(_ context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	return SuccessOutput("No MCP resources configured"), nil
}

// ReadMcpResourceTool reads a specific MCP resource by URI.
type ReadMcpResourceTool struct{}

func (t *ReadMcpResourceTool) Name() string        { return "ReadMcpResource" }
func (t *ReadMcpResourceTool) Description() string { return "Read a specific MCP resource" }
func (t *ReadMcpResourceTool) IsReadOnly() bool    { return true }

func (t *ReadMcpResourceTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"uri": {"type": "string", "description": "The URI of the MCP resource to read"}
		},
		"required": ["uri"],
		"additionalProperties": false
	}`)
}

func (t *ReadMcpResourceTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.URI == "" {
		return ErrorOutput("uri is required"), nil
	}
	return ErrorOutput(fmt.Sprintf("MCP resource %q not found: no MCP resources are configured", params.URI)), nil
}
