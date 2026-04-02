package tools

import "github.com/projectbarks/gopher-code/pkg/provider"

// RegisterDefaults registers all built-in tools with the given registry.
// It returns the PlanState so the caller (e.g. REPL) can inspect/set plan mode.
func RegisterDefaults(registry *ToolRegistry) *PlanState {
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

	return planState
}

// RegisterAgentTool registers the Agent tool, which needs runtime dependencies
// (provider, registry, and a query function) that aren't available at
// RegisterDefaults time.
func RegisterAgentTool(registry *ToolRegistry, prov provider.ModelProvider, queryFn QueryFunc) {
	registry.Register(NewAgentTool(prov, registry, queryFn))
}
