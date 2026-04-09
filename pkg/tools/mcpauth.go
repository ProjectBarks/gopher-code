package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Source: tools/McpAuthTool/McpAuthTool.ts
//
// In TS, createMcpAuthTool is a factory that creates a per-server pseudo-tool
// for OAuth authentication. When an MCP server needs auth, this tool replaces
// its real tools so the model can trigger the OAuth flow.

// McpAuthOutput describes the result of an auth attempt.
type McpAuthOutput struct {
	Status  string `json:"status"`  // "auth_url", "unsupported", "error"
	Message string `json:"message"`
	AuthURL string `json:"authUrl,omitempty"`
}

// MCPAuthFlowRunner abstracts the OAuth flow for testability.
type MCPAuthFlowRunner interface {
	// StartOAuthFlow begins the OAuth flow for a server and returns the auth URL.
	// The flow completes asynchronously; the URL is returned for the user to open.
	StartOAuthFlow(ctx context.Context, serverName string) (authURL string, err error)
}

// McpAuthTool is a per-server pseudo-tool that triggers OAuth authentication.
// Created by CreateMcpAuthTool for each unauthenticated MCP server.
// Source: McpAuthTool.ts:49-215
type McpAuthTool struct {
	serverName  string
	transport   string // "stdio", "sse", "http", "ws", "claudeai-proxy"
	serverURL   string // URL for remote servers
	description string
	authRunner  MCPAuthFlowRunner // nil if no OAuth flow available
}

// CreateMcpAuthTool creates a pseudo-tool for an unauthenticated MCP server.
// The tool name follows the mcp__<server>__authenticate convention.
// Source: McpAuthTool.ts:49-52
func CreateMcpAuthTool(serverName, transport, serverURL string, authRunner MCPAuthFlowRunner) *McpAuthTool {
	location := transport
	if serverURL != "" {
		location = fmt.Sprintf("%s at %s", transport, serverURL)
	}

	desc := fmt.Sprintf(
		"The `%s` MCP server (%s) is installed but requires authentication. "+
			"Call this tool to start the OAuth flow — you'll receive an authorization URL "+
			"to share with the user. Once they complete the flow, the server's tools will "+
			"become available automatically.",
		serverName, location,
	)

	return &McpAuthTool{
		serverName:  serverName,
		transport:   transport,
		serverURL:   serverURL,
		description: desc,
		authRunner:  authRunner,
	}
}

func (t *McpAuthTool) Name() string {
	return fmt.Sprintf("mcp__%s__authenticate", t.serverName)
}

func (t *McpAuthTool) Description() string { return t.description }
func (t *McpAuthTool) IsReadOnly() bool    { return false }

func (t *McpAuthTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": [],
		"additionalProperties": false
	}`)
}

// Execute starts the OAuth flow for the server.
// Source: McpAuthTool.ts:85-205
func (t *McpAuthTool) Execute(ctx context.Context, _ *ToolContext, _ json.RawMessage) (*ToolOutput, error) {
	// claudeai-proxy uses a separate auth flow via /mcp command
	if t.transport == "claudeai-proxy" {
		out := McpAuthOutput{
			Status:  "unsupported",
			Message: fmt.Sprintf("This is a claude.ai MCP connector. Ask the user to run /mcp and select %q to authenticate.", t.serverName),
		}
		return t.outputFromAuth(out), nil
	}

	// OAuth only works for sse/http transports
	if t.transport != "sse" && t.transport != "http" {
		out := McpAuthOutput{
			Status:  "unsupported",
			Message: fmt.Sprintf("Server %q uses %s transport which does not support OAuth from this tool. Ask the user to run /mcp and authenticate manually.", t.serverName, t.transport),
		}
		return t.outputFromAuth(out), nil
	}

	// No auth runner configured
	if t.authRunner == nil {
		out := McpAuthOutput{
			Status:  "error",
			Message: fmt.Sprintf("OAuth flow not available for %q. Ask the user to run /mcp and authenticate manually.", t.serverName),
		}
		return t.outputFromAuth(out), nil
	}

	// Start the OAuth flow
	authURL, err := t.authRunner.StartOAuthFlow(ctx, t.serverName)
	if err != nil {
		out := McpAuthOutput{
			Status:  "error",
			Message: fmt.Sprintf("Failed to start OAuth flow for %s: %s. Ask the user to run /mcp and authenticate manually.", t.serverName, err),
		}
		return t.outputFromAuth(out), nil
	}

	if authURL == "" {
		// Silent auth completed (e.g. cached IdP token)
		out := McpAuthOutput{
			Status:  "auth_url",
			Message: fmt.Sprintf("Authentication completed silently for %s. The server's tools should now be available.", t.serverName),
		}
		return t.outputFromAuth(out), nil
	}

	out := McpAuthOutput{
		Status:  "auth_url",
		AuthURL: authURL,
		Message: fmt.Sprintf("Ask the user to open this URL in their browser to authorize the %s MCP server:\n\n%s\n\nOnce they complete the flow, the server's tools will become available automatically.", t.serverName, authURL),
	}
	return t.outputFromAuth(out), nil
}

func (t *McpAuthTool) outputFromAuth(out McpAuthOutput) *ToolOutput {
	if out.Status == "error" {
		return ErrorOutput(out.Message)
	}
	return SuccessOutput(out.Message)
}
