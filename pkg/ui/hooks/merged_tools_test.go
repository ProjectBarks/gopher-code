package hooks

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/permissions"
)

func TestMergedTools_EmptyInputs(t *testing.T) {
	m := NewMergedTools(permissions.ModeDefault)
	tools := m.Tools()
	if len(tools) != 0 {
		t.Fatalf("len = %d, want 0", len(tools))
	}
}

func TestMergedTools_InitialToolsOnly(t *testing.T) {
	m := NewMergedTools(permissions.ModeDefault)
	m.SetInitialTools([]ToolEntry{
		{Name: "bash", Source: ToolSourceBuiltIn},
		{Name: "read", Source: ToolSourceBuiltIn},
	})

	tools := m.Tools()
	if len(tools) != 2 {
		t.Fatalf("len = %d, want 2", len(tools))
	}
}

func TestMergedTools_MCPToolsMerged(t *testing.T) {
	m := NewMergedTools(permissions.ModeDefault)
	m.SetInitialTools([]ToolEntry{
		{Name: "bash", Source: ToolSourceBuiltIn},
	})
	m.SetMCPTools([]ToolEntry{
		{Name: "mcp-search", Source: ToolSourceMCP},
	})

	tools := m.Tools()
	if len(tools) != 2 {
		t.Fatalf("len = %d, want 2", len(tools))
	}
	names := map[string]bool{}
	for _, te := range tools {
		names[te.Name] = true
	}
	if !names["bash"] || !names["mcp-search"] {
		t.Fatalf("tools = %v, want bash and mcp-search", names)
	}
}

func TestMergedTools_InitialTakesPrecedence(t *testing.T) {
	m := NewMergedTools(permissions.ModeDefault)
	m.SetInitialTools([]ToolEntry{
		{Name: "search", Source: ToolSourceBuiltIn, ReadOnly: true},
	})
	m.SetMCPTools([]ToolEntry{
		{Name: "search", Source: ToolSourceMCP, ReadOnly: false},
	})

	tools := m.Tools()
	if len(tools) != 1 {
		t.Fatalf("len = %d, want 1 (dedup)", len(tools))
	}
	if tools[0].Source != ToolSourceBuiltIn {
		t.Fatalf("source = %q, want builtin (initial takes precedence)", tools[0].Source)
	}
}

func TestMergedTools_DenyRules(t *testing.T) {
	m := NewMergedTools(permissions.ModeDefault)
	m.SetMCPTools([]ToolEntry{
		{Name: "allowed", Source: ToolSourceMCP},
		{Name: "blocked", Source: ToolSourceMCP},
	})
	m.SetDenyRules([]DenyRule{
		{ToolName: "blocked", Reason: "test"},
	})

	tools := m.Tools()
	if len(tools) != 1 {
		t.Fatalf("len = %d, want 1", len(tools))
	}
	if tools[0].Name != "allowed" {
		t.Fatalf("tool = %q, want allowed", tools[0].Name)
	}
}

func TestMergedTools_MCPCLIExcluded(t *testing.T) {
	m := NewMergedTools(permissions.ModeDefault)
	m.SetMCPTools([]ToolEntry{
		{Name: "normal", Source: ToolSourceMCP},
		{Name: "cli-tool", Source: ToolSourceMCPCLI},
	})

	tools := m.Tools()
	if len(tools) != 1 {
		t.Fatalf("len = %d, want 1", len(tools))
	}
	if tools[0].Name != "normal" {
		t.Fatalf("tool = %q, want normal", tools[0].Name)
	}
}

func TestMergedTools_PlanModeFiltersNonReadOnly(t *testing.T) {
	m := NewMergedTools(permissions.ModePlan)
	m.SetInitialTools([]ToolEntry{
		{Name: "read-file", Source: ToolSourceBuiltIn, ReadOnly: true},
		{Name: "write-file", Source: ToolSourceBuiltIn, ReadOnly: false},
	})
	m.SetMCPTools([]ToolEntry{
		{Name: "mcp-read", Source: ToolSourceMCP, ReadOnly: true},
		{Name: "mcp-write", Source: ToolSourceMCP, ReadOnly: false},
	})

	tools := m.Tools()
	if len(tools) != 2 {
		t.Fatalf("len = %d, want 2", len(tools))
	}
	for _, te := range tools {
		if !te.ReadOnly {
			t.Fatalf("non-read-only tool %q in plan mode", te.Name)
		}
	}
}

func TestMergedTools_CacheInvalidation(t *testing.T) {
	m := NewMergedTools(permissions.ModeDefault)
	m.SetInitialTools([]ToolEntry{
		{Name: "a", Source: ToolSourceBuiltIn},
	})

	tools1 := m.Tools()
	tools2 := m.Tools()
	// Same pointer = cached.
	if &tools1[0] != &tools2[0] {
		t.Fatal("expected cached result")
	}

	// Invalidate by setting new MCP tools.
	m.SetMCPTools([]ToolEntry{
		{Name: "b", Source: ToolSourceMCP},
	})
	tools3 := m.Tools()
	if len(tools3) != 2 {
		t.Fatalf("len = %d after invalidation, want 2", len(tools3))
	}
}

func TestMergedTools_ModeChange(t *testing.T) {
	m := NewMergedTools(permissions.ModeDefault)
	m.SetInitialTools([]ToolEntry{
		{Name: "write", Source: ToolSourceBuiltIn, ReadOnly: false},
		{Name: "read", Source: ToolSourceBuiltIn, ReadOnly: true},
	})

	if len(m.Tools()) != 2 {
		t.Fatal("default mode should include all tools")
	}

	m.SetMode(permissions.ModePlan)
	if len(m.Tools()) != 1 {
		t.Fatal("plan mode should filter write tools")
	}
}
