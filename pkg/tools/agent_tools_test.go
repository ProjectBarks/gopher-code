package tools

import (
	"context"
	"encoding/json"
	"testing"
)

// Source: tools/AgentTool/agentToolUtils.ts, constants/tools.ts

// mockTool is a simple tool for testing.
type mockTool struct {
	name     string
	readOnly bool
}

func (m *mockTool) Name() string                  { return m.name }
func (m *mockTool) Description() string            { return "mock tool " + m.name }
func (m *mockTool) InputSchema() json.RawMessage   { return json.RawMessage(`{"type":"object"}`) }
func (m *mockTool) IsReadOnly() bool               { return m.readOnly }
func (m *mockTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	return SuccessOutput("ok"), nil
}

func makeMockTools(names ...string) []Tool {
	tools := make([]Tool, len(names))
	for i, n := range names {
		tools[i] = &mockTool{name: n}
	}
	return tools
}

func toolNames(tools []Tool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name()
	}
	return names
}

func containsTool(tools []Tool, name string) bool {
	for _, t := range tools {
		if t.Name() == name {
			return true
		}
	}
	return false
}

func TestAllAgentDisallowedTools(t *testing.T) {
	// Source: constants/tools.ts:36-46
	expected := []string{"TaskOutput", "ExitPlanMode", "EnterPlanMode", "Agent", "AskUserQuestion", "TaskStop"}
	for _, name := range expected {
		if !AllAgentDisallowedTools[name] {
			t.Errorf("expected %q in AllAgentDisallowedTools", name)
		}
	}
}

func TestAsyncAgentAllowedTools(t *testing.T) {
	// Source: constants/tools.ts:55-71
	expected := []string{"Read", "WebSearch", "Grep", "WebFetch", "Glob", "Bash", "Edit", "Write", "ToolSearch"}
	for _, name := range expected {
		if !AsyncAgentAllowedTools[name] {
			t.Errorf("expected %q in AsyncAgentAllowedTools", name)
		}
	}
	if AsyncAgentAllowedTools["Agent"] {
		t.Error("Agent should NOT be in AsyncAgentAllowedTools")
	}
}

func TestFilterToolsForAgent_BuiltIn_Sync(t *testing.T) {
	// Source: agentToolUtils.ts:70-116
	tools := makeMockTools("Read", "Bash", "Agent", "TaskOutput", "ExitPlanMode", "AskUserQuestion")

	filtered := FilterToolsForAgent(tools, true, false, "")

	// Agent should be blocked (in AllAgentDisallowedTools)
	if containsTool(filtered, "Agent") {
		t.Error("Agent should be blocked for sub-agents")
	}
	// TaskOutput should be blocked
	if containsTool(filtered, "TaskOutput") {
		t.Error("TaskOutput should be blocked")
	}
	// Read and Bash should pass
	if !containsTool(filtered, "Read") {
		t.Error("Read should be allowed")
	}
	if !containsTool(filtered, "Bash") {
		t.Error("Bash should be allowed")
	}
}

func TestFilterToolsForAgent_NonBuiltIn(t *testing.T) {
	// Source: agentToolUtils.ts:97-99 — custom agents use CUSTOM_AGENT_DISALLOWED_TOOLS
	tools := makeMockTools("Read", "Agent", "TaskOutput")

	filtered := FilterToolsForAgent(tools, false, false, "")

	if containsTool(filtered, "Agent") {
		t.Error("Agent should be blocked for custom agents")
	}
	if containsTool(filtered, "TaskOutput") {
		t.Error("TaskOutput should be blocked for custom agents")
	}
	if !containsTool(filtered, "Read") {
		t.Error("Read should be allowed")
	}
}

func TestFilterToolsForAgent_Async(t *testing.T) {
	// Source: agentToolUtils.ts:100 — async only allows ASYNC_AGENT_ALLOWED_TOOLS
	tools := makeMockTools("Read", "Bash", "Glob", "SendMessage", "TaskCreate")

	filtered := FilterToolsForAgent(tools, true, true, "")

	if !containsTool(filtered, "Read") {
		t.Error("Read should be in async allowed list")
	}
	if !containsTool(filtered, "Bash") {
		t.Error("Bash should be in async allowed list")
	}
	if !containsTool(filtered, "Glob") {
		t.Error("Glob should be in async allowed list")
	}
	// SendMessage and TaskCreate are in InProcessTeammateAllowedTools
	if !containsTool(filtered, "SendMessage") {
		t.Error("SendMessage should be allowed (teammate tool)")
	}
	if !containsTool(filtered, "TaskCreate") {
		t.Error("TaskCreate should be allowed (teammate tool)")
	}
}

func TestFilterToolsForAgent_MCPPassthrough(t *testing.T) {
	// Source: agentToolUtils.ts:83 — MCP tools always allowed
	tools := makeMockTools("mcp__slack__send", "mcp__github__pr", "Read")

	filtered := FilterToolsForAgent(tools, true, false, "")

	if !containsTool(filtered, "mcp__slack__send") {
		t.Error("MCP tools should always be allowed")
	}
	if !containsTool(filtered, "mcp__github__pr") {
		t.Error("MCP tools should always be allowed")
	}
}

func TestFilterToolsForAgent_PlanModeExitPlanMode(t *testing.T) {
	// Source: agentToolUtils.ts:88-93 — ExitPlanMode allowed in plan mode
	tools := makeMockTools("Read", "ExitPlanMode")

	// Plan mode: ExitPlanMode should pass
	filtered := FilterToolsForAgent(tools, true, false, "plan")
	if !containsTool(filtered, "ExitPlanMode") {
		t.Error("ExitPlanMode should be allowed in plan mode")
	}

	// Non-plan mode: ExitPlanMode should be blocked
	filtered = FilterToolsForAgent(tools, true, false, "")
	if containsTool(filtered, "ExitPlanMode") {
		t.Error("ExitPlanMode should be blocked outside plan mode")
	}
}

func TestResolveAgentTools_Wildcard(t *testing.T) {
	// Source: agentToolUtils.ts:163-173 — nil tools = wildcard
	tools := makeMockTools("Read", "Bash", "Glob")

	// nil tools = wildcard
	result := ResolveAgentTools(nil, nil, "built-in", "", tools, false, true)
	if !result.HasWildcard {
		t.Error("nil tools should be wildcard")
	}
	if len(result.ResolvedTools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(result.ResolvedTools))
	}

	// ["*"] = wildcard
	result = ResolveAgentTools([]string{"*"}, nil, "built-in", "", tools, false, true)
	if !result.HasWildcard {
		t.Error("[*] should be wildcard")
	}
}

func TestResolveAgentTools_SpecificTools(t *testing.T) {
	// Source: agentToolUtils.ts:186-215
	tools := makeMockTools("Read", "Bash", "Glob", "Write")

	result := ResolveAgentTools([]string{"Read", "Glob"}, nil, "built-in", "", tools, false, true)
	if result.HasWildcard {
		t.Error("should not be wildcard")
	}
	if len(result.ResolvedTools) != 2 {
		t.Errorf("expected 2 resolved tools, got %d", len(result.ResolvedTools))
	}
	if !containsTool(result.ResolvedTools, "Read") {
		t.Error("Read should be resolved")
	}
	if !containsTool(result.ResolvedTools, "Glob") {
		t.Error("Glob should be resolved")
	}
	if len(result.ValidTools) != 2 {
		t.Errorf("expected 2 valid, got %d", len(result.ValidTools))
	}
}

func TestResolveAgentTools_InvalidTools(t *testing.T) {
	// Source: agentToolUtils.ts:213 — tools not in available list are invalid
	tools := makeMockTools("Read", "Bash")

	result := ResolveAgentTools([]string{"Read", "Nonexistent"}, nil, "built-in", "", tools, false, true)
	if len(result.InvalidTools) != 1 || result.InvalidTools[0] != "Nonexistent" {
		t.Errorf("expected ['Nonexistent'] invalid, got %v", result.InvalidTools)
	}
}

func TestResolveAgentTools_DisallowedTools(t *testing.T) {
	// Source: agentToolUtils.ts:150-160
	tools := makeMockTools("Read", "Bash", "Write")

	result := ResolveAgentTools(nil, []string{"Bash"}, "built-in", "", tools, false, true)
	if containsTool(result.ResolvedTools, "Bash") {
		t.Error("Bash should be disallowed")
	}
	if !containsTool(result.ResolvedTools, "Read") {
		t.Error("Read should still be allowed")
	}
	if !containsTool(result.ResolvedTools, "Write") {
		t.Error("Write should still be allowed")
	}
}

func TestResolveAgentTools_AgentSpec(t *testing.T) {
	// Source: agentToolUtils.ts:191-204 — Agent(type1, type2)
	tools := makeMockTools("Read", "Agent")

	result := ResolveAgentTools(
		[]string{"Read", "Agent(Explore, Plan)"},
		nil, "built-in", "", tools, false, false,
	)

	if len(result.AllowedAgentTypes) != 2 {
		t.Fatalf("expected 2 allowed agent types, got %d", len(result.AllowedAgentTypes))
	}
	if result.AllowedAgentTypes[0] != "Explore" || result.AllowedAgentTypes[1] != "Plan" {
		t.Errorf("got %v", result.AllowedAgentTypes)
	}
}

func TestResolveAgentTools_FilteringApplied(t *testing.T) {
	// Source: agentToolUtils.ts:140-147 — isMainThread=false applies filtering
	tools := makeMockTools("Read", "Bash", "Agent", "TaskOutput")

	// isMainThread=false should filter disallowed tools
	result := ResolveAgentTools(nil, nil, "built-in", "", tools, false, false)
	if containsTool(result.ResolvedTools, "Agent") {
		t.Error("Agent should be filtered for non-main-thread")
	}
	if containsTool(result.ResolvedTools, "TaskOutput") {
		t.Error("TaskOutput should be filtered for non-main-thread")
	}
	if !containsTool(result.ResolvedTools, "Read") {
		t.Error("Read should pass through")
	}
}

func TestResolveAgentTools_MainThreadSkipsFilter(t *testing.T) {
	// Source: agentToolUtils.ts:140-141 — isMainThread=true skips filterToolsForAgent
	tools := makeMockTools("Read", "Agent", "TaskOutput")

	result := ResolveAgentTools(nil, nil, "built-in", "", tools, false, true)
	// Main thread should keep everything
	if !containsTool(result.ResolvedTools, "Agent") {
		t.Error("Agent should be kept on main thread")
	}
	if !containsTool(result.ResolvedTools, "TaskOutput") {
		t.Error("TaskOutput should be kept on main thread")
	}
}

func TestExtractToolName(t *testing.T) {
	// Source: utils/permissions/permissionRuleParser.ts
	tests := []struct {
		spec string
		want string
	}{
		{"Read", "Read"},
		{"Bash(git *)", "Bash"},
		{"Agent(Explore, Plan)", "Agent"},
		{"mcp__slack__send", "mcp__slack__send"},
	}
	for _, tc := range tests {
		if got := extractToolName(tc.spec); got != tc.want {
			t.Errorf("extractToolName(%q) = %q, want %q", tc.spec, got, tc.want)
		}
	}
}

func TestExtractRuleContent(t *testing.T) {
	tests := []struct {
		spec string
		want string
	}{
		{"Read", ""},
		{"Bash(git *)", "git *"},
		{"Agent(Explore, Plan)", "Explore, Plan"},
		{"Tool()", ""},
	}
	for _, tc := range tests {
		if got := extractRuleContent(tc.spec); got != tc.want {
			t.Errorf("extractRuleContent(%q) = %q, want %q", tc.spec, got, tc.want)
		}
	}
}

func TestGetAgentModel(t *testing.T) {
	// Source: utils/model/agent.ts:37-95

	t.Run("inherit", func(t *testing.T) {
		// Source: agent.ts:80-88
		model := GetAgentModel("inherit", "claude-opus-4-6", "")
		if model != "claude-opus-4-6" {
			t.Errorf("inherit should return parent model, got %q", model)
		}
	})

	t.Run("default_is_inherit", func(t *testing.T) {
		// Source: agent.ts:78 — empty agentModel defaults to inherit
		model := GetAgentModel("", "claude-opus-4-6", "")
		if model != "claude-opus-4-6" {
			t.Errorf("empty should inherit parent model, got %q", model)
		}
	})

	t.Run("explicit_model", func(t *testing.T) {
		model := GetAgentModel("haiku", "claude-opus-4-6", "")
		if model != "haiku" {
			t.Errorf("expected haiku, got %q", model)
		}
	})

	t.Run("tool_specified_overrides_definition", func(t *testing.T) {
		// Source: agent.ts:70-76
		model := GetAgentModel("sonnet", "claude-opus-4-6", "haiku")
		if model != "haiku" {
			t.Errorf("tool-specified should override definition, got %q", model)
		}
	})

	t.Run("env_override_highest_priority", func(t *testing.T) {
		// Source: agent.ts:43-45
		old := getEnvVar
		getEnvVar = func(key string) string {
			if key == "CLAUDE_CODE_SUBAGENT_MODEL" {
				return "custom-model"
			}
			return ""
		}
		defer func() { getEnvVar = old }()

		model := GetAgentModel("sonnet", "claude-opus-4-6", "haiku")
		if model != "custom-model" {
			t.Errorf("env should override all, got %q", model)
		}
	})
}

func TestInProcessTeammateAllowedTools(t *testing.T) {
	// Source: constants/tools.ts:77-85
	expected := []string{"TaskCreate", "TaskGet", "TaskList", "TaskUpdate", "SendMessage"}
	for _, name := range expected {
		if !InProcessTeammateAllowedTools[name] {
			t.Errorf("expected %q in InProcessTeammateAllowedTools", name)
		}
	}
}
