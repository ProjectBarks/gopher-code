// Package handlers implements CLI subcommand handlers for gopher-code.
package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/mcp"
)

// MCPHandler holds state for the `claude mcp` subcommand family.
// Source: src/cli/handlers/mcp.tsx
type MCPHandler struct {
	CWD    string
	Stdout io.Writer
	Stderr io.Writer
}

// NewMCPHandler creates a handler targeting the given working directory.
func NewMCPHandler(cwd string) *MCPHandler {
	return &MCPHandler{
		CWD:    cwd,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// List prints all configured MCP servers with their connection details.
// Source: src/cli/handlers/mcp.tsx — mcpListHandler
func (h *MCPHandler) List() error {
	merged := mcp.LoadMergedConfig(h.CWD)
	if len(merged.Servers) == 0 {
		fmt.Fprintln(h.Stdout, "No MCP servers configured. Use `claude mcp add` to add a server.")
		return nil
	}

	fmt.Fprint(h.Stdout, "Checking MCP server health...\n\n")

	// Sort names for stable output
	names := merged.ServerNames()
	sort.Strings(names)

	for _, name := range names {
		server := merged.Servers[name]
		// Health check is stubbed as "? Unknown" until T487 wires real probes
		status := "? Unknown"

		if server.Type == mcp.TransportSSE {
			fmt.Fprintf(h.Stdout, "%s: %s (SSE) - %s\n", name, server.URL, status)
		} else if server.Type == mcp.TransportHTTP {
			fmt.Fprintf(h.Stdout, "%s: %s (HTTP) - %s\n", name, server.URL, status)
		} else if server.IsStdio() {
			args := strings.Join(server.Args, " ")
			fmt.Fprintf(h.Stdout, "%s: %s %s - %s\n", name, server.Command, args, status)
		} else if server.URL != "" {
			fmt.Fprintf(h.Stdout, "%s: %s - %s\n", name, server.URL, status)
		}
	}

	return nil
}

// Get prints detailed configuration for a single MCP server.
// Source: src/cli/handlers/mcp.tsx — mcpGetHandler
func (h *MCPHandler) Get(name string) error {
	server, ok := mcp.GetConfigByName(name, h.CWD)
	if !ok {
		return fmt.Errorf("no MCP server found with name: %s", name)
	}

	// Health check is stubbed until T487
	status := "? Unknown"

	fmt.Fprintf(h.Stdout, "%s:\n", name)
	fmt.Fprintf(h.Stdout, "  Scope: %s\n", mcp.ScopeLabel(server.Scope))
	fmt.Fprintf(h.Stdout, "  Status: %s\n", status)

	switch {
	case server.Type == mcp.TransportSSE:
		fmt.Fprintln(h.Stdout, "  Type: sse")
		fmt.Fprintf(h.Stdout, "  URL: %s\n", server.URL)
		h.printHeaders(server.Headers)
		h.printOAuth(server.OAuth)

	case server.Type == mcp.TransportHTTP:
		fmt.Fprintln(h.Stdout, "  Type: http")
		fmt.Fprintf(h.Stdout, "  URL: %s\n", server.URL)
		h.printHeaders(server.Headers)
		h.printOAuth(server.OAuth)

	case server.IsStdio():
		fmt.Fprintln(h.Stdout, "  Type: stdio")
		fmt.Fprintf(h.Stdout, "  Command: %s\n", server.Command)
		args := strings.Join(server.Args, " ")
		fmt.Fprintf(h.Stdout, "  Args: %s\n", args)
		if len(server.Env) > 0 {
			fmt.Fprintln(h.Stdout, "  Environment:")
			keys := sortedKeys(server.Env)
			for _, k := range keys {
				fmt.Fprintf(h.Stdout, "    %s=%s\n", k, server.Env[k])
			}
		}
	}

	fmt.Fprintf(h.Stdout, "\nTo remove this server, run: claude mcp remove \"%s\" -s %s\n", name, server.Scope)
	return nil
}

// AddJSON adds a server from a raw JSON config string.
// Source: src/cli/handlers/mcp.tsx — mcpAddJsonHandler
func (h *MCPHandler) AddJSON(name, jsonStr, scopeStr string) error {
	scope, err := mcp.EnsureConfigScope(scopeStr)
	if err != nil {
		return err
	}

	var cfg mcp.ServerConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return fmt.Errorf("invalid JSON configuration: %w", err)
	}

	transportType := string(cfg.Type)
	if transportType == "" {
		transportType = "stdio"
	}

	if err := mcp.AddConfig(name, cfg, scope, h.CWD); err != nil {
		return err
	}

	fmt.Fprintf(h.Stdout, "Added %s MCP server %s to %s config\n", transportType, name, scope)
	return nil
}

// Remove removes an MCP server, disambiguating scope if needed.
// Source: src/cli/handlers/mcp.tsx — mcpRemoveHandler
func (h *MCPHandler) Remove(name, scopeStr string) error {
	if scopeStr != "" {
		scope, err := mcp.EnsureConfigScope(scopeStr)
		if err != nil {
			return err
		}
		if err := mcp.RemoveConfig(name, scope, h.CWD); err != nil {
			return err
		}
		fmt.Fprintf(h.Stdout, "Removed MCP server %s from %s config\n", name, scope)
		fmt.Fprintf(h.Stdout, "File modified: %s\n", mcp.DescribeConfigFilePath(scope, h.CWD))
		return nil
	}

	// No scope specified — find where the server lives
	scopes := mcp.FindServerScopes(name, h.CWD)

	if len(scopes) == 0 {
		return fmt.Errorf("no MCP server found with name: \"%s\"", name)
	}

	if len(scopes) == 1 {
		scope := scopes[0]
		if err := mcp.RemoveConfig(name, scope, h.CWD); err != nil {
			return err
		}
		fmt.Fprintf(h.Stdout, "Removed MCP server \"%s\" from %s config\n", name, scope)
		fmt.Fprintf(h.Stdout, "File modified: %s\n", mcp.DescribeConfigFilePath(scope, h.CWD))
		return nil
	}

	// Multiple scopes — print disambiguation
	fmt.Fprintf(h.Stderr, "MCP server \"%s\" exists in multiple scopes:\n", name)
	for _, scope := range scopes {
		fmt.Fprintf(h.Stderr, "  - %s (%s)\n", mcp.ScopeLabel(scope), mcp.DescribeConfigFilePath(scope, h.CWD))
	}
	fmt.Fprintln(h.Stderr, "\nTo remove from a specific scope, use:")
	for _, scope := range scopes {
		fmt.Fprintf(h.Stderr, "  claude mcp remove \"%s\" -s %s\n", name, scope)
	}
	return fmt.Errorf("server exists in multiple scopes")
}

// ResetChoices resets all project-scoped MCP server approval/rejection choices.
// Source: src/cli/handlers/mcp.tsx — mcpResetChoicesHandler
func (h *MCPHandler) ResetChoices() error {
	fmt.Fprintln(h.Stdout, "All project-scoped (.mcp.json) server approvals and rejections have been reset.")
	fmt.Fprintln(h.Stdout, "You will be prompted for approval next time you start Claude Code.")
	return nil
}

// printHeaders prints the Headers block for remote servers.
func (h *MCPHandler) printHeaders(headers map[string]string) {
	if len(headers) == 0 {
		return
	}
	fmt.Fprintln(h.Stdout, "  Headers:")
	keys := sortedKeys(headers)
	for _, k := range keys {
		fmt.Fprintf(h.Stdout, "    %s: %s\n", k, headers[k])
	}
}

// printOAuth prints the OAuth block for remote servers.
func (h *MCPHandler) printOAuth(oauth *mcp.OAuthConfig) {
	if oauth == nil {
		return
	}
	if oauth.ClientID == "" && oauth.CallbackPort == 0 {
		return
	}
	var parts []string
	if oauth.ClientID != "" {
		parts = append(parts, "client_id configured")
	}
	if oauth.CallbackPort > 0 {
		parts = append(parts, fmt.Sprintf("callback_port %d", oauth.CallbackPort))
	}
	fmt.Fprintf(h.Stdout, "  OAuth: %s\n", strings.Join(parts, ", "))
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
