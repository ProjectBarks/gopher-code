package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// mockMCPProvider implements tools.MCPClientProvider for testing.
type mockMCPProvider struct {
	servers map[string]*mockResourceClient
}

func (m *mockMCPProvider) ServerNames() []string {
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

func (m *mockMCPProvider) ResourceClient(name string) tools.MCPResourceClient {
	c, ok := m.servers[name]
	if !ok {
		return nil
	}
	return c
}

type mockResourceClient struct {
	resources []tools.MCPResourceInfo
	readFunc  func(ctx context.Context, uri string) (*tools.MCPResourceResult, error)
}

func (m *mockResourceClient) ListResources(_ context.Context) ([]tools.MCPResourceInfo, error) {
	return m.resources, nil
}

func (m *mockResourceClient) ReadResource(ctx context.Context, uri string) (*tools.MCPResourceResult, error) {
	if m.readFunc != nil {
		return m.readFunc(ctx, uri)
	}
	return nil, fmt.Errorf("resource %q not found", uri)
}

func TestListMcpResourcesTool(t *testing.T) {
	tool := &tools.ListMcpResourcesTool{}

	t.Run("name_and_schema", func(t *testing.T) {
		if tool.Name() != "ListMcpResources" {
			t.Errorf("name = %q", tool.Name())
		}
		if !tool.IsReadOnly() {
			t.Error("should be read-only")
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("invalid schema: %v", err)
		}
	})

	t.Run("no_mcp_configured", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), &tools.ToolContext{}, json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if out.IsError {
			t.Error("should not be error")
		}
		if !strings.Contains(out.Content, "No MCP resources configured") {
			t.Errorf("got %q", out.Content)
		}
	})

	t.Run("nil_context", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.Content, "No MCP resources configured") {
			t.Errorf("got %q", out.Content)
		}
	})

	t.Run("lists_resources_from_all_servers", func(t *testing.T) {
		provider := &mockMCPProvider{
			servers: map[string]*mockResourceClient{
				"server-a": {
					resources: []tools.MCPResourceInfo{
						{URI: "file:///a.txt", Name: "a.txt", Server: "server-a"},
					},
				},
				"server-b": {
					resources: []tools.MCPResourceInfo{
						{URI: "file:///b.txt", Name: "b.txt", Server: "server-b"},
					},
				},
			},
		}
		tc := &tools.ToolContext{MCP: provider}
		out, err := tool.Execute(context.Background(), tc, json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "a.txt") || !strings.Contains(out.Content, "b.txt") {
			t.Errorf("should contain both resources: %s", out.Content)
		}
	})

	t.Run("filter_by_server", func(t *testing.T) {
		provider := &mockMCPProvider{
			servers: map[string]*mockResourceClient{
				"server-a": {
					resources: []tools.MCPResourceInfo{
						{URI: "file:///a.txt", Name: "a.txt", Server: "server-a"},
					},
				},
				"server-b": {
					resources: []tools.MCPResourceInfo{
						{URI: "file:///b.txt", Name: "b.txt", Server: "server-b"},
					},
				},
			},
		}
		tc := &tools.ToolContext{MCP: provider}
		out, err := tool.Execute(context.Background(), tc, json.RawMessage(`{"server":"server-a"}`))
		if err != nil {
			t.Fatal(err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "a.txt") {
			t.Error("should contain server-a resource")
		}
		if strings.Contains(out.Content, "b.txt") {
			t.Error("should NOT contain server-b resource")
		}
	})

	t.Run("server_not_found", func(t *testing.T) {
		provider := &mockMCPProvider{
			servers: map[string]*mockResourceClient{
				"server-a": {resources: nil},
			},
		}
		tc := &tools.ToolContext{MCP: provider}
		out, err := tool.Execute(context.Background(), tc, json.RawMessage(`{"server":"nonexistent"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !out.IsError {
			t.Error("should be error for missing server")
		}
		if !strings.Contains(out.Content, "not found") {
			t.Errorf("got %q", out.Content)
		}
	})

	t.Run("no_resources_found", func(t *testing.T) {
		provider := &mockMCPProvider{
			servers: map[string]*mockResourceClient{
				"server-a": {resources: nil},
			},
		}
		tc := &tools.ToolContext{MCP: provider}
		out, err := tool.Execute(context.Background(), tc, json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if out.IsError {
			t.Error("no resources should not be an error")
		}
		if !strings.Contains(out.Content, "No resources found") {
			t.Errorf("got %q", out.Content)
		}
	})
}

func TestReadMcpResourceTool(t *testing.T) {
	tool := &tools.ReadMcpResourceTool{}

	t.Run("name_and_schema", func(t *testing.T) {
		if tool.Name() != "ReadMcpResource" {
			t.Errorf("name = %q", tool.Name())
		}
		if !tool.IsReadOnly() {
			t.Error("should be read-only")
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("invalid schema: %v", err)
		}
	})

	t.Run("reads_resource", func(t *testing.T) {
		provider := &mockMCPProvider{
			servers: map[string]*mockResourceClient{
				"myserver": {
					readFunc: func(_ context.Context, uri string) (*tools.MCPResourceResult, error) {
						return &tools.MCPResourceResult{
							Contents: []tools.MCPResourceContent{
								{URI: uri, MimeType: "text/plain", Text: "hello world"},
							},
						}, nil
					},
				},
			},
		}
		tc := &tools.ToolContext{MCP: provider}
		out, err := tool.Execute(context.Background(), tc, json.RawMessage(`{"server":"myserver","uri":"file:///test.txt"}`))
		if err != nil {
			t.Fatal(err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "hello world") {
			t.Errorf("should contain text: %s", out.Content)
		}
	})

	t.Run("server_not_found", func(t *testing.T) {
		provider := &mockMCPProvider{
			servers: map[string]*mockResourceClient{
				"other": {resources: nil},
			},
		}
		tc := &tools.ToolContext{MCP: provider}
		out, err := tool.Execute(context.Background(), tc, json.RawMessage(`{"server":"nonexistent","uri":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !out.IsError {
			t.Error("should be error for missing server")
		}
		if !strings.Contains(out.Content, "not found") {
			t.Errorf("got %q", out.Content)
		}
	})

	t.Run("missing_server_field", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), &tools.ToolContext{}, json.RawMessage(`{"uri":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !out.IsError {
			t.Error("should error when server missing")
		}
	})

	t.Run("missing_uri_field", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), &tools.ToolContext{}, json.RawMessage(`{"server":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !out.IsError {
			t.Error("should error when uri missing")
		}
	})

	t.Run("no_mcp_configured", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{"server":"x","uri":"y"}`))
		if err != nil {
			t.Fatal(err)
		}
		if !out.IsError {
			t.Error("should be error when no MCP")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatal(err)
		}
		if !out.IsError {
			t.Error("should error on bad JSON")
		}
	})
}
