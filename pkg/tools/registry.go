package tools

import (
	"sort"
	"sync"

	"github.com/projectbarks/gopher-code/pkg/provider"
)

// ToolRegistry holds registered tools keyed by name.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get returns a tool by name, or nil if not found.
func (r *ToolRegistry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// All returns all registered tools, sorted by name for prompt cache stability.
// All returns all enabled, registered tools, sorted by name.
// Filters out tools where IsEnabled() returns false.
// Source: tools.ts:181-182, 362-364
func (r *ToolRegistry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		if IsToolEnabled(t) {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// Unregister removes a tool from the registry by name.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// ToolDefinitions returns tool definitions for enabled tools, sorted by name.
// Filters out disabled tools (IsEnabled() == false).
// Source: tools.ts:181-182, 354-366
func (r *ToolRegistry) ToolDefinitions() []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]provider.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		if !IsToolEnabled(t) {
			continue
		}
		defs = append(defs, provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}
