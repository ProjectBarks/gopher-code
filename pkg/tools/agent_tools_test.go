package tools

import (
	"context"
	"encoding/json"
	"strings"
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

// --- T501: AgentTool constants, schema, prompt, and validation tests ---

func TestAgentToolConstants(t *testing.T) {
	// Source: AgentTool/constants.ts:1-12
	t.Run("AGENT_TOOL_NAME", func(t *testing.T) {
		if AgentToolName != "Agent" {
			t.Errorf("AgentToolName = %q, want %q", AgentToolName, "Agent")
		}
	})
	t.Run("LEGACY_AGENT_TOOL_NAME", func(t *testing.T) {
		if LegacyAgentToolName != "Task" {
			t.Errorf("LegacyAgentToolName = %q, want %q", LegacyAgentToolName, "Task")
		}
	})
	t.Run("VERIFICATION_AGENT_TYPE", func(t *testing.T) {
		if VerificationAgentType != "verification" {
			t.Errorf("VerificationAgentType = %q, want %q", VerificationAgentType, "verification")
		}
	})
	t.Run("ONE_SHOT_BUILTIN_AGENT_TYPES", func(t *testing.T) {
		if !OneShotBuiltinAgentTypes["Explore"] {
			t.Error("Explore should be in OneShotBuiltinAgentTypes")
		}
		if !OneShotBuiltinAgentTypes["Plan"] {
			t.Error("Plan should be in OneShotBuiltinAgentTypes")
		}
		if OneShotBuiltinAgentTypes["general-purpose"] {
			t.Error("general-purpose should NOT be in OneShotBuiltinAgentTypes")
		}
	})
	t.Run("IsOneShotBuiltinAgent", func(t *testing.T) {
		if !IsOneShotBuiltinAgent("Explore") {
			t.Error("Explore should be one-shot")
		}
		if IsOneShotBuiltinAgent("custom-agent") {
			t.Error("custom-agent should NOT be one-shot")
		}
	})
}

func TestAgentToolInputSchemaFields(t *testing.T) {
	// Source: AgentTool/AgentTool.tsx:82-102 — verify all input schema fields
	tool := &mockTool{name: "Agent"}
	_ = tool // schema is on AgentTool, test via AgentToolInput struct

	// Verify the struct has all expected fields by constructing one
	input := AgentToolInput{
		Description:  "test desc",
		Prompt:       "test prompt",
		SubagentType: "Explore",
		Model:        "sonnet",
		RunInBG:      true,
		Name:         "my-agent",
		TeamName:     "my-team",
		Mode:         "plan",
		Isolation:    "worktree",
		CWD:          "/tmp/test",
	}

	// Verify JSON round-trip preserves all fields
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded AgentToolInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Description != "test desc" {
		t.Errorf("Description = %q", decoded.Description)
	}
	if decoded.Prompt != "test prompt" {
		t.Errorf("Prompt = %q", decoded.Prompt)
	}
	if decoded.SubagentType != "Explore" {
		t.Errorf("SubagentType = %q", decoded.SubagentType)
	}
	if decoded.Model != "sonnet" {
		t.Errorf("Model = %q", decoded.Model)
	}
	if !decoded.RunInBG {
		t.Error("RunInBG should be true")
	}
	if decoded.Name != "my-agent" {
		t.Errorf("Name = %q", decoded.Name)
	}
	if decoded.TeamName != "my-team" {
		t.Errorf("TeamName = %q", decoded.TeamName)
	}
	if decoded.Mode != "plan" {
		t.Errorf("Mode = %q", decoded.Mode)
	}
	if decoded.Isolation != "worktree" {
		t.Errorf("Isolation = %q", decoded.Isolation)
	}
	if decoded.CWD != "/tmp/test" {
		t.Errorf("CWD = %q", decoded.CWD)
	}
}

func TestAgentToolInputSchemaJSON(t *testing.T) {
	// Verify the JSON schema returned by AgentTool has all expected properties.
	// Source: AgentTool/AgentTool.tsx:82-102
	tool := &AgentTool_schemaChecker{}
	schema := tool.InputSchema()

	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema missing properties")
	}

	expectedFields := []string{
		"description", "prompt", "subagent_type", "model",
		"run_in_background", "name", "team_name", "mode",
		"isolation", "cwd",
	}
	for _, field := range expectedFields {
		if _, ok := props[field]; !ok {
			t.Errorf("schema missing field %q", field)
		}
	}

	// Check model enum values
	modelProp := props["model"].(map[string]interface{})
	modelEnum := modelProp["enum"].([]interface{})
	modelSet := make(map[string]bool)
	for _, v := range modelEnum {
		modelSet[v.(string)] = true
	}
	for _, expected := range []string{"sonnet", "opus", "haiku"} {
		if !modelSet[expected] {
			t.Errorf("model enum missing %q", expected)
		}
	}

	// Check isolation enum
	isoProp := props["isolation"].(map[string]interface{})
	isoEnum := isoProp["enum"].([]interface{})
	if len(isoEnum) != 1 || isoEnum[0] != "worktree" {
		t.Errorf("isolation enum = %v, want [worktree]", isoEnum)
	}
}

// AgentTool_schemaChecker is a minimal type to test the schema JSON.
type AgentTool_schemaChecker struct{}

func (a *AgentTool_schemaChecker) InputSchema() json.RawMessage {
	// Delegate to the real AgentTool
	tool := &AgentTool{}
	return tool.InputSchema()
}

func TestAgentToolPromptContainsKeyPhrases(t *testing.T) {
	// Source: AgentTool/prompt.ts — verify key phrases in the prompt
	prompt := AgentToolPrompt("test agent list section")

	keyPhrases := []string{
		"Launch a new agent to handle complex, multi-step tasks autonomously",
		"The Agent tool launches specialized agents",
		"When NOT to use the Agent tool",
		"Always include a short description",
		"Writing the prompt",
		"Brief the agent like a smart colleague",
		"Never delegate understanding",
		"run_in_background",
		"Foreground vs background",
		"isolation: \"worktree\"",
		"SendMessage",
		"subagent_type",
	}
	for _, phrase := range keyPhrases {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("prompt missing key phrase: %q", phrase)
		}
	}
}

func TestAgentToolPromptCore(t *testing.T) {
	// Source: AgentTool/prompt.ts:202-217
	core := AgentToolPromptCore("my agents here")
	if !strings.Contains(core, "my agents here") {
		t.Error("core prompt should include agent list section")
	}
	if !strings.Contains(core, "specify a subagent_type") {
		t.Error("core prompt should mention subagent_type")
	}
}

func TestValidateAgentName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"", false},                  // optional, empty is fine
		{"my-agent", false},          // valid
		{"agent123", false},          // valid
		{"a", false},                 // single char
		{"ship-audit", false},        // from TS example
		{"migration-review", false},  // from TS example
		{"-leading", true},           // starts with hyphen
		{"trailing-", true},          // ends with hyphen
		{"My-Agent", true},           // uppercase
		{"agent name", true},         // space
		{"agent_name", true},         // underscore
		{"agent.name", true},         // dot
		{strings.Repeat("a", 65), true}, // too long
		{strings.Repeat("a", 64), false}, // exactly at limit
	}
	for _, tc := range tests {
		err := ValidateAgentName(tc.name)
		if tc.wantErr && err == nil {
			t.Errorf("ValidateAgentName(%q) = nil, want error", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ValidateAgentName(%q) = %v, want nil", tc.name, err)
		}
	}
}

func TestValidateAgentToolInput(t *testing.T) {
	t.Run("valid_minimal", func(t *testing.T) {
		input := &AgentToolInput{Prompt: "do something", Description: "test"}
		if err := ValidateAgentToolInput(input); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("empty_prompt", func(t *testing.T) {
		input := &AgentToolInput{Description: "test"}
		err := ValidateAgentToolInput(input)
		if err == nil || !strings.Contains(err.Error(), "prompt is required") {
			t.Errorf("expected prompt required error, got %v", err)
		}
	})
	t.Run("invalid_model", func(t *testing.T) {
		input := &AgentToolInput{Prompt: "test", Model: "gpt-4"}
		err := ValidateAgentToolInput(input)
		if err == nil || !strings.Contains(err.Error(), "invalid model") {
			t.Errorf("expected invalid model error, got %v", err)
		}
	})
	t.Run("valid_model_enum", func(t *testing.T) {
		for _, m := range []string{"sonnet", "opus", "haiku"} {
			input := &AgentToolInput{Prompt: "test", Model: m}
			if err := ValidateAgentToolInput(input); err != nil {
				t.Errorf("model %q should be valid, got %v", m, err)
			}
		}
	})
	t.Run("invalid_isolation", func(t *testing.T) {
		input := &AgentToolInput{Prompt: "test", Isolation: "docker"}
		err := ValidateAgentToolInput(input)
		if err == nil || !strings.Contains(err.Error(), "invalid isolation") {
			t.Errorf("expected invalid isolation error, got %v", err)
		}
	})
	t.Run("cwd_with_worktree_mutually_exclusive", func(t *testing.T) {
		input := &AgentToolInput{Prompt: "test", Isolation: "worktree", CWD: "/tmp"}
		err := ValidateAgentToolInput(input)
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("expected mutually exclusive error, got %v", err)
		}
	})
	t.Run("invalid_permission_mode", func(t *testing.T) {
		input := &AgentToolInput{Prompt: "test", Mode: "yolo"}
		err := ValidateAgentToolInput(input)
		if err == nil || !strings.Contains(err.Error(), "invalid permission mode") {
			t.Errorf("expected invalid permission mode error, got %v", err)
		}
	})
	t.Run("valid_permission_modes", func(t *testing.T) {
		for _, m := range []string{"acceptEdits", "auto", "bypassPermissions", "default", "dontAsk", "plan"} {
			input := &AgentToolInput{Prompt: "test", Mode: m}
			if err := ValidateAgentToolInput(input); err != nil {
				t.Errorf("mode %q should be valid, got %v", m, err)
			}
		}
	})
	t.Run("invalid_agent_name", func(t *testing.T) {
		input := &AgentToolInput{Prompt: "test", Name: "Bad Name"}
		err := ValidateAgentToolInput(input)
		if err == nil {
			t.Error("expected error for invalid name")
		}
	})
	t.Run("full_valid_input", func(t *testing.T) {
		input := &AgentToolInput{
			Prompt:       "do the thing",
			Description:  "thing-doer",
			SubagentType: "Explore",
			Model:        "haiku",
			RunInBG:      true,
			Name:         "my-agent",
			TeamName:     "team-a",
			Mode:         "plan",
			Isolation:    "worktree",
		}
		// CWD is not set so no conflict with worktree
		if err := ValidateAgentToolInput(input); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestAgentToolAliasesInterface(t *testing.T) {
	// Source: AgentTool/AgentTool.tsx:228 — aliases: [LEGACY_AGENT_TOOL_NAME]
	tool := NewAgentTool(nil, nil, nil)
	aliases := tool.Aliases()
	if len(aliases) != 1 || aliases[0] != "Task" {
		t.Errorf("aliases = %v, want [Task]", aliases)
	}
	// Also verify via the Aliaser interface
	var itool Tool = tool
	if got := GetAliases(itool); len(got) != 1 || got[0] != "Task" {
		t.Errorf("GetAliases via interface = %v, want [Task]", got)
	}
}

func TestAgentToolSearchHint(t *testing.T) {
	// Source: AgentTool/AgentTool.tsx:227
	tool := NewAgentTool(nil, nil, nil)
	if tool.SearchHint() != "delegate work to a subagent" {
		t.Errorf("SearchHint() = %q", tool.SearchHint())
	}
	// Also via interface
	var itool Tool = tool
	if GetSearchHint(itool) != "delegate work to a subagent" {
		t.Errorf("GetSearchHint via interface = %q", GetSearchHint(itool))
	}
}

func TestAgentToolDescription(t *testing.T) {
	// Source: AgentTool/AgentTool.tsx:232
	tool := NewAgentTool(nil, nil, nil)
	if tool.Description() != "Launch a new agent" {
		t.Errorf("Description() = %q, want %q", tool.Description(), "Launch a new agent")
	}
}

func TestAgentToolPromptMethod(t *testing.T) {
	// Verify the Prompt() method returns non-empty and includes key content
	tool := NewAgentTool(nil, nil, nil)
	p := tool.Prompt()
	if p == "" {
		t.Fatal("Prompt() should not be empty")
	}
	if !strings.Contains(p, "Launch a new agent") {
		t.Error("Prompt() should contain 'Launch a new agent'")
	}
	// Also verify via interface
	var itool Tool = tool
	if GetToolPrompt(itool) == "" {
		t.Error("GetToolPrompt via interface should not be empty")
	}
}

func TestGetAliases(t *testing.T) {
	// Test the GetAliases helper function
	tool := NewAgentTool(nil, nil, nil)
	aliases := GetAliases(tool)
	if len(aliases) != 1 || aliases[0] != "Task" {
		t.Errorf("GetAliases = %v, want [Task]", aliases)
	}

	// Non-aliased tool returns nil
	mock := &mockTool{name: "Read"}
	if got := GetAliases(mock); got != nil {
		t.Errorf("GetAliases(mockTool) = %v, want nil", got)
	}
}

func TestValidModelEnums(t *testing.T) {
	// Source: AgentTool/AgentTool.tsx:86
	for _, m := range []string{"sonnet", "opus", "haiku"} {
		if !ValidModelEnums[m] {
			t.Errorf("ValidModelEnums missing %q", m)
		}
	}
	if ValidModelEnums["gpt-4"] {
		t.Error("gpt-4 should not be in ValidModelEnums")
	}
}

func TestValidIsolationModes(t *testing.T) {
	// Source: AgentTool/AgentTool.tsx:99
	if !ValidIsolationModes["worktree"] {
		t.Error("worktree should be valid")
	}
	if ValidIsolationModes["docker"] {
		t.Error("docker should not be valid")
	}
}

func TestValidPermissionModes(t *testing.T) {
	// Source: permissionModeSchema
	expected := []string{"acceptEdits", "auto", "bypassPermissions", "default", "dontAsk", "plan"}
	for _, m := range expected {
		if !ValidPermissionModes[m] {
			t.Errorf("ValidPermissionModes missing %q", m)
		}
	}
}
