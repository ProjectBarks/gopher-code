package hooks

import (
	"github.com/projectbarks/gopher-code/pkg/permissions"
)

// ToolEntry represents a tool in the merged pool. This is a lightweight
// descriptor used for pool assembly; the full tools.Tool interface is
// resolved at execution time.
// Source: hooks/useMergedTools.ts + utils/toolPool.ts
type ToolEntry struct {
	Name     string
	Source   ToolSource
	ReadOnly bool
}

// ToolSource identifies the origin of a tool.
type ToolSource string

const (
	ToolSourceBuiltIn ToolSource = "builtin"
	ToolSourceMCP     ToolSource = "mcp"
	ToolSourcePlugin  ToolSource = "plugin"
	ToolSourceMCPCLI  ToolSource = "mcp-cli"
)

// DenyRule prevents a tool from appearing in the merged pool.
type DenyRule struct {
	ToolName string
	Reason   string
}

// MergedTools assembles and caches the full tool pool from all sources.
// Source: hooks/useMergedTools.ts
type MergedTools struct {
	initialTools []ToolEntry
	mcpTools     []ToolEntry
	denyRules    []DenyRule
	mode         permissions.PermissionMode

	// cached is the last computed merged pool.
	cached []ToolEntry
}

// NewMergedTools creates a tool pool assembler.
func NewMergedTools(mode permissions.PermissionMode) *MergedTools {
	return &MergedTools{
		mode: mode,
	}
}

// SetInitialTools sets the base tools (built-in + startup MCP from props).
func (m *MergedTools) SetInitialTools(tools []ToolEntry) {
	m.initialTools = tools
	m.cached = nil // invalidate
}

// SetMCPTools sets the dynamically discovered MCP tools.
func (m *MergedTools) SetMCPTools(tools []ToolEntry) {
	m.mcpTools = tools
	m.cached = nil
}

// SetDenyRules sets the deny rules from permission context.
func (m *MergedTools) SetDenyRules(rules []DenyRule) {
	m.denyRules = rules
	m.cached = nil
}

// SetMode updates the permission mode filter.
func (m *MergedTools) SetMode(mode permissions.PermissionMode) {
	if m.mode != mode {
		m.mode = mode
		m.cached = nil
	}
}

// Tools returns the merged, filtered, deduplicated tool list.
// The result is cached until inputs change.
func (m *MergedTools) Tools() []ToolEntry {
	if m.cached != nil {
		return m.cached
	}
	m.cached = m.assemble()
	return m.cached
}

// assemble builds the merged pool: assembleToolPool + mergeAndFilter.
func (m *MergedTools) assemble() []ToolEntry {
	// Step 1: assemble pool (MCP tools filtered by deny rules, MCP-CLI excluded).
	assembled := assembleToolPool(m.mcpTools, m.denyRules)

	// Step 2: merge initial tools on top (initial takes precedence in dedup).
	return mergeAndFilterTools(m.initialTools, assembled, m.mode)
}

// assembleToolPool combines MCP tools, applies deny rules, and excludes MCP-CLI tools.
// Source: tools.ts assembleToolPool
func assembleToolPool(mcpTools []ToolEntry, denyRules []DenyRule) []ToolEntry {
	denied := make(map[string]bool, len(denyRules))
	for _, r := range denyRules {
		denied[r.ToolName] = true
	}

	var result []ToolEntry
	for _, t := range mcpTools {
		if denied[t.Name] {
			continue
		}
		// Exclude MCP-CLI tools (separate invocation path).
		if t.Source == ToolSourceMCPCLI {
			continue
		}
		result = append(result, t)
	}
	return result
}

// mergeAndFilterTools merges initialTools on top of assembled, deduplicating
// by name (initial wins) and filtering by permission mode.
// Source: utils/toolPool.ts mergeAndFilterTools
func mergeAndFilterTools(initial, assembled []ToolEntry, mode permissions.PermissionMode) []ToolEntry {
	seen := make(map[string]bool, len(initial)+len(assembled))
	var result []ToolEntry

	// Initial tools take precedence.
	for _, t := range initial {
		if !shouldIncludeTool(t, mode) {
			continue
		}
		if !seen[t.Name] {
			seen[t.Name] = true
			result = append(result, t)
		}
	}

	for _, t := range assembled {
		if !shouldIncludeTool(t, mode) {
			continue
		}
		if !seen[t.Name] {
			seen[t.Name] = true
			result = append(result, t)
		}
	}
	return result
}

// shouldIncludeTool filters tools based on permission mode.
// In plan mode, only read-only tools are included.
func shouldIncludeTool(t ToolEntry, mode permissions.PermissionMode) bool {
	if mode == permissions.ModePlan && !t.ReadOnly {
		return false
	}
	return true
}
