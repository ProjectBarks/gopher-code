package mcp

import "testing"

// Source: services/mcp/normalization.ts

func TestNormalizeNameForMCP_SimpleNames(t *testing.T) {
	// Names with only valid chars pass through unchanged
	tests := []struct {
		input, want string
	}{
		{"my-server", "my-server"},
		{"my_server", "my_server"},
		{"MyServer123", "MyServer123"},
		{"a", "a"},
	}
	for _, tt := range tests {
		if got := NormalizeNameForMCP(tt.input); got != tt.want {
			t.Errorf("NormalizeNameForMCP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeNameForMCP_InvalidChars(t *testing.T) {
	// Dots, spaces, and other invalid chars become underscores
	tests := []struct {
		input, want string
	}{
		{"my.server", "my_server"},
		{"my server", "my_server"},
		{"my@server!", "my_server_"},
		{"a/b/c", "a_b_c"},
	}
	for _, tt := range tests {
		if got := NormalizeNameForMCP(tt.input); got != tt.want {
			t.Errorf("NormalizeNameForMCP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeNameForMCP_ClaudeAIPrefix(t *testing.T) {
	// Source: normalization.ts:19-21 — claude.ai servers get extra normalization:
	// consecutive underscores collapsed, leading/trailing stripped
	tests := []struct {
		input, want string
	}{
		// "claude.ai Gmail" -> "claude_ai_Gmail" (dots and space -> _, collapsed, trimmed)
		{"claude.ai Gmail", "claude_ai_Gmail"},
		// "claude.ai Google Calendar" -> "claude_ai_Google_Calendar"
		{"claude.ai Google Calendar", "claude_ai_Google_Calendar"},
		// Edge case: multiple dots/spaces in sequence
		{"claude.ai  ..test", "claude_ai_test"},
	}
	for _, tt := range tests {
		if got := NormalizeNameForMCP(tt.input); got != tt.want {
			t.Errorf("NormalizeNameForMCP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeNameForMCP_NonClaudeAIPreservesConsecutive(t *testing.T) {
	// Non-claude.ai servers do NOT collapse consecutive underscores
	got := NormalizeNameForMCP("foo..bar")
	want := "foo__bar"
	if got != want {
		t.Errorf("NormalizeNameForMCP(%q) = %q, want %q", "foo..bar", got, want)
	}
}

func TestMCPToolPrefix(t *testing.T) {
	got := MCPToolPrefix("my.server")
	want := "mcp__my_server__"
	if got != want {
		t.Errorf("MCPToolPrefix(%q) = %q, want %q", "my.server", got, want)
	}
}

func TestIsMCPToolName(t *testing.T) {
	if !IsMCPToolName("mcp__server__tool") {
		t.Error("should be MCP tool name")
	}
	if IsMCPToolName("BashTool") {
		t.Error("should not be MCP tool name")
	}
	if IsMCPToolName("") {
		t.Error("empty should not be MCP tool name")
	}
}

func TestParseMCPToolName(t *testing.T) {
	tests := []struct {
		input      string
		server     string
		tool       string
		ok         bool
	}{
		{"mcp__myserver__echo", "myserver", "echo", true},
		{"mcp__claude_ai_Gmail__send_email", "claude_ai_Gmail", "send_email", true},
		{"BashTool", "", "", false},
		{"mcp__nodelimiter", "", "", false},
		{"", "", "", false},
	}
	for _, tt := range tests {
		server, tool, ok := ParseMCPToolName(tt.input)
		if ok != tt.ok || server != tt.server || tool != tt.tool {
			t.Errorf("ParseMCPToolName(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, server, tool, ok, tt.server, tt.tool, tt.ok)
		}
	}
}
