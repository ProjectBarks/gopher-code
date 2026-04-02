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
			registry.Register(&MCPTool{client: client, info: info, serverName: name})
		}
	}
	return nil
}

// CloseAll shuts down all MCP server connections.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.Close()
	}
}
