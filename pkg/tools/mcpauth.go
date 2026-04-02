package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// McpAuthTool authenticates with an MCP server.
type McpAuthTool struct{}

func (t *McpAuthTool) Name() string        { return "McpAuth" }
func (t *McpAuthTool) Description() string { return "Authenticate with an MCP server" }
func (t *McpAuthTool) IsReadOnly() bool    { return true }

func (t *McpAuthTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"server_name": {"type": "string", "description": "MCP server to authenticate with"},
			"action": {"type": "string", "enum": ["login", "status", "logout"], "description": "Auth action"}
		},
		"required": ["server_name", "action"],
		"additionalProperties": false
	}`)
}

func (t *McpAuthTool) Execute(_ context.Context, _ *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		ServerName string `json:"server_name"`
		Action     string `json:"action"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput("invalid input: " + err.Error()), nil
	}
	switch params.Action {
	case "status":
		return SuccessOutput(fmt.Sprintf("MCP server '%s': not authenticated", params.ServerName)), nil
	case "login":
		return ErrorOutput("MCP OAuth authentication not yet implemented. Configure API keys in mcp.json instead."), nil
	case "logout":
		return SuccessOutput("Logged out from " + params.ServerName), nil
	}
	return ErrorOutput("unknown action: " + params.Action), nil
}
