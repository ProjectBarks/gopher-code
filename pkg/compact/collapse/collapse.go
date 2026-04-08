// Package collapse provides content collapse pipelines for the TUI.
// Consecutive read/search/glob tool calls are grouped into summary lines
// to keep the conversation display compact.
//
// Source: utils/collapseReadSearch.ts, collapseHookSummaries.ts,
//         collapseTeammateShutdowns.ts, collapseBackgroundBashNotifications.ts
package collapse

import (
	"fmt"
	"strings"
)

// ToolCategory classifies a tool call for collapse grouping.
type ToolCategory int

const (
	CategoryNone        ToolCategory = iota
	CategorySearch                   // Grep, Glob
	CategoryRead                     // Read, FileRead
	CategoryList                     // LS
	CategoryREPL                     // REPL
	CategoryMemoryWrite              // Edit/Write targeting memory files
	CategoryBash                     // Bash (non-search)
	CategoryAbsorbed                 // ToolSearch, Snip (silent)
	CategoryMCP                      // MCP tool
)

// ClassifyResult holds the classification of a tool call.
// Source: collapseReadSearch.ts — SearchOrReadResult
type ClassifyResult struct {
	Category      ToolCategory
	IsCollapsible bool
	MCPServerName string
}

// ClassifyTool determines whether a tool call is collapsible and its category.
// Source: collapseReadSearch.ts — getToolSearchOrReadInfo
func ClassifyTool(toolName string, inputPath string) ClassifyResult {
	switch toolName {
	case "Grep":
		return ClassifyResult{Category: CategorySearch, IsCollapsible: true}
	case "Glob":
		return ClassifyResult{Category: CategorySearch, IsCollapsible: true}
	case "Read":
		return ClassifyResult{Category: CategoryRead, IsCollapsible: true}
	case "LS", "ListDirectory":
		return ClassifyResult{Category: CategoryList, IsCollapsible: true}
	case "ToolSearch":
		return ClassifyResult{Category: CategoryAbsorbed, IsCollapsible: true}
	}

	// MCP tools (mcp__{server}__{tool})
	if strings.HasPrefix(toolName, "mcp__") {
		parts := strings.SplitN(toolName, "__", 3)
		server := ""
		if len(parts) >= 2 {
			server = parts[1]
		}
		return ClassifyResult{Category: CategoryMCP, IsCollapsible: true, MCPServerName: server}
	}

	return ClassifyResult{Category: CategoryNone, IsCollapsible: false}
}

// Group represents a collapsed group of consecutive tool calls.
// Source: types/message.ts — CollapsedReadSearchGroup
type Group struct {
	SearchCount      int
	ReadCount        int
	ListCount        int
	REPLCount        int
	MemoryWriteCount int
	MCPCounts        map[string]int // server → count
	ToolUseIDs       []string
	InProgress       bool
}

// NewGroup creates an empty collapse group.
func NewGroup() *Group {
	return &Group{MCPCounts: make(map[string]int)}
}

// Add records a tool call into the group.
func (g *Group) Add(result ClassifyResult, toolUseID string) {
	g.ToolUseIDs = append(g.ToolUseIDs, toolUseID)
	switch result.Category {
	case CategorySearch:
		g.SearchCount++
	case CategoryRead:
		g.ReadCount++
	case CategoryList:
		g.ListCount++
	case CategoryREPL:
		g.REPLCount++
	case CategoryMemoryWrite:
		g.MemoryWriteCount++
	case CategoryMCP:
		g.MCPCounts[result.MCPServerName]++
	case CategoryAbsorbed:
		// Don't increment any count
	}
}

// IsEmpty returns true if no tools have been added.
func (g *Group) IsEmpty() bool {
	return len(g.ToolUseIDs) == 0
}

// TotalCount returns the total number of non-absorbed tool calls.
func (g *Group) TotalCount() int {
	total := g.SearchCount + g.ReadCount + g.ListCount + g.REPLCount + g.MemoryWriteCount
	for _, c := range g.MCPCounts {
		total += c
	}
	return total
}

// SummaryText returns a human-readable summary of the group.
// Source: collapseReadSearch.ts — getSearchReadSummaryText
func (g *Group) SummaryText() string {
	var parts []string
	if g.ReadCount > 0 {
		parts = append(parts, pluralize(g.ReadCount, "file read", "file reads"))
	}
	if g.SearchCount > 0 {
		parts = append(parts, pluralize(g.SearchCount, "search", "searches"))
	}
	if g.ListCount > 0 {
		parts = append(parts, pluralize(g.ListCount, "directory listing", "directory listings"))
	}
	if g.REPLCount > 0 {
		parts = append(parts, pluralize(g.REPLCount, "REPL execution", "REPL executions"))
	}
	if g.MemoryWriteCount > 0 {
		parts = append(parts, pluralize(g.MemoryWriteCount, "memory write", "memory writes"))
	}
	for server, count := range g.MCPCounts {
		parts = append(parts, fmt.Sprintf("%d %s call%s", count, server, plural(count)))
	}

	if len(parts) == 0 {
		return ""
	}

	summary := strings.Join(parts, ", ")
	if g.InProgress {
		return summary + "…"
	}
	return summary
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
