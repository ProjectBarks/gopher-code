package tools

// RegisterDefaults registers all built-in tools with the given registry.
func RegisterDefaults(registry *ToolRegistry) {
	registry.Register(&BashTool{})
	registry.Register(&FileReadTool{})
	registry.Register(&FileWriteTool{})
	registry.Register(&FileEditTool{})
	registry.Register(&GlobTool{})
	registry.Register(&GrepTool{})
}
