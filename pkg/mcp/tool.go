package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// MCPTool wraps an MCP server tool so it can be used as a tools.Tool.
type MCPTool struct {
	client     *MCPClient
	info       ToolInfo
	serverName string
}

// Name returns the fully-qualified tool name: serverName__toolName.
func (t *MCPTool) Name() string { return t.serverName + "__" + t.info.Name }

// Description returns the tool's description from the MCP server.
func (t *MCPTool) Description() string { return t.info.Description }

// InputSchema returns the JSON Schema for the tool's input.
func (t *MCPTool) InputSchema() json.RawMessage { return t.info.InputSchema }

// IsReadOnly returns true as a conservative default for MCP tools.
func (t *MCPTool) IsReadOnly() bool { return true }

// Execute calls the tool on the MCP server and returns the result.
func (t *MCPTool) Execute(ctx context.Context, tc *tools.ToolContext, input json.RawMessage) (*tools.ToolOutput, error) {
	result, err := t.client.CallTool(ctx, t.info.Name, input)
	if err != nil {
		return tools.ErrorOutput("MCP call failed: " + err.Error()), nil
	}

	// Parse MCP tool call result content
	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &callResult); err != nil {
		// If we can't parse structured content, return raw result
		return tools.SuccessOutput(string(result)), nil
	}

	var text strings.Builder
	for _, c := range callResult.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	if callResult.IsError {
		return tools.ErrorOutput(text.String()), nil
	}
	return tools.SuccessOutput(text.String()), nil
}
