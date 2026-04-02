package tools_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func agentSystemPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "agent_system.json")
}

type AgentSystem struct {
	BuiltInAgents struct {
		AlwaysAvailable []struct {
			Type  string   `json:"type"`
			Tools []string `json:"tools"`
			Model string   `json:"model"`
		} `json:"always_available"`
		ConditionallyAvailable []struct {
			Type      string `json:"type"`
			Model     string `json:"model,omitempty"`
			Condition string `json:"condition"`
		} `json:"conditionally_available"`
		CanDisableAllVia string `json:"can_disable_all_via"`
	} `json:"built_in_agents"`
	AgentToolBehavior struct {
		DefaultType              string   `json:"default_type_when_omitted"`
		ForkOverridesDefault     bool     `json:"fork_subagent_overrides_default"`
		ExplicitTypeWins         bool     `json:"subagent_type_explicit_wins"`
		SpawnsOwnSession         bool     `json:"agent_spawns_with_own_session"`
		InheritsToolContext       bool     `json:"agent_inherits_tool_context"`
		IsolationModes           []string `json:"isolation_modes"`
		ModelOptions             []string `json:"model_options"`
	} `json:"agent_tool_behavior"`
	ExploreAgent struct {
		Type             string   `json:"type"`
		DisallowedTools  []string `json:"disallowed_tools"`
		ModelExternal    string   `json:"model_external"`
		ModelAnt         string   `json:"model_ant"`
	} `json:"explore_agent"`
	PlanAgent struct {
		Type                string   `json:"type"`
		DisallowedTools     []string `json:"disallowed_tools"`
		Model               string   `json:"model"`
		SharesToolsWithExplore bool  `json:"shares_tools_with_explore"`
	} `json:"plan_agent"`
	MCPNaming struct {
		ToolNameFormat string `json:"tool_name_format"`
		Separator      string `json:"separator"`
		Prefix         string `json:"prefix"`
		Detection      string `json:"detection"`
		ParseRule      string `json:"parse_rule"`
	} `json:"mcp_naming"`
	SessionPersistence struct {
		Format    string `json:"format"`
		Extension string `json:"file_extension"`
		Location  string `json:"location"`
		HistoryFile string `json:"history_file"`
	} `json:"session_persistence"`
}

func loadAgentSystem(t *testing.T) *AgentSystem {
	t.Helper()
	data, err := os.ReadFile(agentSystemPath())
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	var as AgentSystem
	if err := json.Unmarshal(data, &as); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return &as
}

// TestBuiltInAgents validates the agent type system.
// Source: builtInAgents.ts:22-75
func TestBuiltInAgents(t *testing.T) {
	as := loadAgentSystem(t)

	t.Run("always_available_count", func(t *testing.T) {
		if len(as.BuiltInAgents.AlwaysAvailable) != 2 {
			t.Errorf("expected 2 always-available agents, got %d", len(as.BuiltInAgents.AlwaysAvailable))
		}
	})

	t.Run("general_purpose_agent", func(t *testing.T) {
		var gp *struct {
			Type  string   `json:"type"`
			Tools []string `json:"tools"`
			Model string   `json:"model"`
		}
		for i := range as.BuiltInAgents.AlwaysAvailable {
			if as.BuiltInAgents.AlwaysAvailable[i].Type == "general-purpose" {
				gp = &as.BuiltInAgents.AlwaysAvailable[i]
				break
			}
		}
		if gp == nil {
			t.Fatal("general-purpose agent not found")
		}
		t.Run("tools_wildcard", func(t *testing.T) {
			if len(gp.Tools) != 1 || gp.Tools[0] != "*" {
				t.Errorf("expected [*], got %v", gp.Tools)
			}
		})
	})

	t.Run("statusline_setup_agent", func(t *testing.T) {
		var sl *struct {
			Type  string   `json:"type"`
			Tools []string `json:"tools"`
			Model string   `json:"model"`
		}
		for i := range as.BuiltInAgents.AlwaysAvailable {
			if as.BuiltInAgents.AlwaysAvailable[i].Type == "statusline-setup" {
				sl = &as.BuiltInAgents.AlwaysAvailable[i]
				break
			}
		}
		if sl == nil {
			t.Fatal("statusline-setup agent not found")
		}
		t.Run("limited_tools", func(t *testing.T) {
			expected := []string{"Read", "Edit"}
			if len(sl.Tools) != len(expected) {
				t.Fatalf("expected %v, got %v", expected, sl.Tools)
			}
			for i, e := range expected {
				if sl.Tools[i] != e {
					t.Errorf("tool[%d] = %q, want %q", i, sl.Tools[i], e)
				}
			}
		})
		t.Run("model_sonnet", func(t *testing.T) {
			if sl.Model != "sonnet" {
				t.Errorf("expected sonnet, got %s", sl.Model)
			}
		})
	})

	t.Run("conditionally_available_agents", func(t *testing.T) {
		if len(as.BuiltInAgents.ConditionallyAvailable) < 4 {
			t.Errorf("expected at least 4 conditional agents, got %d", len(as.BuiltInAgents.ConditionallyAvailable))
		}
	})

	conditionalTypes := []string{"Explore", "Plan", "claude-code-guide", "verification"}
	for _, typ := range conditionalTypes {
		typ := typ
		t.Run(fmt.Sprintf("conditional_%s", typ), func(t *testing.T) {
			found := false
			for _, a := range as.BuiltInAgents.ConditionallyAvailable {
				if a.Type == typ {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("conditional agent %q not found", typ)
			}
		})
	}

	t.Run("can_disable_via_env", func(t *testing.T) {
		if as.BuiltInAgents.CanDisableAllVia == "" {
			t.Error("should document how to disable built-in agents")
		}
	})
}

// TestAgentToolBehavior validates agent spawning rules.
// Source: AgentTool.tsx:319-322
func TestAgentToolBehavior(t *testing.T) {
	as := loadAgentSystem(t)
	ab := as.AgentToolBehavior

	t.Run("default_type_general_purpose", func(t *testing.T) {
		if ab.DefaultType != "general-purpose" {
			t.Errorf("expected general-purpose, got %s", ab.DefaultType)
		}
	})
	t.Run("explicit_type_wins", func(t *testing.T) {
		if !ab.ExplicitTypeWins {
			t.Error("explicit subagent_type must take priority")
		}
	})
	t.Run("isolation_modes", func(t *testing.T) {
		if len(ab.IsolationModes) != 1 || ab.IsolationModes[0] != "worktree" {
			t.Errorf("expected [worktree], got %v", ab.IsolationModes)
		}
	})
	t.Run("model_options", func(t *testing.T) {
		expected := []string{"sonnet", "opus", "haiku", "inherit"}
		if len(ab.ModelOptions) != len(expected) {
			t.Fatalf("expected %d model options, got %d", len(expected), len(ab.ModelOptions))
		}
		for i, e := range expected {
			if ab.ModelOptions[i] != e {
				t.Errorf("model[%d] = %q, want %q", i, ab.ModelOptions[i], e)
			}
		}
	})
}

// TestExploreAgent validates the Explore agent configuration.
// Source: exploreAgent.ts:65-78
func TestExploreAgent(t *testing.T) {
	as := loadAgentSystem(t)
	ea := as.ExploreAgent

	t.Run("type", func(t *testing.T) {
		if ea.Type != "Explore" {
			t.Errorf("expected Explore, got %s", ea.Type)
		}
	})
	t.Run("model_external_haiku", func(t *testing.T) {
		if ea.ModelExternal != "haiku" {
			t.Errorf("expected haiku for external, got %s", ea.ModelExternal)
		}
	})
	t.Run("model_ant_inherit", func(t *testing.T) {
		if ea.ModelAnt != "inherit" {
			t.Errorf("expected inherit for ant, got %s", ea.ModelAnt)
		}
	})

	disallowed := []string{"Agent", "ExitPlanMode", "Edit", "Write", "NotebookEdit"}
	for _, tool := range disallowed {
		tool := tool
		t.Run(fmt.Sprintf("disallows_%s", tool), func(t *testing.T) {
			found := false
			for _, d := range ea.DisallowedTools {
				if d == tool {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Explore agent should disallow %s", tool)
			}
		})
	}
}

// TestPlanAgent validates the Plan agent configuration.
// Source: planAgent.ts:74-87
func TestPlanAgent(t *testing.T) {
	as := loadAgentSystem(t)
	pa := as.PlanAgent

	t.Run("type", func(t *testing.T) {
		if pa.Type != "Plan" {
			t.Errorf("expected Plan, got %s", pa.Type)
		}
	})
	t.Run("model_inherit", func(t *testing.T) {
		if pa.Model != "inherit" {
			t.Errorf("expected inherit, got %s", pa.Model)
		}
	})
	t.Run("shares_tools_with_explore", func(t *testing.T) {
		// Source: planAgent.ts:85 — tools: EXPLORE_AGENT.tools
		if !pa.SharesToolsWithExplore {
			t.Error("Plan agent should share tool config with Explore agent")
		}
	})

	// Same disallowed tools as Explore
	for _, tool := range pa.DisallowedTools {
		tool := tool
		t.Run(fmt.Sprintf("disallows_%s", tool), func(t *testing.T) {
			found := false
			for _, d := range as.ExploreAgent.DisallowedTools {
				if d == tool {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Plan disallows %s but Explore doesn't — should match", tool)
			}
		})
	}
}

// TestMCPNamingConvention validates MCP tool name format.
// Source: mcp/utils.ts:40-47
func TestMCPNamingConvention(t *testing.T) {
	as := loadAgentSystem(t)
	mcp := as.MCPNaming

	t.Run("format", func(t *testing.T) {
		if mcp.ToolNameFormat != "mcp__{serverName}__{toolName}" {
			t.Errorf("expected mcp__{serverName}__{toolName}, got %s", mcp.ToolNameFormat)
		}
	})
	t.Run("separator_double_underscore", func(t *testing.T) {
		if mcp.Separator != "__" {
			t.Errorf("expected __, got %s", mcp.Separator)
		}
	})
	t.Run("prefix_mcp__", func(t *testing.T) {
		if mcp.Prefix != "mcp__" {
			t.Errorf("expected mcp__, got %s", mcp.Prefix)
		}
	})
	t.Run("detection_rule", func(t *testing.T) {
		if mcp.Detection == "" {
			t.Error("should document detection rule")
		}
	})
}

// TestSessionPersistence validates session storage format.
// Source: history.ts, FileReadTool.ts:216-219
func TestSessionPersistence(t *testing.T) {
	as := loadAgentSystem(t)
	sp := as.SessionPersistence

	t.Run("format_jsonl", func(t *testing.T) {
		if sp.Format != "JSONL (one JSON object per line)" {
			t.Errorf("expected JSONL, got %s", sp.Format)
		}
	})
	t.Run("extension_jsonl", func(t *testing.T) {
		if sp.Extension != ".jsonl" {
			t.Errorf("expected .jsonl, got %s", sp.Extension)
		}
	})
	t.Run("location_in_claude_dir", func(t *testing.T) {
		if sp.Location == "" {
			t.Error("should document session file location")
		}
	})
	t.Run("history_file", func(t *testing.T) {
		if sp.HistoryFile != "~/.claude/history.jsonl" {
			t.Errorf("expected ~/.claude/history.jsonl, got %s", sp.HistoryFile)
		}
	})
}
