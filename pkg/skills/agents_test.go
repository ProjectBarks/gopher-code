package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// Source: tools/AgentTool/loadAgentsDir.ts, tools/AgentTool/builtInAgents.ts

func TestGetBuiltInAgents(t *testing.T) {
	// Source: builtInAgents.ts:45-71
	agents := GetBuiltInAgents()

	if len(agents) < 5 {
		t.Errorf("expected at least 5 built-in agents, got %d", len(agents))
	}

	// All must have source=built-in
	for _, a := range agents {
		if a.Source != AgentSourceBuiltIn {
			t.Errorf("agent %q source = %q, want built-in", a.AgentType, a.Source)
		}
		if a.BaseDir != "built-in" {
			t.Errorf("agent %q baseDir = %q, want built-in", a.AgentType, a.BaseDir)
		}
	}
}

func TestBuiltInAgentTypes(t *testing.T) {
	// Source: builtInAgents.ts — verify all expected types exist
	agents := GetBuiltInAgents()
	expected := map[string]bool{
		"general-purpose":  false,
		"statusline-setup": false,
		"Explore":          false,
		"Plan":             false,
		"claude-code-guide": false,
	}

	for _, a := range agents {
		if _, ok := expected[a.AgentType]; ok {
			expected[a.AgentType] = true
		}
	}

	for typ, found := range expected {
		if !found {
			t.Errorf("missing built-in agent %q", typ)
		}
	}
}

func TestGeneralPurposeAgent(t *testing.T) {
	// Source: built-in/generalPurposeAgent.ts
	agents := GetBuiltInAgents()
	gp := FindAgent(agents, AgentTypeGeneralPurpose)
	if gp == nil {
		t.Fatal("general-purpose agent not found")
	}
	if gp.Tools != nil {
		t.Error("general-purpose should have nil tools (all tools available)")
	}
	if gp.WhenToUse == "" {
		t.Error("should have a description")
	}
}

func TestExploreAgent(t *testing.T) {
	// Source: built-in/exploreAgent.ts
	agents := GetBuiltInAgents()
	explore := FindAgent(agents, AgentTypeExplore)
	if explore == nil {
		t.Fatal("Explore agent not found")
	}
	if explore.Model != "haiku" {
		t.Errorf("model = %q, want haiku", explore.Model)
	}
	if !explore.OmitClaudeMd {
		t.Error("Explore should have omitClaudeMd=true")
	}
	if len(explore.DisallowedTools) == 0 {
		t.Error("Explore should have disallowed tools")
	}
}

func TestPlanAgent(t *testing.T) {
	// Source: built-in/planAgent.ts
	agents := GetBuiltInAgents()
	plan := FindAgent(agents, AgentTypePlan)
	if plan == nil {
		t.Fatal("Plan agent not found")
	}
	if plan.Model != "inherit" {
		t.Errorf("model = %q, want inherit", plan.Model)
	}
	if !plan.OmitClaudeMd {
		t.Error("Plan should have omitClaudeMd=true")
	}
}

func TestClaudeCodeGuideAgent(t *testing.T) {
	// Source: built-in/claudeCodeGuideAgent.ts
	agents := GetBuiltInAgents()
	guide := FindAgent(agents, AgentTypeClaudeCodeGuide)
	if guide == nil {
		t.Fatal("claude-code-guide agent not found")
	}
	if guide.Model != "haiku" {
		t.Errorf("model = %q, want haiku", guide.Model)
	}
	if len(guide.Tools) == 0 {
		t.Error("guide should have explicit tools list")
	}
}

func TestStatuslineSetupAgent(t *testing.T) {
	// Source: built-in/statuslineSetup.ts
	agents := GetBuiltInAgents()
	sl := FindAgent(agents, AgentTypeStatuslineSetup)
	if sl == nil {
		t.Fatal("statusline-setup agent not found")
	}
	if sl.Model != "sonnet" {
		t.Errorf("model = %q, want sonnet", sl.Model)
	}
	if sl.Color != "orange" {
		t.Errorf("color = %q, want orange", sl.Color)
	}
	if len(sl.Tools) != 2 {
		t.Errorf("tools count = %d, want 2 (Read, Edit)", len(sl.Tools))
	}
}

func TestAgentTypeConstants(t *testing.T) {
	// Source: built-in/*.ts — exact string values
	if AgentTypeGeneralPurpose != "general-purpose" {
		t.Error("wrong")
	}
	if AgentTypeExplore != "Explore" {
		t.Error("wrong")
	}
	if AgentTypePlan != "Plan" {
		t.Error("wrong")
	}
	if AgentTypeClaudeCodeGuide != "claude-code-guide" {
		t.Error("wrong")
	}
	if AgentTypeStatuslineSetup != "statusline-setup" {
		t.Error("wrong")
	}
	if AgentTypeVerification != "verification" {
		t.Error("wrong")
	}
}

func TestAgentColors(t *testing.T) {
	// Source: agentColorManager.ts
	if len(AgentColors) != 8 {
		t.Errorf("expected 8 agent colors, got %d", len(AgentColors))
	}
	for _, c := range []string{"blue", "green", "purple", "orange", "red", "cyan", "magenta", "yellow"} {
		if !isValidAgentColor(c) {
			t.Errorf("%q should be a valid color", c)
		}
	}
	if isValidAgentColor("pink") {
		t.Error("pink should not be valid")
	}
}

func TestParseAgentFromMarkdown(t *testing.T) {
	// Source: loadAgentsDir.ts:541-755
	t.Run("basic_agent", func(t *testing.T) {
		content := `---
name: my-agent
description: A custom agent for testing
model: sonnet
---
You are a test agent.`

		agent := ParseAgentFromMarkdown("/tmp/my-agent.md", "/tmp", content, AgentSourceProject)
		if agent == nil {
			t.Fatal("expected non-nil agent")
		}
		if agent.AgentType != "my-agent" {
			t.Errorf("agentType = %q", agent.AgentType)
		}
		if agent.WhenToUse != "A custom agent for testing" {
			t.Errorf("whenToUse = %q", agent.WhenToUse)
		}
		if agent.Model != "sonnet" {
			t.Errorf("model = %q", agent.Model)
		}
		if agent.SystemPrompt != "You are a test agent." {
			t.Errorf("systemPrompt = %q", agent.SystemPrompt)
		}
		if agent.Source != AgentSourceProject {
			t.Errorf("source = %q", agent.Source)
		}
		if agent.Filename != "my-agent" {
			t.Errorf("filename = %q", agent.Filename)
		}
	})

	t.Run("missing_name_returns_nil", func(t *testing.T) {
		// Source: loadAgentsDir.ts:554-556
		content := `---
description: No name field
---
Some content`

		agent := ParseAgentFromMarkdown("/tmp/test.md", "/tmp", content, AgentSourceUser)
		if agent != nil {
			t.Error("expected nil for missing name")
		}
	})

	t.Run("missing_description_returns_nil", func(t *testing.T) {
		// Source: loadAgentsDir.ts:557-562
		content := `---
name: test-agent
---
Some content`

		agent := ParseAgentFromMarkdown("/tmp/test.md", "/tmp", content, AgentSourceUser)
		if agent != nil {
			t.Error("expected nil for missing description")
		}
	})

	t.Run("all_optional_fields", func(t *testing.T) {
		content := `---
name: full-agent
description: Full featured agent
model: haiku
color: blue
background: true
memory: project
isolation: worktree
effort: medium
permissionMode: auto
maxTurns: 10
tools: [Read, Bash, Glob]
disallowedTools: [Write]
initialPrompt: Start here
---
Full system prompt.`

		agent := ParseAgentFromMarkdown("/tmp/full.md", "/tmp", content, AgentSourceProject)
		if agent == nil {
			t.Fatal("expected non-nil agent")
		}
		if agent.Model != "haiku" {
			t.Errorf("model = %q", agent.Model)
		}
		if agent.Color != "blue" {
			t.Errorf("color = %q", agent.Color)
		}
		if !agent.Background {
			t.Error("background should be true")
		}
		if agent.Memory != AgentMemoryProject {
			t.Errorf("memory = %q", agent.Memory)
		}
		if agent.Isolation != "worktree" {
			t.Errorf("isolation = %q", agent.Isolation)
		}
		if agent.Effort != "medium" {
			t.Errorf("effort = %q", agent.Effort)
		}
		if agent.PermissionMode != "auto" {
			t.Errorf("permissionMode = %q", agent.PermissionMode)
		}
		if agent.MaxTurns != 10 {
			t.Errorf("maxTurns = %d", agent.MaxTurns)
		}
		if len(agent.Tools) != 3 {
			t.Errorf("tools count = %d", len(agent.Tools))
		}
		if len(agent.DisallowedTools) != 1 {
			t.Errorf("disallowedTools count = %d", len(agent.DisallowedTools))
		}
		if agent.InitialPrompt != "Start here" {
			t.Errorf("initialPrompt = %q", agent.InitialPrompt)
		}
	})

	t.Run("inherit_model", func(t *testing.T) {
		// Source: loadAgentsDir.ts:572
		content := `---
name: test
description: test
model: INHERIT
---
prompt`

		agent := ParseAgentFromMarkdown("/tmp/test.md", "/tmp", content, AgentSourceUser)
		if agent == nil {
			t.Fatal("expected non-nil agent")
		}
		if agent.Model != "inherit" {
			t.Errorf("model = %q, want 'inherit' (case-insensitive)", agent.Model)
		}
	})

	t.Run("no_frontmatter_returns_nil", func(t *testing.T) {
		agent := ParseAgentFromMarkdown("/tmp/test.md", "/tmp", "Just plain text", AgentSourceUser)
		if agent != nil {
			t.Error("expected nil for no frontmatter")
		}
	})
}

func TestLoadAgentsFromDir(t *testing.T) {
	dir := t.TempDir()

	// Create a valid agent
	os.WriteFile(filepath.Join(dir, "test-agent.md"), []byte(`---
name: test-agent
description: A test agent
model: sonnet
---
You are a test agent.`), 0644)

	// Create an invalid file (no frontmatter)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("Just a readme"), 0644)

	// Create a non-md file (should be ignored)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("notes"), 0644)

	agents := loadAgentsFromDir(dir, AgentSourceProject)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].AgentType != "test-agent" {
		t.Errorf("agentType = %q", agents[0].AgentType)
	}
}

func TestLoadAgentsFromNonexistentDir(t *testing.T) {
	agents := loadAgentsFromDir("/nonexistent/path", AgentSourceUser)
	if agents != nil {
		t.Error("expected nil for nonexistent dir")
	}
}

func TestGetActiveAgents(t *testing.T) {
	// Source: loadAgentsDir.ts:193-221 — later sources override earlier
	all := []AgentDefinition{
		{AgentType: "agent-a", Source: AgentSourceBuiltIn, WhenToUse: "built-in version"},
		{AgentType: "agent-b", Source: AgentSourceBuiltIn, WhenToUse: "only built-in"},
		{AgentType: "agent-a", Source: AgentSourceProject, WhenToUse: "project override"},
	}

	active := GetActiveAgents(all)
	// agent-a should be overridden by project version
	agentA := FindAgent(active, "agent-a")
	if agentA == nil {
		t.Fatal("agent-a not found")
	}
	if agentA.Source != AgentSourceProject {
		t.Errorf("agent-a should be project version, got source=%q", agentA.Source)
	}
	if agentA.WhenToUse != "project override" {
		t.Errorf("whenToUse = %q", agentA.WhenToUse)
	}
}

func TestHasRequiredMcpServers(t *testing.T) {
	// Source: loadAgentsDir.ts:229-239
	t.Run("no_requirements", func(t *testing.T) {
		agent := AgentDefinition{AgentType: "test"}
		if !HasRequiredMcpServers(agent, nil) {
			t.Error("no requirements should always pass")
		}
	})

	t.Run("requirements_met", func(t *testing.T) {
		agent := AgentDefinition{
			AgentType:          "test",
			RequiredMcpServers: []string{"slack"},
		}
		if !HasRequiredMcpServers(agent, []string{"slack-server", "github"}) {
			t.Error("should match 'slack' in 'slack-server'")
		}
	})

	t.Run("requirements_not_met", func(t *testing.T) {
		agent := AgentDefinition{
			AgentType:          "test",
			RequiredMcpServers: []string{"jira"},
		}
		if HasRequiredMcpServers(agent, []string{"slack", "github"}) {
			t.Error("jira requirement should not be met")
		}
	})

	t.Run("case_insensitive", func(t *testing.T) {
		// Source: loadAgentsDir.ts:238 — case-insensitive matching
		agent := AgentDefinition{
			AgentType:          "test",
			RequiredMcpServers: []string{"SLACK"},
		}
		if !HasRequiredMcpServers(agent, []string{"slack-bot"}) {
			t.Error("matching should be case-insensitive")
		}
	})
}

func TestIsBuiltInAgent(t *testing.T) {
	// Source: loadAgentsDir.ts:168-172
	if !IsBuiltInAgent(AgentDefinition{Source: AgentSourceBuiltIn}) {
		t.Error("should be built-in")
	}
	if IsBuiltInAgent(AgentDefinition{Source: AgentSourceProject}) {
		t.Error("should not be built-in")
	}
}

func TestIsCustomAgent(t *testing.T) {
	// Source: loadAgentsDir.ts:174-178
	if !IsCustomAgent(AgentDefinition{Source: AgentSourceProject}) {
		t.Error("project agent should be custom")
	}
	if !IsCustomAgent(AgentDefinition{Source: AgentSourceUser}) {
		t.Error("user agent should be custom")
	}
	if IsCustomAgent(AgentDefinition{Source: AgentSourceBuiltIn}) {
		t.Error("built-in should not be custom")
	}
	if IsCustomAgent(AgentDefinition{Source: AgentSourcePlugin}) {
		t.Error("plugin should not be custom")
	}
}

func TestBuiltInSkillNames(t *testing.T) {
	// Source: skills/bundled/index.ts
	if len(AllBuiltInSkillNames) < 8 {
		t.Errorf("expected at least 8 built-in skills, got %d", len(AllBuiltInSkillNames))
	}
	expected := []string{"update-config", "keybindings-help", "verify", "debug", "remember", "simplify", "batch", "stuck"}
	for _, e := range expected {
		found := false
		for _, s := range AllBuiltInSkillNames {
			if s == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing built-in skill %q", e)
		}
	}
}

func TestFindAgent(t *testing.T) {
	agents := []AgentDefinition{
		{AgentType: "a", WhenToUse: "desc-a"},
		{AgentType: "b", WhenToUse: "desc-b"},
	}
	if a := FindAgent(agents, "a"); a == nil || a.WhenToUse != "desc-a" {
		t.Error("should find agent 'a'")
	}
	if FindAgent(agents, "c") != nil {
		t.Error("should return nil for missing agent")
	}
}

func TestParseFrontmatterMap(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		fm, body := parseFrontmatterMap("---\nname: test\ndescription: desc\n---\nbody content")
		if fm == nil {
			t.Fatal("expected frontmatter")
		}
		if fm["name"] != "test" {
			t.Errorf("name = %v", fm["name"])
		}
		if fm["description"] != "desc" {
			t.Errorf("description = %v", fm["description"])
		}
		if body != "body content" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("array_syntax", func(t *testing.T) {
		fm, _ := parseFrontmatterMap("---\ntools: [Read, Write, Bash]\n---\n")
		tools, ok := fm["tools"].([]interface{})
		if !ok {
			t.Fatalf("tools should be array, got %T", fm["tools"])
		}
		if len(tools) != 3 {
			t.Errorf("expected 3 tools, got %d", len(tools))
		}
	})

	t.Run("boolean_values", func(t *testing.T) {
		fm, _ := parseFrontmatterMap("---\nbackground: true\nomitClaudeMd: false\n---\n")
		if fm["background"] != true {
			t.Errorf("background = %v", fm["background"])
		}
		if fm["omitClaudeMd"] != false {
			t.Errorf("omitClaudeMd = %v", fm["omitClaudeMd"])
		}
	})

	t.Run("integer_values", func(t *testing.T) {
		fm, _ := parseFrontmatterMap("---\nmaxTurns: 10\n---\n")
		if fm["maxTurns"] != 10 {
			t.Errorf("maxTurns = %v (%T)", fm["maxTurns"], fm["maxTurns"])
		}
	})

	t.Run("no_frontmatter", func(t *testing.T) {
		fm, body := parseFrontmatterMap("just text")
		if fm != nil {
			t.Error("expected nil frontmatter")
		}
		if body != "just text" {
			t.Errorf("body = %q", body)
		}
	})
}

func TestParseToolsList(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if parseToolsList(nil) != nil {
			t.Error("nil should return nil")
		}
	})

	t.Run("comma_separated", func(t *testing.T) {
		result := parseToolsList("Read, Write, Bash")
		if len(result) != 3 {
			t.Fatalf("expected 3, got %d", len(result))
		}
		if result[0] != "Read" || result[1] != "Write" || result[2] != "Bash" {
			t.Errorf("got %v", result)
		}
	})

	t.Run("array_interface", func(t *testing.T) {
		result := parseToolsList([]interface{}{"Read", "Write"})
		if len(result) != 2 {
			t.Errorf("expected 2, got %d", len(result))
		}
	})

	t.Run("string_array", func(t *testing.T) {
		result := parseToolsList([]string{"Glob", "Grep"})
		if len(result) != 2 {
			t.Errorf("expected 2, got %d", len(result))
		}
	})

	t.Run("empty_string", func(t *testing.T) {
		if parseToolsList("") != nil {
			t.Error("empty string should return nil")
		}
	})
}

func TestLoadAllAgents(t *testing.T) {
	cwd := t.TempDir()
	agentsDir := filepath.Join(cwd, ".claude", "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "custom.md"), []byte(`---
name: custom-agent
description: A custom project agent
---
Custom system prompt.`), 0644)

	result := LoadAllAgents(cwd)
	if len(result.AllAgents) < 6 { // 5 built-in + 1 custom
		t.Errorf("expected at least 6 agents, got %d", len(result.AllAgents))
	}

	// Find custom agent
	custom := FindAgent(result.AllAgents, "custom-agent")
	if custom == nil {
		t.Fatal("custom agent not found")
	}
	if custom.Source != AgentSourceProject {
		t.Errorf("source = %q, want projectSettings", custom.Source)
	}
}

func TestAgentSourceConstants(t *testing.T) {
	// Source: loadAgentsDir.ts
	if AgentSourceBuiltIn != "built-in" {
		t.Error("wrong")
	}
	if AgentSourceUser != "userSettings" {
		t.Error("wrong")
	}
	if AgentSourceProject != "projectSettings" {
		t.Error("wrong")
	}
	if AgentSourcePolicy != "policySettings" {
		t.Error("wrong")
	}
	if AgentSourcePlugin != "plugin" {
		t.Error("wrong")
	}
}
