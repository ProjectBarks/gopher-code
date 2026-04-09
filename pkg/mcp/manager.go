package mcp

import (
	"context"
	"sync"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// Manager manages multiple MCP server connections and registers their tools.
type Manager struct {
	clients map[string]*MCPClient
	mu      sync.Mutex
}

// NewManager creates an empty MCP Manager.
func NewManager() *Manager {
	return &Manager{clients: make(map[string]*MCPClient)}
}

// Connect starts an MCP server and stores the client connection.
func (m *Manager) Connect(ctx context.Context, name string, cfg ServerConfig) error {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()
	return nil
}

// RegisterTools discovers tools from all connected servers and registers them
// in the provided tool registry.
func (m *Manager) RegisterTools(ctx context.Context, registry *tools.ToolRegistry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, client := range m.clients {
		toolList, err := client.ListTools(ctx)
		if err != nil {
			return err
		}
		for _, info := range toolList {
			registry.Register(NewMCPTool(client, name, info))
		}
	}
	return nil
}

// ResourceClient returns a tools.MCPResourceClient for the named server, or nil.
func (m *Manager) ResourceClient(name string) tools.MCPResourceClient {
	m.mu.Lock()
	client, ok := m.clients[name]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return &mcpResourceAdapter{client: client, serverName: name}
}

// mcpResourceAdapter wraps MCPClient to implement tools.MCPResourceClient.
type mcpResourceAdapter struct {
	client     *MCPClient
	serverName string
}

func (a *mcpResourceAdapter) ListResources(ctx context.Context) ([]tools.MCPResourceInfo, error) {
	resources, err := a.client.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]tools.MCPResourceInfo, len(resources))
	for i, r := range resources {
		result[i] = tools.MCPResourceInfo{
			URI:         r.URI,
			Name:        r.Name,
			MimeType:    r.MimeType,
			Description: r.Description,
			Server:      a.serverName,
		}
	}
	return result, nil
}

func (a *mcpResourceAdapter) ReadResource(ctx context.Context, uri string) (*tools.MCPResourceResult, error) {
	result, err := a.client.ReadResource(ctx, uri)
	if err != nil {
		return nil, err
	}
	contents := make([]tools.MCPResourceContent, len(result.Contents))
	for i, c := range result.Contents {
		contents[i] = tools.MCPResourceContent{
			URI:      c.URI,
			MimeType: c.MimeType,
			Text:     c.Text,
		}
	}
	return &tools.MCPResourceResult{Contents: contents}, nil
}

// CloseAll shuts down all MCP server connections.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.Close()
	}
}
