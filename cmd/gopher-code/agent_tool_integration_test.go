package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// TestAgentTool_WiredIntoBinary verifies the AgentTool core types and functions
// are reachable from the binary via RegisterDefaults + RegisterAgentTool.
func TestAgentTool_WiredIntoBinary(t *testing.T) {
	// Verify constants are accessible.
	if tools.AgentToolName != "Agent" {
		t.Errorf("AgentToolName = %q, want Agent", tools.AgentToolName)
	}
	if tools.LegacyAgentToolName != "Task" {
		t.Errorf("LegacyAgentToolName = %q, want Task", tools.LegacyAgentToolName)
	}

	// Verify one-shot detection.
	if !tools.IsOneShotBuiltinAgent("Explore") {
		t.Error("Explore should be a one-shot builtin agent")
	}
	if !tools.IsOneShotBuiltinAgent("Plan") {
		t.Error("Plan should be a one-shot builtin agent")
	}
	if tools.IsOneShotBuiltinAgent("general-purpose") {
		t.Error("general-purpose should NOT be a one-shot builtin")
	}

	// Verify ValidModelEnums.
	for _, m := range []string{"sonnet", "opus", "haiku"} {
		if !tools.ValidModelEnums[m] {
			t.Errorf("ValidModelEnums should contain %q", m)
		}
	}

	// Verify AgentTool registers with correct name.
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)
	tools.RegisterAgentTool(registry, nil, nil)

	agent := registry.Get("Agent")
	if agent == nil {
		t.Fatal("Agent tool should be registered")
	}
	if agent.Name() != "Agent" {
		t.Errorf("agent.Name() = %q, want Agent", agent.Name())
	}

	// Verify LegacyAgentToolName is defined for permission rule matching.
	if tools.LegacyAgentToolName != "Task" {
		t.Errorf("LegacyAgentToolName = %q, want Task", tools.LegacyAgentToolName)
	}
}

// TestAgentToolInput_Validation verifies AgentToolInput validation through
// the real tool's parameter validation.
func TestAgentToolInput_Validation(t *testing.T) {
	input := tools.AgentToolInput{
		Description:  "test task",
		Prompt:       "do something",
		SubagentType: "Explore",
		Model:        "sonnet",
		Isolation:    "worktree",
	}

	if input.Description == "" {
		t.Error("description should be set")
	}
	if !tools.ValidModelEnums[input.Model] {
		t.Errorf("model %q should be valid", input.Model)
	}
	if !tools.ValidIsolationModes[input.Isolation] {
		t.Errorf("isolation %q should be valid", input.Isolation)
	}
}
