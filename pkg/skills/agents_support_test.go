package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAgentOverrides_Basic(t *testing.T) {
	all := []AgentDefinition{
		{AgentType: "Explore", Source: AgentSourceBuiltIn},
		{AgentType: "Explore", Source: AgentSourceProject}, // overrides built-in
		{AgentType: "Plan", Source: AgentSourceBuiltIn},
	}
	active := []AgentDefinition{
		{AgentType: "Explore", Source: AgentSourceProject},
		{AgentType: "Plan", Source: AgentSourceBuiltIn},
	}

	resolved := ResolveAgentOverrides(all, active)
	if len(resolved) != 3 {
		t.Fatalf("expected 3 resolved, got %d", len(resolved))
	}

	// Built-in Explore should be overridden by project
	if resolved[0].OverriddenBy != AgentSourceProject {
		t.Errorf("built-in Explore should be overridden by project, got %q", resolved[0].OverriddenBy)
	}
	// Project Explore is active — not overridden
	if resolved[1].OverriddenBy != "" {
		t.Errorf("project Explore should not be overridden, got %q", resolved[1].OverriddenBy)
	}
	// Plan is active built-in — not overridden
	if resolved[2].OverriddenBy != "" {
		t.Errorf("Plan should not be overridden, got %q", resolved[2].OverriddenBy)
	}
}

func TestResolveAgentOverrides_Dedup(t *testing.T) {
	// Duplicate from worktree
	all := []AgentDefinition{
		{AgentType: "custom", Source: AgentSourceProject},
		{AgentType: "custom", Source: AgentSourceProject}, // duplicate
	}
	active := []AgentDefinition{
		{AgentType: "custom", Source: AgentSourceProject},
	}
	resolved := ResolveAgentOverrides(all, active)
	if len(resolved) != 1 {
		t.Errorf("should deduplicate, got %d", len(resolved))
	}
}

func TestGetAgentMemoryDir(t *testing.T) {
	cwd := "/projects/my-app"

	userDir := GetAgentMemoryDir("Explore", AgentMemoryUser, cwd)
	home, _ := os.UserHomeDir()
	if userDir != filepath.Join(home, ".claude", "agent-memory", "Explore") {
		t.Errorf("user dir = %q", userDir)
	}

	projDir := GetAgentMemoryDir("Plan", AgentMemoryProject, cwd)
	if projDir != filepath.Join(cwd, ".claude", "agent-memory", "Plan") {
		t.Errorf("project dir = %q", projDir)
	}

	localDir := GetAgentMemoryDir("my-plugin:agent", AgentMemoryLocal, cwd)
	if localDir != filepath.Join(cwd, ".claude", "agent-memory-local", "my-plugin-agent") {
		t.Errorf("local dir = %q (colon should be replaced)", localDir)
	}
}

func TestAgentColorManager(t *testing.T) {
	m := NewAgentColorManager()

	c1 := m.AssignColor("Explore")
	c2 := m.AssignColor("Plan")
	c3 := m.AssignColor("Explore") // should be same as c1

	if c1 == "" {
		t.Error("color should not be empty")
	}
	if c1 == c2 {
		t.Error("different agents should get different colors")
	}
	if c1 != c3 {
		t.Error("same agent should get same color")
	}
}

func TestAgentSourceGroups(t *testing.T) {
	if len(AgentSourceGroups) == 0 {
		t.Error("AgentSourceGroups should not be empty")
	}
	// First should be user, last should be built-in
	if AgentSourceGroups[0].Source != AgentSourceUser {
		t.Errorf("first group should be user, got %q", AgentSourceGroups[0].Source)
	}
	if AgentSourceGroups[len(AgentSourceGroups)-1].Source != AgentSourceBuiltIn {
		t.Errorf("last group should be built-in, got %q", AgentSourceGroups[len(AgentSourceGroups)-1].Source)
	}
}
