package collapse

import (
	"strings"
	"testing"
)

func TestClassifyTool(t *testing.T) {
	tests := []struct {
		name string
		tool string
		want ToolCategory
	}{
		{"grep", "Grep", CategorySearch},
		{"glob", "Glob", CategorySearch},
		{"read", "Read", CategoryRead},
		{"ls", "LS", CategoryList},
		{"toolsearch", "ToolSearch", CategoryAbsorbed},
		{"bash", "Bash", CategoryNone},
		{"edit", "Edit", CategoryNone},
		{"mcp", "mcp__github__list_repos", CategoryMCP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyTool(tt.tool, "")
			if result.Category != tt.want {
				t.Errorf("ClassifyTool(%q) = %d, want %d", tt.tool, result.Category, tt.want)
			}
		})
	}
}

func TestClassifyTool_MCPServerName(t *testing.T) {
	result := ClassifyTool("mcp__github__create_pr", "")
	if result.MCPServerName != "github" {
		t.Errorf("MCPServerName = %q, want github", result.MCPServerName)
	}
	if !result.IsCollapsible {
		t.Error("MCP tools should be collapsible")
	}
}

func TestGroup_AddAndSummary(t *testing.T) {
	g := NewGroup()
	g.Add(ClassifyResult{Category: CategoryRead, IsCollapsible: true}, "id1")
	g.Add(ClassifyResult{Category: CategoryRead, IsCollapsible: true}, "id2")
	g.Add(ClassifyResult{Category: CategorySearch, IsCollapsible: true}, "id3")

	if g.TotalCount() != 3 {
		t.Errorf("TotalCount = %d, want 3", g.TotalCount())
	}
	if g.ReadCount != 2 {
		t.Errorf("ReadCount = %d, want 2", g.ReadCount)
	}

	summary := g.SummaryText()
	if !strings.Contains(summary, "2 file reads") {
		t.Errorf("summary should contain '2 file reads', got %q", summary)
	}
	if !strings.Contains(summary, "1 search") {
		t.Errorf("summary should contain '1 search', got %q", summary)
	}
}

func TestGroup_AbsorbedDoesNotCount(t *testing.T) {
	g := NewGroup()
	g.Add(ClassifyResult{Category: CategoryAbsorbed, IsCollapsible: true}, "id1")
	g.Add(ClassifyResult{Category: CategoryRead, IsCollapsible: true}, "id2")

	if g.TotalCount() != 1 {
		t.Errorf("TotalCount = %d, want 1 (absorbed should not count)", g.TotalCount())
	}
	if len(g.ToolUseIDs) != 2 {
		t.Errorf("ToolUseIDs should have 2 entries, got %d", len(g.ToolUseIDs))
	}
}

func TestGroup_InProgress(t *testing.T) {
	g := NewGroup()
	g.Add(ClassifyResult{Category: CategorySearch, IsCollapsible: true}, "id1")
	g.InProgress = true

	summary := g.SummaryText()
	if !strings.HasSuffix(summary, "…") {
		t.Errorf("in-progress summary should end with …, got %q", summary)
	}
}

func TestGroup_Empty(t *testing.T) {
	g := NewGroup()
	if !g.IsEmpty() {
		t.Error("new group should be empty")
	}
	if g.SummaryText() != "" {
		t.Error("empty group summary should be empty string")
	}
}

func TestGroup_MCPCounts(t *testing.T) {
	g := NewGroup()
	g.Add(ClassifyResult{Category: CategoryMCP, MCPServerName: "github"}, "id1")
	g.Add(ClassifyResult{Category: CategoryMCP, MCPServerName: "github"}, "id2")
	g.Add(ClassifyResult{Category: CategoryMCP, MCPServerName: "slack"}, "id3")

	if g.TotalCount() != 3 {
		t.Errorf("TotalCount = %d, want 3", g.TotalCount())
	}
	summary := g.SummaryText()
	if !strings.Contains(summary, "github") || !strings.Contains(summary, "slack") {
		t.Errorf("summary should mention both servers, got %q", summary)
	}
}
