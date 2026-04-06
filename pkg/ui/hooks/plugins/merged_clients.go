// Package plugins implements MCP/plugin hooks: merged MCP clients,
// plugin management state, and LSP plugin recommendation logic.
//
// Source: src/hooks/useMergedClients.ts, src/hooks/useManagePlugins.ts,
// src/hooks/useLspPluginRecommendation.tsx, src/hooks/usePluginRecommendationBase.tsx
package plugins

import (
	"github.com/projectbarks/gopher-code/pkg/mcp"
)

// MCPServerConnection represents a named MCP server with its scoped config.
// Mirrors MCPServerConnection from src/services/mcp/types.ts.
type MCPServerConnection struct {
	Name   string
	Config mcp.ScopedServerConfig
}

// MergeClients combines MCP servers from initial (config-based) and dynamic
// (plugin/managed) sources, deduplicating by name. The first occurrence wins
// (initial takes precedence over dynamic when names collide).
//
// Source: src/hooks/useMergedClients.ts — mergeClients()
func MergeClients(initial, dynamic []MCPServerConnection) []MCPServerConnection {
	if len(dynamic) == 0 {
		// Fast path: nothing to merge.
		if initial == nil {
			return []MCPServerConnection{}
		}
		return initial
	}

	seen := make(map[string]struct{}, len(initial)+len(dynamic))
	merged := make([]MCPServerConnection, 0, len(initial)+len(dynamic))

	for _, c := range initial {
		if _, dup := seen[c.Name]; !dup {
			seen[c.Name] = struct{}{}
			merged = append(merged, c)
		}
	}
	for _, c := range dynamic {
		if _, dup := seen[c.Name]; !dup {
			seen[c.Name] = struct{}{}
			merged = append(merged, c)
		}
	}
	return merged
}

// MergedClients holds all MCP servers from every source (user, project,
// local, managed/plugin). It is the Go equivalent of the React hook
// useMergedClients — callers update it imperatively instead of via
// useState/useMemo.
type MergedClients struct {
	initial []MCPServerConnection
	dynamic []MCPServerConnection
	merged  []MCPServerConnection
}

// NewMergedClients creates a MergedClients from the initial config-based
// servers. Dynamic (plugin) servers can be added later via SetDynamic.
func NewMergedClients(initial []MCPServerConnection) *MergedClients {
	mc := &MergedClients{initial: initial}
	mc.merged = MergeClients(initial, nil)
	return mc
}

// SetDynamic replaces the dynamic (plugin/managed) server list and
// recomputes the merged set.
func (mc *MergedClients) SetDynamic(dynamic []MCPServerConnection) {
	mc.dynamic = dynamic
	mc.merged = MergeClients(mc.initial, mc.dynamic)
}

// All returns the deduplicated list of MCP servers from all sources.
func (mc *MergedClients) All() []MCPServerConnection {
	return mc.merged
}

// Names returns the names of all merged servers.
func (mc *MergedClients) Names() []string {
	names := make([]string, len(mc.merged))
	for i, c := range mc.merged {
		names[i] = c.Name
	}
	return names
}
