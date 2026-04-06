package mcp

import (
	"regexp"
	"strings"
)

// Source: services/mcp/normalization.ts

// ClaudeAIServerPrefix is prepended to Claude.ai server names.
const ClaudeAIServerPrefix = "claude.ai "

// mcpNameInvalidChars matches characters NOT in [a-zA-Z0-9_-].
var mcpNameInvalidChars = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

// mcpConsecutiveUnderscores matches runs of 2+ underscores.
var mcpConsecutiveUnderscores = regexp.MustCompile(`_+`)

// NormalizeNameForMCP normalizes a server name to the API-safe pattern
// ^[a-zA-Z0-9_-]{1,64}$. Invalid characters are replaced with underscores.
//
// For Claude.ai servers (names starting with "claude.ai "), consecutive
// underscores are collapsed and leading/trailing underscores are stripped
// to prevent interference with the __ delimiter used in MCP tool names
// (mcp__{server}__{tool}).
//
// Source: services/mcp/normalization.ts:17-23
func NormalizeNameForMCP(name string) string {
	normalized := mcpNameInvalidChars.ReplaceAllString(name, "_")
	if strings.HasPrefix(name, ClaudeAIServerPrefix) {
		normalized = mcpConsecutiveUnderscores.ReplaceAllString(normalized, "_")
		normalized = strings.Trim(normalized, "_")
	}
	return normalized
}

// MCPToolPrefix returns the tool name prefix for a given server name:
// "mcp__{normalizedName}__".
func MCPToolPrefix(serverName string) string {
	return "mcp__" + NormalizeNameForMCP(serverName) + "__"
}

// IsMCPToolName returns true if the tool name starts with "mcp__".
func IsMCPToolName(toolName string) bool {
	return strings.HasPrefix(toolName, "mcp__")
}

// ParseMCPToolName extracts the server name and tool name from an MCP
// tool name of the form "mcp__{server}__{tool}". Returns ("", "", false)
// if the name doesn't match the expected format.
func ParseMCPToolName(toolName string) (serverName, tool string, ok bool) {
	if !strings.HasPrefix(toolName, "mcp__") {
		return "", "", false
	}
	rest := toolName[len("mcp__"):]
	idx := strings.Index(rest, "__")
	if idx < 0 {
		return "", "", false
	}
	return rest[:idx], rest[idx+2:], true
}
