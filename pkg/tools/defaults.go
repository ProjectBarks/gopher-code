package tools

import (
	"github.com/projectbarks/gopher-code/pkg/coordinator"
	"github.com/projectbarks/gopher-code/pkg/provider"
)

// SimpleToolNames are the only tools available in simple mode.
// Delegates to coordinator.SimpleToolNames for the canonical list.
var SimpleToolNames = coordinator.SimpleToolNames

// IsSimpleMode returns true when CLAUDE_CODE_SIMPLE is set to a truthy value.
// Delegates to coordinator.IsSimpleMode which checks the env var using the
// same isEnvTruthy logic as the TS source (1, true, yes, on).
func IsSimpleMode() bool {
	return coordinator.IsSimpleMode()
}

// RegisterDefaults registers all built-in tools with the given registry.
// When CLAUDE_CODE_SIMPLE=1, only Bash, Read, and Edit are registered.
// Source: tools.ts:271-298
// It returns the PlanState so the caller (e.g. REPL) can inspect/set plan mode.
func RegisterDefaults(registry *ToolRegistry) *PlanState {
	if IsSimpleMode() {
		return registerSimple(registry)
	}
	registry.Register(&BashTool{})
	registry.Register(&FileReadTool{})
	registry.Register(&FileWriteTool{})
	registry.Register(&FileEditTool{})
	registry.Register(&GlobTool{})
	registry.Register(&GrepTool{})
	registry.Register(&WebFetchTool{})
	registry.Register(&WebSearchTool{})
	registry.Register(&AskUserQuestionTool{})
	registry.Register(&NotebookEditTool{})
	registry.Register(&ListDirectoryTool{})
	registry.Register(&SendMessageTool{})
	registry.Register(NewToolSearchTool(registry))

	todoWrite, todoRead := NewTodoTools()
	registry.Register(todoWrite)
	registry.Register(todoRead)

	// Task management
	for _, t := range NewTaskTools() {
		registry.Register(t)
	}

	// Plan mode
	planState, planTools := NewPlanModeTools()
	for _, t := range planTools {
		registry.Register(t)
	}

	// Cron / scheduling
	for _, t := range NewCronTools() {
		registry.Register(t)
	}

	// Sleep
	registry.Register(&SleepTool{})

	// LSP
	registry.Register(&LSPTool{})

	// MCP resources
	registry.Register(&ListMcpResourcesTool{})
	registry.Register(&ReadMcpResourceTool{})

	// Worktree
	registry.Register(&EnterWorktreeTool{})
	registry.Register(&ExitWorktreeTool{})

	// PowerShell (Windows / cross-platform pwsh)
	registry.Register(&PowerShellTool{})

	// Team management
	for _, t := range NewTeamTools() {
		registry.Register(t)
	}

	// Config
	registry.Register(NewConfigTool())

	// Remote trigger (placeholder)
	registry.Register(&RemoteTriggerTool{})

	// Synthetic output (internal use)
	registry.Register(&SyntheticOutputTool{})

	// Brief (context sharing between sessions)
	registry.Register(&BriefTool{})

	// MCP auth tools are created per-server by CreateMcpAuthTool when an
	// unauthenticated MCP server is discovered — not registered by default.

	// REPL (interactive language sessions)
	registry.Register(&REPLTool{})

	return planState
}

// registerSimple registers only Bash, Read, Edit for CLAUDE_CODE_SIMPLE mode.
// Source: tools.ts:287
func registerSimple(registry *ToolRegistry) *PlanState {
	registry.Register(&BashTool{})
	registry.Register(&FileReadTool{})
	registry.Register(&FileEditTool{})
	return &PlanState{} // No plan mode in simple mode
}

// RegisterAgentTool registers the Agent tool, which needs runtime dependencies
// (provider, registry, and a query function) that aren't available at
// RegisterDefaults time.
func RegisterAgentTool(registry *ToolRegistry, prov provider.ModelProvider, queryFn QueryFunc) {
	registry.Register(NewAgentTool(prov, registry, queryFn))
}
