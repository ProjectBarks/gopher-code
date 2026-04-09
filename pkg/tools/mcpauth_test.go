package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

type mockAuthRunner struct {
	authURL string
	err     error
}

func (m *mockAuthRunner) StartOAuthFlow(_ context.Context, _ string) (string, error) {
	return m.authURL, m.err
}

func TestCreateMcpAuthTool(t *testing.T) {
	tool := tools.CreateMcpAuthTool("myserver", "sse", "https://example.com/mcp", nil)

	if tool.Name() != "mcp__myserver__authenticate" {
		t.Errorf("name = %q", tool.Name())
	}
	if !strings.Contains(tool.Description(), "myserver") {
		t.Error("description should mention server name")
	}
	if !strings.Contains(tool.Description(), "requires authentication") {
		t.Error("description should mention auth requirement")
	}
	if tool.IsReadOnly() {
		t.Error("auth tool should not be read-only")
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("invalid schema: %v", err)
	}
}

func TestMcpAuthTool_ClaudeAIProxy(t *testing.T) {
	tool := tools.CreateMcpAuthTool("connector", "claudeai-proxy", "", nil)
	out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Error("claudeai-proxy should not be an error")
	}
	if !strings.Contains(out.Content, "/mcp") {
		t.Errorf("should suggest /mcp: %s", out.Content)
	}
}

func TestMcpAuthTool_UnsupportedTransport(t *testing.T) {
	for _, transport := range []string{"stdio", "ws"} {
		t.Run(transport, func(t *testing.T) {
			tool := tools.CreateMcpAuthTool("srv", transport, "", nil)
			out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			if out.IsError {
				t.Error("unsupported should not be an error")
			}
			if !strings.Contains(out.Content, "does not support OAuth") {
				t.Errorf("should mention OAuth unsupported: %s", out.Content)
			}
		})
	}
}

func TestMcpAuthTool_NoAuthRunner(t *testing.T) {
	tool := tools.CreateMcpAuthTool("srv", "sse", "https://example.com", nil)
	out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !out.IsError {
		t.Error("should be error when no auth runner")
	}
	if !strings.Contains(out.Content, "not available") {
		t.Errorf("got %q", out.Content)
	}
}

func TestMcpAuthTool_OAuthFlowReturnsURL(t *testing.T) {
	runner := &mockAuthRunner{authURL: "https://auth.example.com/authorize?code=abc"}
	tool := tools.CreateMcpAuthTool("myserver", "http", "https://api.example.com", runner)
	out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}
	if !strings.Contains(out.Content, "https://auth.example.com/authorize") {
		t.Errorf("should contain auth URL: %s", out.Content)
	}
	if !strings.Contains(out.Content, "open this URL") {
		t.Error("should instruct user to open URL")
	}
}

func TestMcpAuthTool_SilentAuth(t *testing.T) {
	runner := &mockAuthRunner{authURL: ""} // empty URL = silent auth
	tool := tools.CreateMcpAuthTool("myserver", "sse", "https://example.com", runner)
	out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.IsError {
		t.Error("silent auth should not be error")
	}
	if !strings.Contains(out.Content, "completed silently") {
		t.Errorf("should mention silent completion: %s", out.Content)
	}
}

func TestMcpAuthTool_OAuthFlowError(t *testing.T) {
	runner := &mockAuthRunner{err: fmt.Errorf("network timeout")}
	tool := tools.CreateMcpAuthTool("myserver", "http", "https://example.com", runner)
	out, err := tool.Execute(context.Background(), nil, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !out.IsError {
		t.Error("should be error when OAuth flow fails")
	}
	if !strings.Contains(out.Content, "network timeout") {
		t.Errorf("should contain error: %s", out.Content)
	}
}
