package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Source: tools/ListMcpResourcesTool/ListMcpResourcesTool.ts
// Source: tools/ReadMcpResourceTool/ReadMcpResourceTool.ts

// ListMcpResourcesTool lists available MCP resources from connected servers.
type ListMcpResourcesTool struct{}

func (t *ListMcpResourcesTool) Name() string { return "ListMcpResources" }
func (t *ListMcpResourcesTool) Description() string {
	return "Lists available resources from configured MCP servers. Each resource object includes a 'server' field indicating which server it's from."
}
func (t *ListMcpResourcesTool) IsReadOnly() bool { return true }

func (t *ListMcpResourcesTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"server": {"type": "string", "description": "Optional server name to filter resources by"}
		},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *ListMcpResourcesTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Server string `json:"server"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
		}
	}

	if tc == nil || tc.MCP == nil {
		return SuccessOutput("No MCP resources configured"), nil
	}

	serverNames := tc.MCP.ServerNames()
	if len(serverNames) == 0 {
		return SuccessOutput("No MCP resources configured"), nil
	}

	// Filter by server name if specified
	if params.Server != "" {
		found := false
		for _, name := range serverNames {
			if name == params.Server {
				found = true
				break
			}
		}
		if !found {
			return ErrorOutput(fmt.Sprintf("Server %q not found. Available servers: %s",
				params.Server, strings.Join(serverNames, ", "))), nil
		}
		serverNames = []string{params.Server}
	}

	var allResources []MCPResourceInfo
	for _, name := range serverNames {
		client := tc.MCP.ResourceClient(name)
		if client == nil {
			continue
		}
		resources, err := client.ListResources(ctx)
		if err != nil {
			// One server's failure shouldn't sink the whole result
			continue
		}
		allResources = append(allResources, resources...)
	}

	if len(allResources) == 0 {
		return SuccessOutput("No resources found. MCP servers may still provide tools even if they have no resources."), nil
	}

	data, err := json.Marshal(allResources)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to marshal resources: %s", err)), nil
	}
	return SuccessOutput(string(data)), nil
}

// ReadMcpResourceTool reads a specific MCP resource by URI.
type ReadMcpResourceTool struct{}

func (t *ReadMcpResourceTool) Name() string { return "ReadMcpResource" }
func (t *ReadMcpResourceTool) Description() string {
	return "Read a specific MCP resource by URI from a named server."
}
func (t *ReadMcpResourceTool) IsReadOnly() bool { return true }

func (t *ReadMcpResourceTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"server": {"type": "string", "description": "The MCP server name"},
			"uri": {"type": "string", "description": "The resource URI to read"}
		},
		"required": ["server", "uri"],
		"additionalProperties": false
	}`)
}

func (t *ReadMcpResourceTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Server string `json:"server"`
		URI    string `json:"uri"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Server == "" {
		return ErrorOutput("server is required"), nil
	}
	if params.URI == "" {
		return ErrorOutput("uri is required"), nil
	}

	if tc == nil || tc.MCP == nil {
		return ErrorOutput(fmt.Sprintf("Server %q not found: no MCP servers are configured", params.Server)), nil
	}

	client := tc.MCP.ResourceClient(params.Server)
	if client == nil {
		serverNames := tc.MCP.ServerNames()
		return ErrorOutput(fmt.Sprintf("Server %q not found. Available servers: %s",
			params.Server, strings.Join(serverNames, ", "))), nil
	}

	result, err := client.ReadResource(ctx, params.URI)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to read resource: %s", err)), nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return ErrorOutput(fmt.Sprintf("failed to marshal result: %s", err)), nil
	}
	return SuccessOutput(string(data)), nil
}
