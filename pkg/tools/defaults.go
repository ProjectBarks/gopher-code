package tools

import "github.com/projectbarks/gopher-code/pkg/provider"

// RegisterDefaults registers all built-in tools with the given registry.
func RegisterDefaults(registry *ToolRegistry) {
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
}

// RegisterAgentTool registers the Agent tool, which needs runtime dependencies
// (provider, registry, and a query function) that aren't available at
// RegisterDefaults time.
func RegisterAgentTool(registry *ToolRegistry, prov provider.ModelProvider, queryFn QueryFunc) {
	registry.Register(NewAgentTool(prov, registry, queryFn))
}
